# Gravity-Aware BFS — Implementation Notes

Bot: `agent/basic/src/plan.go`

## Architecture

```
Plan.Precompute()                    ← once, turn 1 (1s budget)
  ├─ computeLandY()                  ← fall destinations (wall-only, static)
  ├─ detectSurfaces()                ← platform segments (static, wall-grounded)
  ├─ buildSurfaceGraph()             ← fall + climb links between surfaces
  └─ precomputeAllPairsSurfDist()    ← Floyd-Warshall per body length → O(1) SurfDist

Plan.RebuildAppleMap()     ← per turn, before BFS

Plan.BFSFindAll(body)      ← per snake (up to 4× per turn)
  ├─ simulateFirstMove()   ← accurate body sim for step 1
  └─ (cell, ag) BFS        ← approximate for steps 2+
```

## Precomputation (turn 1)

### LandY — Fall Destination Map

For each free cell, the y-coordinate where it would land after free-falling (wall-only, no apples).

- **Algorithm**: bottom-up propagation per column, O(W×H)
- **Usage**: `computeFallWithBody`, `simulateFirstMove`

### Surfaces — Platform Segments

A surface = contiguous horizontal run of free cells with wall directly below.

- **Detection**: row-by-row scan, wall-only grounding (static, apple-independent)
- **Storage**: `Surfs[]` (list), `SurfAt[cell]` (cell → surface ID, -1 if none)

### Surface Graph — Platform Connections

Directed graph between surfaces with two link types:

**Fall links**: walk off surface edge → body falls to lower surface.
- Landing = `min(LandY)` across body columns (rigid body fall)
- Cost = bodyLen (turns to walk off before fall triggers)
- MinBody = bodyLen (need that many segments to clear the edge)
- Links generated per bodyLen from 2 to MaxAG

**Climb links**: go UP from edge, step sideways onto higher surface.
- Cost = h + 1 (h UP moves + 1 sideways)
- MinBody = h + 1 (body must span head to original surface)

## Per-Turn State

### Apple Bitmap

`appleMap []bool` indexed by cell. Rebuilt each turn via `RebuildAppleMap()`.

- **Reset**: O(ANum) — clears only previously set entries, not full map
- **Lookup**: O(1) — used by `isGroundedAt()` and `isApple()`

## BFS — Cell-Level Pathfinding

### State Model

State: `(cell, ag)` where:
- `cell` = flat cell index (head position)
- `ag` = above-ground counter (segments from head to nearest grounded segment)
  - ag=0: head is grounded (wall or apple below)
  - ag=N: N body segments between head and ground
  - ag≥bodyLen: fully airborne → fall

State space: `W×H × min(bodyLen, MaxAG)` ≈ 1350 × 33 = 44,550 max.

### First Step — Accurate Body Simulation

For the first BFS step, `simulateFirstMove()` uses the actual body:
1. Build new body (head moves, tail drops)
2. Check grounding across all body segments
3. If airborne: compute `min(LandY)` across all body columns → accurate multi-column fall
4. Return actual head cell + ag for the fallen body

Uses scratch buffer `firstMoveBuf []int` (pre-allocated in `Precompute`).

### Subsequent Steps — (cell, ag) Approximation

For each direction d from state (cell, ag):
1. `nc = Nb[cell][d]` — neighbor lookup (wall/OOB filtered)
2. If `isGroundedAt(nc)`: nag = 0
3. Else: nag = ag + 1
4. If nag ≥ bodyLen:
   - **Apple eating**: if nc is apple, nag = ag (growth prevents fall)
   - **Fall**: `computeFallWithBody(nc, d, bodyLen)` → landing cell + post-fall ag
5. Record result if first visit to (finalCell, nag)

### Fall Computation — computeFallWithBody

When the snake becomes airborne (ag ≥ bodyLen):
1. Body extends opposite to move direction
2. Landing y = `min(LandY)` across all body columns (rigid body)
3. Apples in fall path intercepted via `clipFallByApples()`
4. Post-fall ag = index of first grounded segment (head-to-tail scan)

### Neck Check

First BFS step blocks backward movement (head into body[1]). Uses `neckOf(body)` helper.

## Surface Graph Pathfinding

### All-Pairs Precomputation (Floyd-Warshall)

`precomputeAllPairsSurfDist()` runs Floyd-Warshall for each body length 1..MaxAG during `Precompute()`. Stores flat N×N distance matrices in `surfAPD[bodyLen]`.

- **Cost**: O(MaxAG × N³) once at init (~264K ops for N=20, negligible)
- **Memory**: MaxAG × N² ints (~13K for N=20)

### SurfDist — O(1) Lookup

Returns precomputed shortest path distance between two surfaces, filtered by body length. Single array lookup into `surfAPD[bodyLen][from*surfN+to]`.

### EstimateDist — Fast Approximate Distance

Combines body simulation (first step) + surface graph (rest):
```
total = 1 + walk_to_edge + SurfDist(landing, target_surface) + walk_from_edge
```

Used in the decision pipeline for distance estimation. With O(1) SurfDist, the cost is dominated by `simulateFirstMove` (O(bodyLen) × 4 directions).

**Primary use case**: cheap opponent distance estimation for race filtering — avoids running full BFS for enemy snakes.

## Known Limitations

1. **Multi-column body falls after step 1**: `computeFallWithBody` assumes body extends in a straight line opposite to movement. Real body shapes may differ after multiple turns.

2. **Distance underestimate**: BFS may report fewer steps than the real game. The `ag` model doesn't track horizontal gap distance separately from vertical climb distance.

3. **No self-collision avoidance**: BFS doesn't check if the head would collide with its own body segments.

4. **Apple growth not chained**: eating apple A to grow and then reach apple B is partially modeled (single-step apple eating), but multi-apple chains are not.

5. **Surface graph is static**: doesn't account for dynamic support from apples or other snake bodies.

## Performance Budget

| Operation | Cost | Per turn |
|-----------|------|----------|
| RebuildAppleMap | O(ANum) | 1× |
| simulateFirstMove | O(bodyLen) | 4× per snake |
| BFSFindAll | O(states × 4 × 1) | 1× per snake |
| isGroundedAt / isApple | O(1) bitmap | thousands |
| SurfDist | O(1) table lookup | on demand |
| EstimateDist | O(bodyLen × 4) | on demand |
| precomputeAllPairsSurfDist | O(MaxAG × N³) | 1× at init |

Total per-turn budget: ~2-5ms on arena (Haswell single-core), well within 50ms limit.

## Memory Layout

All large buffers are heap-allocated slices sized to actual grid dimensions (`n = W*H`), not worst-case constants. Allocated once in `Precompute()`.

| Buffer | Type | Size |
|--------|------|------|
| LandY | `[]int` | n |
| SurfAt | `[]int` | n |
| appleMap | `[]bool` | n |
| appleMapPrev | `[]int` | MaxAp |
| visitGen | `[]uint16` | n × MaxAG (flat, `cell*MaxAG+ag`) |
| firstMoveBuf | `[]int` | MaxSeg |
| surfAPD | `[][]int` | (MaxAG+1) × N² per body length |
| fallSeen | `[]bool` | N (surface count) |
| queue | `[]bfsNode` | cap: n × 4 |

## Constants

```
MaxW     = 45    max grid width
MaxH     = 30    max grid height
MaxCells = 1350  W×H (upper bound, actual n = W*H)
MaxAG    = 33    max above-ground counter (capped for memory)
MaxSeg   = 256   max body segments per snake
MaxAp    = 64    max apples
```
