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
    artifacts_dir,
    emit_go_weights,
    fixtures_dir,
    load_traces,
    teacher_perspective_samples,
    traces_dir,
    weights_out_path,
)


def train_trace_path():
    return traces_dir() / "opponent.jsonl.gz"


def cache_path(trace_file, max_rows, max_samples):
    """Deterministic cache path based on trace file content and limits."""
    h = hashlib.md5()
    h.update(str(trace_file).encode())
    h.update(str(os.path.getmtime(trace_file)).encode())
    h.update(f"{max_rows}:{max_samples}".encode())
    return artifacts_dir() / f"features_{h.hexdigest()[:12]}.npz"


def load_or_extract(trace_file, max_rows, max_samples):
    """Load cached features or extract from traces."""
    cp = cache_path(trace_file, max_rows, max_samples)
    if cp.exists():
        print(f"cache hit: {cp.name}", file=sys.stderr)
        data = np.load(cp, allow_pickle=True)
        fixture = json.loads(str(data["fixture"])) if "fixture" in data else None
        return data["X"], data["M"], data["y"], int(data["rows_used"]), fixture

    print("extracting features …", file=sys.stderr)
    t0 = time.time()
    rows = load_traces(trace_file)
    samples = []
    fixture = None
    rows_used = 0
    for row in rows:
        if max_rows > 0 and rows_used >= max_rows:
            break
        rows_used += 1
        for feats, mask, label, snake_id, side in teacher_perspective_samples(row):
            samples.append((feats, mask, label))
            if fixture is None:
                fixture = {
                    "row": row,
                    "snake_id": snake_id,
                    "teacher_side": side,
                    "features": feats,
                    "mask": [1 if v else 0 for v in mask],
                    "label": label,
                }
            if max_samples > 0 and len(samples) >= max_samples:
                break
        if max_samples > 0 and len(samples) >= max_samples:
            break

    if not samples:
        raise SystemExit(f"no training samples in {trace_file}")

    X = np.asarray([s[0] for s in samples], dtype=np.float32)
    M = np.asarray([s[1] for s in samples], dtype=np.bool_)
    y = np.asarray([s[2] for s in samples], dtype=np.int32)

    cp.parent.mkdir(parents=True, exist_ok=True)
    fixture_str = json.dumps(fixture) if fixture else ""
    np.savez(cp, X=X, M=M, y=y, rows_used=np.array(rows_used), fixture=np.array(fixture_str))
    print(f"cached {len(X)} samples in {time.time()-t0:.1f}s -> {cp.name}", file=sys.stderr)
    return X, M, y, rows_used, fixture


class MLP(nn.Module):
    def __init__(self):
        super().__init__()
        self.fc1 = nn.Linear(96, 160)
        self.fc2 = nn.Linear(160, 96)
        self.fc3 = nn.Linear(96, 1)

    def __call__(self, x):
        x = nn.relu(self.fc1(x))
        x = nn.relu(self.fc2(x))
        return self.fc3(x).squeeze(-1)


def loss_fn(model, x, mask, y):
    logits = model(x)
    logits = mx.where(mask, logits, mx.array(-1e9, dtype=mx.float32))
    log_probs = logits - mx.logsumexp(logits, axis=1, keepdims=True)
    target_log_probs = mx.take_along_axis(log_probs, y[:, None], axis=1).squeeze(1)
    return -mx.mean(target_log_probs)


def main() -> None:
    max_rows = int(os.environ.get("NN_MAX_ROWS", "0"))
    max_samples = int(os.environ.get("NN_MAX_SAMPLES", "0"))
    epochs = int(os.environ.get("NN_EPOCHS", "8"))

    X_np, M_np, y_np, rows_used, fixture = load_or_extract(
        train_trace_path(), max_rows, max_samples
    )

    # Initialize weights matching numpy version distribution
    rng = np.random.default_rng(7)
    model = MLP()
    # nn.Linear weight shape: (out, in); numpy convention: (in, out)
    model.fc1.weight = mx.array((rng.standard_normal((96, 160)) * 0.03).astype(np.float32).T)
    model.fc1.bias = mx.zeros(160)
    model.fc2.weight = mx.array((rng.standard_normal((160, 96)) * 0.03).astype(np.float32).T)
    model.fc2.bias = mx.zeros(96)
    model.fc3.weight = mx.array((rng.standard_normal((96, 1)) * 0.03).astype(np.float32).T)
    model.fc3.bias = mx.zeros(1)

    optimizer = optim.SGD(learning_rate=0.03)
    loss_and_grad_fn = nn.value_and_grad(model, loss_fn)

    X = mx.array(X_np)
    M = mx.array(M_np)
    y = mx.array(y_np)

    t0 = time.time()
    for epoch in range(epochs):
        order = rng.permutation(len(X_np))
        for start in range(0, len(X_np), 256):
            batch_idx = mx.array(order[start : start + 256])
            xb = X[batch_idx]
            mb = M[batch_idx]
            yb = y[batch_idx]
            loss, grads = loss_and_grad_fn(model, xb, mb, yb)
            optimizer.update(model, grads)
            mx.eval(loss, model.parameters(), optimizer.state)
    print(f"training: {time.time()-t0:.1f}s ({epochs} epochs)", file=sys.stderr)

    logits = model(X)
    logits = mx.where(M, logits, mx.array(-1e9, dtype=mx.float32))
    preds = mx.argmax(logits, axis=1)
    mx.eval(preds)
    train_acc = float(mx.mean(preds == y))

    # Export weights (transpose back to numpy in/out convention)
    w1 = np.array(model.fc1.weight.T)
    b1 = np.array(model.fc1.bias)
    w2 = np.array(model.fc2.weight.T)
    b2 = np.array(model.fc2.bias)
    w3 = np.array(model.fc3.weight.T)
    b3 = np.array(model.fc3.bias)

    meta = emit_go_weights(
        [
            w1.reshape(-1).tolist(),
            b1.tolist(),
            w2.reshape(-1).tolist(),
            b2.tolist(),
            w3.reshape(-1).tolist(),
            b3.tolist(),
        ],
        weights_out_path(),
    )

    artifacts = artifacts_dir()
    artifacts.mkdir(parents=True, exist_ok=True)
    (artifacts / "train_meta.json").write_text(
        json.dumps(
            {
                "teacher": "opponent",
                "trace_path": str(train_trace_path()),
                "samples": len(X_np),
                "rows_used": rows_used,
                "train_accuracy": train_acc,
                "blob_chars": meta["blob_chars"],
                "mode": "mlx",
                "epochs": epochs,
            },
            indent=2,
        ),
        encoding="utf-8",
    )
    if fixture is not None:
        fx = fixtures_dir()
        fx.mkdir(parents=True, exist_ok=True)
        (fx / "feature_fixture.json").write_text(json.dumps(fixture, indent=2), encoding="utf-8")
        (artifacts / "feature_fixture.json").write_text(json.dumps(fixture, indent=2), encoding="utf-8")

    print(
        json.dumps(
            {
                "teacher": "opponent",
                "samples": len(X_np),
                "rows_used": rows_used,
                "train_accuracy": train_acc,
                "blob_chars": meta["blob_chars"],
                "mode": "mlx",
                "epochs": epochs,
            },
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
