from __future__ import annotations

import json

from common import artifacts_dir


def main() -> None:
    payload = {
        "teacher": "opponent",
        "source": "user-selected strongest binary",
    }
    out = artifacts_dir()
    out.mkdir(parents=True, exist_ok=True)
    (out / "teacher_info.json").write_text(json.dumps(payload, indent=2), encoding="utf-8")
    print(json.dumps(payload, indent=2))


if __name__ == "__main__":
    main()
