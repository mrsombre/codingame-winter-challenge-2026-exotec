# Optimization: BestAction SupPathBFS → SupReachMulti

## Problem

`BestAction` called `SupPathBFS` once per source (apple) to find which sources are reachable with the current body length. With 18 apples, this meant 18 independent BFS runs from the same start point — each exploring mostly the same cells.

**Cost breakdown (seed1001, 32x17 map, bodyLen=5):**
- `SupPathBFS` per call: ~13µs (far target), ~2.5µs (near)
- 18 calls total: ~234µs
- `BestAction` total: **217µs** (dominated by SupPathBFS loop)

## Root Causes

1. **Redundant BFS work** — each `SupPathBFS` call explores from the same start, traversing the same cells to find one target
2. **No body-length cap** — BFS explored buckets up to `maxLen=49` (width+height), then the caller filtered `MinLen <= bodyLen` (5). Most work was in buckets 6..49 — completely wasted
3. **Unnecessary path reconstruction** — `SupPathBFS` built waypoint paths via prev-pointer chain; `BestAction` never used them (`srcCand.dist` was dead code)

## Solution: `SupReachMulti`

Single support-aware BFS that finds ALL reachable targets at once.

**Key changes:**
- One BFS replaces N separate calls (18× → 1×)
- BFS capped at `bodyLen` instead of `maxLen` (explores buckets 1..5 instead of 1..49)
- Uses lighter `ImmBuf` (no prev-pointer tracking)
- Target adjacency check fused into BFS expansion loop (one neighbor iteration, not two)
- Returns `[]Point` instead of `[]SBResult` (no waypoints, no dist — only reachability needed)

**Algorithm:**
1. Build `BitGrid` of target positions (skip walls)
2. Bucket-BFS from start, capped at `maxBodyLen`
3. At each cell, check 4 neighbors against target BitGrid
4. When target found: clear bit, append to result, decrement remaining
5. Early exit when all targets found

## Results

```
GOMAXPROCS=1, Apple M4, seed1001 (32x17), bodyLen=5, 18 apples

BestAction:       217,146 ns → 26,531 ns  (8.2× faster)
FullTurnPipeline: 206,479 ns → 31,924 ns  (6.5× faster)
```

## Correctness

Verified with two tests:
- `TestSupReachMulti_MatchesPerTarget` — compares result sets on engine seed
- `TestSupReachMulti_Seed1001` — compares on benchmark fixture

Both confirm identical reachable sets vs the old per-target `SupPathBFS` loop.

## Files Changed

- `internal/agentkit/support_bfs.go` — added `SupReachMulti`
- `internal/agentkit/search.go` — `BestAction` uses `SupReachMulti`, removed dead `srcCand.dist`
- `internal/agentkit/support_bfs_test.go` — correctness tests
- `internal/agentkit/search_bench_test.go` — benchmarks for all hot-path functions

---

# Optimization: Dead Code Removal & Allocation Reduction (basic agent)

## Profiling (100 turns, 42x23 map, 4v4)

| Function | Total | Calls | Avg/call | % Budget |
|---|---|---|---|---|
| refinePlans | 99.9ms | 100 | 999µs | 53% |
| pathBFS | 52.9ms | 330 | 160µs | 28% |
| planSupportJobs | 15.6ms | 100 | 156µs | 8% |
| calcDirInfo | 6.8ms | 347 | 19µs | 4% |
| calcEnemyDist | 4.3ms | 100 | 43µs | 2% |
| botFloodDist | 3.3ms | 100 | 33µs | 2% |
| bestAction | 3.7ms | 161 | 23µs | 2% |
| myFloodDist | 2.6ms | 347 | 7µs | 1% |

## Dead Code Removed

- `Body` type + 9 methods — never referenced after switch to `[]Point` slices
- `Bot` struct — replaced by `botEntry`/`enemyInfo`
- `State.Bots`, `State.DistVals` fields — allocated but never read
- `STerrain.WallSup` field + builder loop — built on init but never read
- `SBResult` type, `sbReconstruct` function — only `MinLen` was used from `SupPathBFS`
- `SBBuf.PrevSt`, `PrevGen`, `SetPrev`, `GetPrev` — only used by removed `sbReconstruct`

Net: **-186 lines**, ~62 lines added.

## Performance Changes

1. **`SupPathBFS` simplified** — returns `int` (minLen) instead of `*SBResult`. Eliminated path reconstruction allocations (`[]Point` waypoints slice per call), prev-pointer storage (`[]int32` + `[]uint32` arrays in `SBBuf.Init`).

2. **`commandDirs` → `validDirs`** — old `commandDirs` allocated `[]Direction` slice via `make([]Direction, 0, 3)` on every call. Called inside `worstCasePlanRisk`'s combinatorial `walk()` — the innermost hot loop of `refinePlans` (53% of budget). Replaced with `validDirs` returning `[4]Direction` fixed array + count (zero allocations). Also unified duplicate `legalDirs` function.

3. **`abs(dx)` simplification** — collapsed 7-line branching `if dx != 0 { if dx < 0 { ... } else { ... } }` into `abs(dx) * 10`.

## Remaining Bottlenecks

1. **`refinePlans` / `simulateOneTurn` (53%)** — 10+ allocations per call (`make([]localBird)`, `make([]Point)` body copies, `NewBG`, `make([]bool)` × 4, `occExcept`). Called combinatorially (3^enemies × bots). Needs pre-allocated scratch buffers.

2. **`pathBFS` (28%)** — `map[uint64]bool` for seen states + `make([]Point, len(nb))` per queued node. Needs flat generation-stamped array and body arena.

3. **`planSupportJobs` (8%)** — `SupReachMulti` allocates `NewBG` for `tgtBG` per call. `SupPathBFS` still called per target per climber.
