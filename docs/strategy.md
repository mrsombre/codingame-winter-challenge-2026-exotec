# Winning strategies for CodinGame's SnakeByte challenge

**The most effective approach for SnakeByte (Winter Challenge 2026 – Exotec) combines gravity-aware BFS pathfinding, greedy-to-Hungarian resource assignment, and flood-fill territory control — layered into a per-turn decision pipeline that runs under 50ms.** This mirrors the strategies that consistently win similar CodinGame contests: the Spring 2020 Pac-Man champion used a genetic algorithm with Voronoi scoring, the Hypersonic champion used beam search on grid states, and Battlesnake's world #1 (Shapeshifter) paired minimax with bitboard flood fill. The game's distinctive mechanic — gravity plus snake-body-as-platform — demands pathfinding adaptations that go well beyond standard grid BFS.

The contest launched March 10, 2026 and is still live, so no post-mortems exist yet. However, the game's mechanics (multi-agent snake control, gravity, collectible power sources, simultaneous movement) map closely to several well-studied problem families. Below is a complete algorithmic playbook drawn from CodinGame post-mortems, Battlesnake competitions, MAPF research, and competitive programming practice.

---

## Gravity-aware pathfinding changes everything

Standard 4-directional BFS fails immediately in SnakeByte because unsupported agents fall. The state space must encode gravity constraints. The most practical approach for competitive programming is a **gravity-constrained BFS** where transitions enforce physics:

A cell is "supported" if it has a wall, platform, or snake body segment directly below. If unsupported, the snake falls until hitting support or dying (falling off the bottom kills the snake). Horizontal movement is only valid from supported positions. The snake's own body acts as scaffolding — longer snakes can bridge gaps and reach elevated platforms that shorter snakes cannot.

The state representation becomes `(x, y, velocity_y)` or, if gravity is instantaneous (fall completes within one tick), simply `(x, y)` with constrained neighbor generation. The neighbor function returns only `[(x, y+1)]` (forced fall) when unsupported, and `{left, right, up-if-climbable}` when on solid ground. For a 30×20 grid, this gives roughly **600 states per agent** — trivially searchable within time limits.

**A\* with platform-graph preprocessing** offers an alternative: precompute a graph where nodes are walkable platform segments and edges represent jump/fall trajectories between them. Standard A\* on this compressed graph runs extremely fast, though it trades flexibility for speed. For SnakeByte, where body positions change every turn, raw BFS on the gravity-constrained grid is likely more practical than maintaining a dynamic platform graph.

**Snake body as state**: The snake's body is a dynamic obstacle. The key optimization is tracking `(head_x, head_y, time)` in BFS and computing body occupancy from the initial body plus the path taken so far. At time `t`, tail segments `[0..t-1]` have been vacated. A critical rule: **don't revisit (x, y) if already visited at an earlier time**, since earlier arrival means a shorter body blocking fewer cells.

---

## Multi-agent coordination: from simple to optimal

With 2–5 friendly snakebots to coordinate, you need algorithms that prevent agents from competing with each other while remaining fast enough for real-time play.

**Prioritized Planning (simplest, most practical)** assigns a fixed ordering to your snakes, plans each sequentially using space-time A\*, and treats higher-priority snakes' planned paths as obstacles via a reservation table — a hash map of `(x, y, t) → occupied`. Each subsequent snake routes around already-reserved cells. Complexity is **O(n × W × H × T)** where T is the planning horizon. For 4 agents on a 30×20 grid with horizon 15, this runs in under 5ms. The approach is incomplete (bad orderings can fail), but in practice works well for small agent counts.

**Windowed Hierarchical Cooperative A\* (WHCA\*)** improves on this by limiting cooperative planning to a fixed-depth window (8–16 steps) and using abstract distance heuristics beyond the window. David Silver's original paper showed **100 agents initializing in under 100ms** with window=16 on 32×32 grids. For SnakeByte's handful of agents, WHCA\* is overkill but remains the gold standard for real-time multi-agent coordination.

**Conflict-Based Search (CBS)** is the optimal MAPF algorithm. It maintains a constraint tree: when two agents collide, it branches into two child nodes, each constraining one agent. The low level runs single-agent A\* per constraint set. CBS handles ~200 agents optimally on 256×256 maps within one minute — but for competitive programming with 2–4 agents, it's both optimal and fast. The bounded-suboptimal variant **ECBS** trades optimality for speed when needed.

For SnakeByte specifically, **prioritized planning is the recommended starting point**, with CBS as an upgrade if agent conflicts become frequent at higher leagues.

---

## Resource assignment: matching agents to power sources

The core decision each turn is: which snake goes after which power source? Three approaches exist on a spectrum from simple to optimal.

