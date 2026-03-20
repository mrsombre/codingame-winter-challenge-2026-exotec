from __future__ import annotations

import json
import subprocess

from common import artifacts_dir, repo_root


ROOT = repo_root()


def run_match(p0: str, p1: str, seed: int) -> dict:
    cmd = [
        "task",
        "match-bin",
        f"P0={p0}",
        f"P1={p1}",
        f"ENGINE_ARGS=--simulations 40 --parallel 4 --seed {seed}",
    ]
    proc = subprocess.run(cmd, cwd=ROOT, check=True, capture_output=True, text=True)
    return json.loads(proc.stdout)


def metric_avg(report: dict, label: str) -> float:
    for metric in report["summary"]["metrics"]:
        if metric["label"] == label:
            return float(metric["avg"])
    raise KeyError(label)


def main() -> None:
    opp_vs_opp = run_match("opponent", "opponent", 61616)
    nn_vs_opp = run_match("nn", "opponent", 70707)

    teacher_wr = metric_avg(opp_vs_opp, "wins_p0")
    nn_wr = metric_avg(nn_vs_opp, "wins_p0")
    payload = {
        "teacher": "opponent",
        "teacher_vs_opponent_win_rate": teacher_wr,
        "nn_vs_opponent_win_rate": nn_wr,
        "teacher_ratio": (nn_wr / teacher_wr) if teacher_wr > 0 else 0.0,
    }
    out = artifacts_dir()
    out.mkdir(parents=True, exist_ok=True)
    (out / "evaluate.json").write_text(json.dumps(payload, indent=2), encoding="utf-8")
    print(json.dumps(payload, indent=2))


if __name__ == "__main__":
    main()
