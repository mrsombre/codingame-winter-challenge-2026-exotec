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
