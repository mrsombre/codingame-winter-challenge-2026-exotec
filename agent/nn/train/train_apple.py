from __future__ import annotations

import hashlib
import json
import os
import sys
import time

import mlx.core as mx
import mlx.nn as nn
import mlx.optimizers as optim
import numpy as np

from common import (
    APPLE_FEATURE_COUNT,
    APPLE_FEATURE_SCHEMA_VERSION,
    apple_features_for_snake,
    artifacts_dir,
    build_blocked,
    build_state,
    emit_go_weights,
    flood_dist,
    load_traces,
    parse_command_line,
    simulate_candidate,
    traces_dir,
    weights_out_path,
)


def train_trace_path():
    return traces_dir() / "opponent.jsonl.gz"


def cache_path(trace_file, max_rows, max_samples):
    h = hashlib.md5()
    h.update(str(trace_file).encode())
    h.update(str(os.path.getmtime(trace_file)).encode())
    h.update(f"apple:{max_rows}:{max_samples}".encode())
    return artifacts_dir() / f"apple_features_{h.hexdigest()[:12]}.npz"


def load_or_extract(trace_file, max_rows, max_samples, neg_ratio=3):
    cp = cache_path(trace_file, max_rows, max_samples)
    if cp.exists():
        print(f"cache hit: {cp.name}", file=sys.stderr)
        data = np.load(cp)
        return data["X"], data["y"], int(data["rows_used"])

    print("extracting apple features …", file=sys.stderr)
    t0 = time.time()
    rows = load_traces(trace_file)
    rng = np.random.default_rng(42)
    features_list = []
    labels_list = []
    rows_used = 0

    for row in rows:
        if max_rows > 0 and rows_used >= max_rows:
            break
        rows_used += 1

        for cmd_key, teacher_owner in [("p0_command", 0), ("p1_command", 1)]:
            state = build_state(row, teacher_owner=teacher_owner)
            commands = parse_command_line(row.get(cmd_key, ""))
            width = state["width"]
            height = state["height"]

            for sn_index, snake in enumerate(state["snakes"]):
                if snake["owner"] != 0 or snake["id"] not in commands:
                    continue
                dir_idx = commands[snake["id"]]
                cand = simulate_candidate(state, sn_index, dir_idx)
                if not cand.legal or not cand.eating:
                    continue

                head = snake["body"][0]
                blocked = build_blocked(state, sn_index, snake["body"])
                flood_count, dists = flood_dist(head, blocked, width, height)

                eaten_apple = cand.head

                # Positive sample
                feats = apple_features_for_snake(state, sn_index, eaten_apple, head, flood_count, dists)
                features_list.append(feats)
                labels_list.append(1)

                # Negative samples
                non_eaten = [a for a in state["apples"] if a != eaten_apple]
                n_neg = min(neg_ratio, len(non_eaten))
                if n_neg > 0:
                    chosen = rng.choice(len(non_eaten), size=n_neg, replace=False)
                    for idx in chosen:
                        feats = apple_features_for_snake(
                            state, sn_index, non_eaten[idx], head, flood_count, dists
                        )
                        features_list.append(feats)
                        labels_list.append(0)

        if max_samples > 0 and len(features_list) >= max_samples:
            features_list = features_list[:max_samples]
            labels_list = labels_list[:max_samples]
            break

    if not features_list:
        raise SystemExit(f"no eating events in {trace_file}")

    X = np.asarray(features_list, dtype=np.float32)
    y = np.asarray(labels_list, dtype=np.float32)
    cp.parent.mkdir(parents=True, exist_ok=True)
    np.savez(cp, X=X, y=y, rows_used=np.array(rows_used))
    pos = int(y.sum())
    print(
        f"cached {len(X)} samples ({pos} pos, {len(X)-pos} neg) in {time.time()-t0:.1f}s -> {cp.name}",
        file=sys.stderr,
    )
    return X, y, rows_used


