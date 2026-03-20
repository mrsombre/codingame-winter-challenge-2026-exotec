from __future__ import annotations

import json
import os

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


def main() -> None:
    rows = load_traces(train_trace_path())
    samples = []
    fixture = None
    max_rows = int(os.environ.get("NN_MAX_ROWS", "0"))
    max_samples = int(os.environ.get("NN_MAX_SAMPLES", "0"))
    epochs = int(os.environ.get("NN_EPOCHS", "8"))
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
        raise SystemExit(f"no training samples in {train_trace_path()}")

    X = np.asarray([sample[0] for sample in samples], dtype=np.float32)
    M = np.asarray([sample[1] for sample in samples], dtype=np.bool_)
    y = np.asarray([sample[2] for sample in samples], dtype=np.int64)

    rng = np.random.default_rng(7)
    w1 = (rng.standard_normal((96, 160)) * 0.03).astype(np.float32)
    b1 = np.zeros(160, dtype=np.float32)
    w2 = (rng.standard_normal((160, 96)) * 0.03).astype(np.float32)
    b2 = np.zeros(96, dtype=np.float32)
    w3 = (rng.standard_normal((96, 1)) * 0.03).astype(np.float32)
    b3 = np.zeros(1, dtype=np.float32)

    def softmax(logits):
        shifted = logits - np.max(logits, axis=1, keepdims=True)
        exps = np.exp(shifted)
        return exps / np.sum(exps, axis=1, keepdims=True)

    for _ in range(epochs):
        order = rng.permutation(len(X))
        for start in range(0, len(X), 256):
            idx = order[start : start + 256]
            xb = X[idx]
            mb = M[idx]
            yb = y[idx]

            h1 = np.maximum(0.0, xb @ w1 + b1)
            h2 = np.maximum(0.0, h1 @ w2 + b2)
            logits = (h2 @ w3 + b3).squeeze(-1)
            logits = np.where(mb, logits, -1e9)
            probs = softmax(logits)

            dlogits = probs
            dlogits[np.arange(len(yb)), yb] -= 1.0
            dlogits /= len(yb)
            dlogits = np.where(mb, dlogits, 0.0)

            dw3 = np.sum(h2 * dlogits[..., None], axis=(0, 1), keepdims=True).reshape(96, 1)
            db3 = np.sum(dlogits)
            dh2 = dlogits[..., None] * w3.T
            dh2[h2 <= 0] = 0
            dw2 = np.tensordot(h1, dh2, axes=([0, 1], [0, 1]))
            db2 = np.sum(dh2, axis=(0, 1))
            dh1 = np.matmul(dh2, w2.T)
            dh1[h1 <= 0] = 0
            dw1 = np.tensordot(xb, dh1, axes=([0, 1], [0, 1]))
            db1 = np.sum(dh1, axis=(0, 1))

            lr = 0.03
            w3 -= lr * dw3.astype(np.float32)
            b3 -= lr * np.asarray([db3], dtype=np.float32)
            w2 -= lr * dw2.astype(np.float32)
            b2 -= lr * db2.astype(np.float32)
            w1 -= lr * dw1.astype(np.float32)
            b1 -= lr * db1.astype(np.float32)

    h1 = np.maximum(0.0, X @ w1 + b1)
    h2 = np.maximum(0.0, h1 @ w2 + b2)
    logits = (h2 @ w3 + b3).squeeze(-1)
    logits = np.where(M, logits, -1e9)
    preds = np.argmax(logits, axis=1)
    train_acc = float(np.mean(preds == y))

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
                "samples": int(len(X)),
                "rows_used": rows_used,
                "train_accuracy": train_acc,
                "blob_chars": meta["blob_chars"],
                "mode": "numpy",
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
                "samples": int(len(X)),
                "rows_used": rows_used,
                "train_accuracy": train_acc,
                "blob_chars": meta["blob_chars"],
                "mode": "numpy",
                "epochs": epochs,
            },
            indent=2,
        )
    )


if __name__ == "__main__":
    main()
