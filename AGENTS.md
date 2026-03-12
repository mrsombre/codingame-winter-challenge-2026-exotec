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

## Project Commands

```shell
# GOCACHE is exported in the Makefile, use make targets:
make test

make build-agent LOGIC=greed
make build-opponent

# greed vs default opponent binary
make match LOGIC=greed

# arbitrary binary-vs-binary match
make match-bin P0=greed P1=opponent
```

## Match Workflow

- Build arena bots from `agent/<logic>` into `bin/<logic>` with `make build-agent LOGIC=<logic>`
- Build the default baseline opponent with `make build-opponent`
- Run local batches through `go run ./cmd/match ...` or the `make match` / `make match-bin` wrappers
- `make match` defaults to:
  - `ENGINE_ARGS=--simulations 30 --parallel 5 --seed 50 --output-matches`
  - `GAME_ARGS=--max-turns 100`
- `cmd/match` defaults player 1 to `./bin/opponent` when `--p1-bin` is not supplied
