# Winter Challenge 2026 — Exotec

<https://www.codingame.com/ide/challenge/winter-challenge-2026-exotec>

## Rules

See [rules.md](rules.md).

## Layout

```text
agent/                  # Arena-ready stdin/stdout bot sources
cmd/
└─ match/               # Local binary-vs-binary match runner
engine/                 # Batch execution and summary helpers
simulator/              # Referee, map generation, subprocess player wiring
```

## Local Workflow

Arena bots live in `agent/<name>/main.go`.

Build and test:

```shell
env GOCACHE=/tmp/go-build go test ./...

make build-agent LOGIC=greed
make build-opponent
```

Run matches:

```shell
# greed vs default opponent
make match LOGIC=greed

# arbitrary binary-vs-binary run
make match-bin P0=greed P1=opponent
```

Defaults:

- `ENGINE_ARGS=--simulations 30 --parallel 5 --seed 50 --output-matches`
- `GAME_ARGS=--max-turns 100`

The local runner is `go run ./cmd/match ...`.
If `--p1-bin` is not supplied, it defaults to `./bin/opponent`.

## Notes

- There is no in-process fallback opponent.
- The source of truth for arena submission is the bot under `agent/<name>/main.go`.
