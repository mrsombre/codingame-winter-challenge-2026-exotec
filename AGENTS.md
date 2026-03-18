# CodinGame Winter Challenge 2026 — Exotec

## Project Structure

```
CLAUDE.md               # Project instructions
agent/                  # Arena-ready stdin/stdout bot sources
cmd/
└─ match/               # Tiny CLI entrypoint for local matches
internal/
├─ engine/              # Java engine parity port used by the simulator
└─ match/               # Local binary-vs-binary runner and stats collection
```

## Project Rules

- NEVER read `bundle.go` files — they are auto-generated single-file bundles for arena submission; read `cmd/main.go` in the same agent directory instead
- Codingame provides `golang 1.18.1`
- Arena bots live in `agent/<name>/main.go`
- Local matches should use external bot binaries; do not reintroduce in-process agent fallbacks
- Default local opponent binary is `./bin/opponent`
- Use `./tmp` for any temporary files, including tests, scripts, probes, and helpers
- Use `./bin` as the target directory for builds

## Building and Testing

ALWAYS use `make` targets. NEVER run `go build` or `go run ./cmd/match` directly.

```shell
# Run tests
make test

# Build a bot binary into ./bin/<name>
make build-agent LOGIC=basic

# Build the baseline opponent into ./bin/opponent
make build-opponent
```

## Running Matches

ALWAYS use `make match` or `make match-bin`. Default: 30 simulations, 5 parallel, 100 max turns.

```shell
# Build + run: basic vs opponent (30 matches, 5 parallel)
make match LOGIC=basic

# Override match count
make match LOGIC=basic ENGINE_ARGS="--simulations 50 --parallel 5 --seed 50"

# Run two pre-built binaries against each other
make match-bin P0=basic P1=opponent
```

## Replay And Debug Viewer

Use `make replay` to regenerate debug data and `make debug` to launch the frontend viewer. The replay/debug flow is driven by tests in `agent/<logic>/src`, not by the normal match runner output format.

```shell
# Regenerate debug JSON in ./debug/public
make replay LOGIC=basic

# Capture one deterministic debug trace from the local match runner
mkdir -p replay
printf 'seed=50\n' > replay/seed.txt
make match LOGIC=basic ENGINE_ARGS="--debug --seed 50" 2> replay/replay.txt
```

Core ideas:

- The frontend is a Vite app in `./debug`; it serves files from `./debug/public` and currently fetches `/map.json`
- Replay input is loaded from `./replay/seed.txt` and `./replay/replay.txt` when those files exist; otherwise the tests fall back to `dbgSeed` and `dbgTurnLines` in `agent/<logic>/src/decision_test.go`
- `replay/replay.txt` can contain the full `--debug` stderr log from `make match`; the loader keeps only lines that start with digits, which are the turn input lines