class AppleMLP(nn.Module):
    def __init__(self):
        super().__init__()
        self.fc1 = nn.Linear(APPLE_FEATURE_COUNT, 32)
        self.fc2 = nn.Linear(32, 1)

    def __call__(self, x):
        x = nn.relu(self.fc1(x))
        return self.fc2(x).squeeze(-1)


def loss_fn(model, x, y):
    logits = model(x)
    # Binary cross-entropy with logits (numerically stable)
    return mx.mean(mx.maximum(logits, 0) - logits * y + mx.log1p(mx.exp(-mx.abs(logits))))


def main() -> None:
    max_rows = int(os.environ.get("NN_MAX_ROWS", "0"))
    max_samples = int(os.environ.get("NN_MAX_SAMPLES", "0"))
    epochs = int(os.environ.get("NN_EPOCHS", "8"))

    X_np, y_np, rows_used = load_or_extract(train_trace_path(), max_rows, max_samples)

    rng = np.random.default_rng(7)
    model = AppleMLP()
    model.fc1.weight = mx.array(
        (rng.standard_normal((APPLE_FEATURE_COUNT, 32)) * 0.1).astype(np.float32).T
    )
    model.fc1.bias = mx.zeros(32)
    model.fc2.weight = mx.array((rng.standard_normal((32, 1)) * 0.1).astype(np.float32).T)
    model.fc2.bias = mx.zeros(1)

    optimizer = optim.SGD(learning_rate=0.05)
    loss_and_grad_fn = nn.value_and_grad(model, loss_fn)

    X = mx.array(X_np)
    y = mx.array(y_np)

    t0 = time.time()
    for epoch in range(epochs):
        order = rng.permutation(len(X_np))
        for start in range(0, len(X_np), 256):
            batch_idx = mx.array(order[start : start + 256])
            xb = X[batch_idx]
            yb = y[batch_idx]
            loss, grads = loss_and_grad_fn(model, xb, yb)
            optimizer.update(model, grads)
            mx.eval(loss, model.parameters(), optimizer.state)
    print(f"training: {time.time()-t0:.1f}s ({epochs} epochs)", file=sys.stderr)

    # Train accuracy
    logits = model(X)
    mx.eval(logits)
    preds = (logits > 0).astype(mx.float32)
    train_acc = float(mx.mean(preds == y))

    # Export weights (transpose back: MLX Linear is (out, in), we need (in, out))
    w1 = np.array(model.fc1.weight.T)  # (16, 32)
    b1 = np.array(model.fc1.bias)  # (32,)
    w2 = np.array(model.fc2.weight.T)  # (32, 1)
    b2 = np.array(model.fc2.bias)  # (1,)

    meta = emit_go_weights(
        [
            w1.reshape(-1).tolist(),
            b1.tolist(),
            w2.reshape(-1).tolist(),
            b2.tolist(),
        ],
        weights_out_path(),
        schema_version=APPLE_FEATURE_SCHEMA_VERSION,
    )

    artifacts = artifacts_dir()
    artifacts.mkdir(parents=True, exist_ok=True)
    pos = int(y_np.sum())
    (artifacts / "train_meta.json").write_text(
        json.dumps(
            {
                "teacher": "opponent",
                "trace_path": str(train_trace_path()),
                "samples": len(X_np),
                "positive": pos,
                "negative": len(X_np) - pos,
                "rows_used": rows_used,
                "train_accuracy": train_acc,
                "blob_chars": meta["blob_chars"],
                "mode": "mlx_apple",
                "epochs": epochs,
            },
            indent=2,
        ),
        encoding="utf-8",
    )

    print(
        json.dumps(
            {
                "teacher": "opponent",
                "samples": len(X_np),
                "positive": pos,
                "negative": len(X_np) - pos,
                "rows_used": rows_used,
                "train_accuracy": train_acc,
                "blob_chars": meta["blob_chars"],
                "mode": "mlx_apple",
                "epochs": epochs,
            },
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
