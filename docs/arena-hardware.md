# Arena Hardware

Observed via `runtime` + `/proc` at bot startup (stderr output).

## Specs

| Property | Value |
|---|---|
| CPU | Intel Core Processor (Haswell, no TSX) |
| Cores | **1** (GOMAXPROCS=1) |
| RAM | **256 MB** total |
| OS | Linux amd64 |

## Performance Calibration

Measured with the genetic agent, `PopSize=20`, `MaxDepth=6`, seed 18 (18×10 map).

| Scenario | Local M4 | Arena (est.) | Ratio |
|---|---|---|---|
| `simulateGene` | 2.65 µs | ~7 µs | ~2.7× slower |
| GA gens / 45ms (4 bots, 2v2) | ~371 | **~23** | — |
| GA gens / 45ms (1v1 endgame) | — | **~44–45** | — |

Arena throughput with 4 alive bots (full game): **~23 gens × 40 sims = ~920 sim calls per turn**.

## Implications

- **No parallelism** — goroutines and `GOMAXPROCS>1` are wasted overhead.
- **256 MB RAM** — avoid large heap allocations per turn; prefer stack-allocated arrays over `map[Point]int`.
- **2.7× slower than M4** — always benchmark with `GOMAXPROCS=1`; the Makefile exports it.
- **23 gens is low** — with 4 bots alive, the GA has little time to converge. Consider reducing `MaxDepth` or `PopSize` during the full-game phase.

## Turn Budget

- First turn: **950 ms** — use for deep search / warm start.
- Subsequent turns: **43 ms** — timer must start **after** all `readline()` calls; engine processing time (20–40 ms) eats into the budget if the timer starts before I/O.

## Local Benchmark Command

```shell
GOMAXPROCS=1 go test -v -run TestBudget45ms ./agent/genetic/
GOMAXPROCS=1 go test -bench=. -benchtime=2s ./agent/genetic/
```

Perftest uses seed 18 (SHA1PRNG, league level 1) → real engine map 18×10, 2 bots per player.