**Greedy sequential assignment** iterates through (agent, resource) pairs sorted by value/distance ratio, assigning the best available pair and removing both from consideration. At **O(A × R)** per turn, it's essentially free computationally. The scoring function should be: `value = base_value / (1 + gravity_bfs_distance) × safety_factor - opponent_proximity_penalty`. Resources at higher elevations that only your longer snakes can reach are effectively exclusive — these deserve a significant bonus.

**The Hungarian algorithm** solves optimal one-to-one matching in **O(n³)**. With 5 agents and 20 resources, this completes in microseconds. Build a cost matrix where `C[i][j]` = gravity-aware BFS distance from agent i to resource j, apply the algorithm, and each agent gets its optimal target. The cp-algorithms.com implementation is clean and competitive-programming-ready. For dynamic reallocation when a resource is consumed, CMU's Dynamic Hungarian variant incrementally updates the assignment without full recomputation.

**Bitmask DP** is ideal for SnakeByte's small agent count. With ≤5 agents, enumerate all `2^5 = 32` subsets of agents, and for each subset find the best resource assignment. This captures interactions (e.g., two agents shouldn't target nearby resources when one could handle both) at negligible cost.

**Voronoi-based territory division** complements any assignment algorithm. Run multi-source BFS from all agent heads simultaneously; each cell is claimed by whichever agent reaches it first. Resources in each agent's Voronoi cell become its responsibility. Include opponent snakes as competing BFS sources — contested resources (closer to opponents) get deprioritized. The entire computation is **O(W × H)**, trivially fast.

---

## What Battlesnake's best bots teach us about evaluation

Battlesnake competitions, where snake AIs compete on grids with food collection and elimination mechanics, have produced deeply tested strategies directly applicable to SnakeByte.

**Shapeshifter**, the 2022 world champion, used **Minimum Combination Search (MCS)** — a novel multiplayer minimax variant with constant branching factor. Instead of exponentially branching for all opponent moves, MCS selects one move per opponent using domain heuristics, then combines them. Combined with **bitboard-based flood fill** (shift entire bitboard per direction, OR with frontier, mask obstacles) and **Best Node Search** (binary search over evaluation scores instead of computing exact minimax), Shapeshifter dominated with >50% win rate across all official competitions.

**Robosnake** (93% win rate, 42/45 matches) revealed the critical phase transition: **as snakes grow, the game shifts from food-chase to Tron-style territory control**. Its evaluation function weighted food distance by current health (when full, food is ignored; when starving, food overwhelms all other factors). Flood fill space count versus snake length detected lethal dead ends. Center-board preference forced opponents toward walls where they're easier to trap.

The **Asymptotic Labs RL bot** (topped global leaderboard) used a hybrid: a 4-layer CNN trained with PPO self-play for policy, overridden by alpha-beta search when it detected definite wins or losses in the RL choice. Their key insight: **"We don't pretend to know the optimal strategy — we know winning is good and losing is bad. Let RL find the rest."** The minimal -1/0/+1 reward function outperformed heavily engineered reward shaping.

The universal Battlesnake evaluation function includes these weighted factors:

- **Flood fill space** (high weight) — reachable squares versus snake length; if reachable < length, the move is likely lethal
- **Food distance** (variable weight) — scales with hunger/health urgency
- **Relative snake length** (medium) — larger snakes win head-on collisions
- **Center proximity** (low-medium) — better reach to future resources
- **Tail accessibility** (medium) — can you reach your own tail as an escape route?
- **Board control percentage** (medium) — Voronoi or flood-fill-based territory estimate

---

## Proven CodinGame contest algorithms ranked by relevance

Past CodinGame challenges provide a clear hierarchy of what works. The table below ranks approaches by their track record in contests with mechanics similar to SnakeByte:

