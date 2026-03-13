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

- Codingame provides `golang 1.18.1`
- Arena bots live in `agent/<name>/main.go`
- Local matches should use external bot binaries; do not reintroduce in-process agent fallbacks
- Default local opponent binary is `./bin/opponent`
- Use `./tmp` for any temporary files, including tests, scripts, probes, and helpers
- Use `./bin` as the target directory for builds
```shell
# go manager
go install golang.org/dl/go1.18.1@latest
go1.18.1 download
go1.18.1 test <puzzle_folder>
# or docker compose
docker compose run --rm builder go test ./..
```

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
