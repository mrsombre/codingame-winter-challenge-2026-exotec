# Pipeline Ideas

## Current Pipeline
1. **BFS** — sim moves to close surfaces and apples, know what we can actually reach and how
2. **Heat map** — how many turns each apple is away (estimated), contestation with enemies
3. **Constellation assign** — assign distinct constellations to best snakes, one per snake
4. **Free snake targeting** — snakes that got a constellation but can't eat yet navigate toward it
5. **Safety** — don't die

## TODO

### Step 3: Constellation Assignment Improvements
- [x] Don't reassign same constellation to multiple snakes — distinct for all
- [x] Even bad/enemy-contested constellations are ok — pick closer actually
- [x] If all good ones taken, take whatever's left
- [x] Distance dominates scoring (scoreDistPenalty=30 >> scoreSizeBonus=10)

### Step 4: Free Snake Apple Targeting
- [x] After constellation assigned, find closest apple the snake can actually eat
- [x] Apple must NOT be in a cluster assigned to a teammate
- [x] Apples from unassigned clusters or enemy-contested clusters — ok, eat freely
- [ ] If occasional apple on the way by movement — eat it (no special targeting needed)
- [x] If no actually eatable apples — find surf-surf link, position head as close as possible to target surface via BFS link
- [x] Other snakes sometimes lurk nearby — we can jump from their bodies (already handled by BFS sim)

### Step 5: Safety Rework
- [ ] **Biggest NO**: dead end where we can't turn — be extra careful in tight spaces
- [x] No falling off map — avoid head outside grid, allow as last resort
- [ ] No crashing with head lost (beheading)
- [ ] If in bad situation — try move toward own tail, sometimes we can just rotate until safe
- [x] **Allow head-on collision** if our body > 3 and enemy is 3-len — they die, we survive beheading
- [x] **Allow apple contest collision** — if we target apple and enemy too, allow it. We both lose apple but at least we don't give it free
- [ ] Tight space detection — flood fill already exists, but needs smarter escape logic

### Step 6: Intra-Constellation Path Planning (RESEARCH)

The problem: we pick the closest apple in the constellation, but with 5-10 apples to collect, the ORDER matters enormously. Wrong order = eat a surface apple → fall → lose access to remaining apples → wasted turns recovering.

**Key questions:**
1. Which apple to eat first/last?
2. When to walk on an apple-surface vs eat it?
3. After clearing a constellation, where are we? Can we U-turn toward the next target?

**Apple eating order rules (priority):**
- [ ] **Free apples first**: eat apples that DON'T support any surface — no risk
- [ ] **Edge-to-center**: eat from constellation edges inward — don't strand yourself in the middle
- [ ] **Surface apples last**: apples supporting SurfApple surfaces should be eaten last, when you have alternate support or it's the only one left
- [ ] **Exit-aware**: the LAST apple you eat should leave you pointing toward the next constellation

**When to walk vs eat:**
- [ ] If apple supports a surface you NEED to reach other apples — walk on it, eat later
- [ ] If apple is a dead-end (no other apples beyond it) — eat it on the way back
- [ ] If eating causes a 1-cell fall to another surface — ok, eat it
- [ ] If eating causes fall into void/far from cluster — DON'T eat, walk over

**Post-constellation planning:**
- [ ] Before eating last apple, check: where does the snake end up? Is it near the next constellation?
- [ ] Consider U-turn: eat last apple in a direction that points toward next target
- [ ] If constellation cleared and snake is in a corner with nothing — tail-chase or navigate to nearest uncleared cluster

**Algorithm approach: Mini-TSP within constellation**
- Constellation has N apples (typically 2-10)
- For N <= 8: can brute-force all permutations (8! = 40320)
- For each permutation: simulate path, check surface validity, compute total turns + exit position
- Pick permutation with best (total turns + distance to next constellation)
- Constraint: some orderings are invalid (eating apple X before Y destroys path to Y)
- This is a constrained TSP — can prune invalid orderings early

**Simpler heuristic (cheaper):**
- Build dependency graph: apple A depends on apple B if eating B destroys the surface needed to reach A
- Topological sort: eat apples with no dependents first, dependents last
- Within same dependency level: eat closest first
- This gives a valid ordering without brute-force

**Implementation location:** new phase between scoring and safety, or within scoring's `constBestApple`

### Done
- [x] SimBFSApples: remove eaten apple from map before gravity (was using eaten apple as ground)
- [x] constBestApple: skip apples that support head surface (prefer walking over eating)