| Algorithm | Champion usage | Best for | SnakeByte fit |
|-----------|---------------|----------|---------------|
| Genetic Algorithm | Saelyos (#1, Spring 2020 Pac-Man) | Multi-agent collection, fog of war | **Excellent** — encode genes as target cells, co-evolve against opponent |
| Beam Search | pb4 (#1, Fall 2020), Izanexis (#1, Hypersonic) | Single-agent optimization, sequential decisions | Good for per-snake planning |
| MCTS / DUCT | reCurse (#1, Spring 2021, AlphaZero-style) | Adversarial games with complex evaluation | Good for opponent modeling |
| Simulation + Heuristics | Multiple top-50 finishers | Quick prototyping, gravity games | **Excellent baseline** |
| Pure RL | reCurse (required custom infrastructure) | Perfect-information 1v1 | Impractical during a 10-day contest |

**Saelyos's Spring 2020 Pac-Man victory** is the closest analog to SnakeByte — a multi-agent collection game on a grid. Key techniques: genes encoded as **target cells** (not raw actions), enabling the GA to adapt to dynamic situations; **bitboard simulation** for speed; **Voronoi-weighted scoring** where pellet value decayed as `0.9^distance`; and co-evolution maintaining separate populations for self and opponent. The balance between Voronoi scoring and raw point collection was reportedly "worth a lot of Elo."

**pb4's Hypersonic and Fall 2020 victories** demonstrated that **beam search alone can reach top 10** on grid games. The critical finding: beam width 400 outperformed width 600 (larger beams introduced noise), and beyond ~30,000 simulations per turn, faster code showed zero Elo benefit. Opponent modeling via a fast pre-search (+30% winrate) mattered more than raw search depth.

**The universal lesson across all post-mortems: bug-free simulation is the #1 priority.** Virtual Atom jumped from rank 400 to rank 100 solely by fixing three simulation bugs discovered through replay parsing. Multiple champions emphasize simulation correctness over algorithmic sophistication.

---

## The recommended per-turn decision pipeline

Based on all research, here is a concrete architecture optimized for SnakeByte's mechanics, ordered by implementation priority:

**Phase 1 — Gravity-aware BFS** (O(W×H) per agent). From each snake head, run BFS with gravity-constrained transitions. Output: distance from each agent to every reachable cell. This is the foundation everything else builds on.

**Phase 2 — Influence mapping** (O(W×H)). Propagate positive influence from friendly agents, negative from opponents, with BFS-based distance decay. Identify safe zones (deep positive influence), contested frontlines (sign flips), and opponent territory. Resources in safe zones get priority.

**Phase 3 — Resource scoring** (O(A×R)). For each agent-resource pair: `score = base_value / (1 + bfs_distance) × reachability_gate × safety_factor × clustering_bonus - opponent_penalty`. Elevated resources reachable only by longer snakes get a height bonus. Clustering bonus rewards resources near other uncollected resources (efficient sequential collection).

**Phase 4 — Assignment** (O(n³) or O(A×R)). Start with greedy allocation; upgrade to Hungarian when tuning. Use Voronoi as a sanity check — never send two agents into the same Voronoi cell unless resource density justifies it.

**Phase 5 — Safety check**. For each planned move, verify the agent can still reach its own tail (escape route) after execution. If not, switch to survival mode: follow the longest path toward the tail. This two-phase approach (greedy + safety) is the most reliable pattern from both Battlesnake and academic snake-solver research.

**Phase 6 — Execute and output moves.**

Start with phases 1, 3, and 4 (greedy variant). This alone will be competitive in lower leagues. Layer in influence maps, Hungarian assignment, and opponent modeling as you climb.

---

## Key resources and repositories

The game's official referee is open-source at `github.com/CodinGame/WinterChallenge2026-Exotec`, with a brutaltester-compatible fork at `github.com/aexg/WinterChallenge2026-Exotec` for local testing. The contest forum at `forum.codingame.com/t/winter-challenge-2026/207974` contains clarifications on collision rules (moving into a cell just vacated by another snake's tail is legal) and gravity mechanics.

For algorithm implementations: CBS/ICBS in Python at `github.com/Stepan-Makarenko/Multi-agent-pathfinding-CBS-ICBS`; the Hungarian algorithm at `cp-algorithms.com/graph/hungarian-algorithm.html`; Battlesnake's official algorithm guide at `docs.battlesnake.com/guides/useful-algorithms`; Shapeshifter's detailed writeup at `notpeerreviewed.com/blog/battlesnake/`; and the Asymptotic Labs RL post-mortem on Medium. Past CodinGame champion code: Saelyos's Spring 2020 GA at `github.com/Saelyos/Spring-Challenge-2020`, pb4's beam search at `github.com/pb4git/Fall-Challenge-2020`, and Agade's opponent-modeling approach at `github.com/Agade09/Agade-Fall2020-Challenge-Postmortem`.

## Conclusion

Three insights emerge that are non-obvious from studying these competitions. First, **the gravity mechanic transforms resource valuation** — elevated power sources reachable only by longer snakes are far more valuable than their base worth suggests, because they're effectively exclusive. Build your scoring function around gravity-aware BFS distance, not Manhattan distance. Second, **opponent modeling consistently delivers +25–30% winrate improvement** across every CodinGame contest studied, yet most competitors skip it; even a crude 10ms fast-search of the opponent's likely moves, feeding into your own planning, is among the highest-value additions. Third, the game will likely shift phases — early resource-racing transitioning to territory-denial as snakes grow — and the bot that detects and adapts to this transition (weighting flood-fill space over food distance as snakes lengthen) will outperform one locked into a single strategy. The winning formula is not one algorithm but a layered pipeline: gravity BFS as the foundation, greedy-to-Hungarian assignment as the decision layer, flood fill as the safety net, and opponent prediction as the edge.