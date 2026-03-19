package main

// --- Phase: Safety (rewritten) ---
// Evaluates every direction for every friendly snake in a single pass.
// Stores per-snake results. Then overrides, deconflicts friendlies, validates.

const safetyFloodCramped = 6

// Direction tiers (higher = safer).
const (
	tierDead    = 0 // wall, OOB, gravity death, fatal beheading, enemy body
	tierBehead  = 1 // beheading for len>3 (lose segment but survive)
	tierDeadEnd = 2 // alive but flood=0 (no next move)
	tierCramped = 3 // alive but tight space (flood < threshold)
	tierRisky   = 4 // enemy head could reach same cell next turn
	tierSafe    = 5 // clear
)

type dirEval struct {
	tier  int
	flood int
}

func betterEval(a, b dirEval) bool {
	return a.tier > b.tier || (a.tier == b.tier && a.flood > b.flood)
}

// safetyEvals stores per-snake direction evaluations.
type safetyEvals [MaxPSn][4]dirEval

func (d *Decision) phaseSafety() {
	g := d.G
	sim := NewSim(g)
	sim.RebuildAppleMap()

	var evals safetyEvals

	// --- Step 1: Evaluate all directions for all my snakes ---
	blocked := make([]bool, g.NCells)
	visited := make([]bool, g.NCells)
	queue := make([]int, g.NCells)

	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}
		head := sn.Body[0]
		if head < 0 || head >= g.NCells {
			continue
		}
		neck := neckOf(sn.Body)

		// Build blocked map: all other snake bodies (excl. movable tails)
		clearBlocked(blocked, g)
		markOtherBodies(blocked, g, sim, snIdx)

		for dir := 0; dir < 4; dir++ {
			evals[si][dir] = evalDir(g, sim, sn, snIdx, dir, head, neck, blocked, visited, queue, d.OpSnakes)
		}
	}

	// --- Step 2: Override unsafe assigned directions ---
	for si := range d.MySnakes {
		sn := &g.Sn[d.MySnakes[si]]
		if !sn.Alive || sn.Len == 0 {
			continue
		}

		assigned := d.AssignedDir[si]
		if evals[si][assigned].tier >= tierSafe {
			continue // fine
		}

		// Find best alternative
		best := assigned
		for dir := 0; dir < 4; dir++ {
			if betterEval(evals[si][dir], evals[si][best]) {
				best = dir
			}
		}
		d.AssignedDir[si] = best
	}

	// --- Step 3: Deconflict friendly snakes ---
	d.safetyFriendly(&evals)

	// --- Step 4: No backward ---
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len < 2 {
			continue
		}
		if d.AssignedDir[si] == Do[sn.Dir] {
			// Pick best non-backward
			best := d.AssignedDir[si]
			for dir := 0; dir < 4; dir++ {
				if dir == Do[sn.Dir] {
					continue
				}
				if betterEval(evals[si][dir], evals[si][best]) || best == Do[sn.Dir] {
					best = dir
				}
			}
			d.AssignedDir[si] = best
		}
	}
}

// evalDir evaluates a single direction for a snake. One function, all checks.
func evalDir(
	g *Game, sim *Sim, sn *Snake, snIdx int, dir int,
	head, neck int,
	blocked, visited []bool, queue []int,
	opSnakes []int,
) dirEval {
	nb := g.Nbm[head][dir]

	// Can't move there at all
	if nb < 0 || nb == neck {
		return dirEval{tierDead, -1}
	}

	// Out of game grid
	if !g.IsInGrid(nb) {
		return dirEval{tierDead, 0}
	}

	// Enemy body collision (immediate death for len-3, beheading for longer)
	if blocked[nb] {
		if sn.Len <= 3 {
			return dirEval{tierDead, 0}
		}
		return dirEval{tierBehead, 0}
	}

	// Simulate the move (handles eating, self-collision)
	newBody, alive := sim.simulateMove(sn.Body, dir)
	if !alive {
		return dirEval{tierDead, 0}
	}

	beheaded := len(newBody) < sn.Len
	if beheaded {
		if sn.Len <= 3 {
			return dirEval{tierDead, 0}
		}
		return dirEval{tierBehead, 0}
	}

	// Apply gravity
	bodycp := make([]int, len(newBody))
	copy(bodycp, newBody)

	if !sim.applyGravity(bodycp) {
		// Check if grounded by another snake
		if !isGroundedByOtherSnake(g, bodycp, snIdx) {
			return dirEval{tierDead, 0}
		}
		// Restore pre-gravity body for flood (gravity failed but we're supported)
		copy(bodycp, newBody)
	}

	newHead := bodycp[0]

	// Flood count: temporarily block own post-move body (preserve existing marks)
	var added [MaxSeg]int
	addedN := 0
	for _, c := range bodycp[1:] {
		if c >= 0 && c < g.NCells && !blocked[c] {
			blocked[c] = true
			added[addedN] = c
			addedN++
		}
	}
	flood := bfsFlood(g, blocked, visited, queue, newHead)
	for i := 0; i < addedN; i++ {
		blocked[added[i]] = false
	}

	// Enemy head-on risk: can any enemy head reach the same cell?
	risky := false
	for _, opIdx := range opSnakes {
		op := &g.Sn[opIdx]
		if !op.Alive || op.Len == 0 {
			continue
		}
		opHead := op.Body[0]
		if opHead < 0 || opHead >= g.NCells {
			continue
		}
		opNeck := neckOf(op.Body)

		for od := 0; od < 4; od++ {
			opNext := g.Nbm[opHead][od]
			if opNext < 0 || opNext == opNeck {
				continue
			}
			if opNext == nb {
				// We're bigger → they die, we survive (or lose 1 segment)
				if sn.Len > 3 && op.Len <= 3 {
					continue
				}
				risky = true
				break
			}
		}
		if risky {
			break
		}
	}

	// Classify
	switch {
	case flood == 0:
		return dirEval{tierDeadEnd, 0}
	case flood < safetyFloodCramped:
		if risky {
			return dirEval{tierCramped, flood} // downgrade risky+cramped
		}
		return dirEval{tierCramped, flood}
	case risky:
		return dirEval{tierRisky, flood}
	default:
		return dirEval{tierSafe, flood}
	}
}

// safetyFriendly deconflicts friendly snakes targeting the same cell.
func (d *Decision) safetyFriendly(evals *safetyEvals) {
	g := d.G
	myN := len(d.MySnakes)
	if myN <= 1 {
		return
	}

	nextHead := make([]int, myN)
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			nextHead[si] = -1
			continue
		}
		nextHead[si] = g.Nbm[sn.Body[0]][d.AssignedDir[si]]
	}

	for i := 0; i < myN; i++ {
		if nextHead[i] < 0 {
			continue
		}
		for j := i + 1; j < myN; j++ {
			if nextHead[j] < 0 || nextHead[j] != nextHead[i] {
				continue
			}
			// Collision — loser is the one with worse eval for assigned dir
			loser := j
			ei := evals[i][d.AssignedDir[i]]
			ej := evals[j][d.AssignedDir[j]]
			if betterEval(ej, ei) {
				loser = i
			}

			// Find best alternative for loser
			snIdx := d.MySnakes[loser]
			sn := &g.Sn[snIdx]
			best := d.AssignedDir[loser]
			bestEval := dirEval{tierDead, -1}

			for dir := 0; dir < 4; dir++ {
				nb := g.Nbm[sn.Body[0]][dir]
				if nb < 0 || nb == neckOf(sn.Body) {
					continue
				}
				// Don't pick the collision cell
				if nb == nextHead[i] || nb == nextHead[j] {
					continue
				}
				if betterEval(evals[loser][dir], bestEval) {
					bestEval = evals[loser][dir]
					best = dir
				}
			}

			d.AssignedDir[loser] = best
			nextHead[loser] = g.Nbm[sn.Body[0]][best]
		}
	}
}

// --- Helpers ---

func clearBlocked(blocked []bool, g *Game) {
	for i := range blocked {
		blocked[i] = false
	}
}

// markOtherBodies marks all other snakes' body cells as blocked.
func markOtherBodies(blocked []bool, g *Game, sim *Sim, excludeIdx int) {
	for i := 0; i < g.SNum; i++ {
		other := &g.Sn[i]
		if i == excludeIdx || !other.Alive || other.Len == 0 {
			continue
		}
		tailIdx := other.Len - 1
		movable := sim.isTailMovable(other)
		for bi, c := range other.Body {
			if c >= 0 && c < g.NCells {
				if bi == tailIdx && movable {
					continue
				}
				blocked[c] = true
			}
		}
	}
}

// isGroundedByOtherSnake checks if any body cell has another snake below it.
func isGroundedByOtherSnake(g *Game, body []int, excludeIdx int) bool {
	for _, c := range body {
		if c < 0 || c >= g.NCells {
			continue
		}
		below := c + g.Stride
		if below < 0 || below >= g.NCells {
			continue
		}
		for i := 0; i < g.SNum; i++ {
			if i == excludeIdx {
				continue
			}
			sn := &g.Sn[i]
			if !sn.Alive {
				continue
			}
			for _, bc := range sn.Body {
				if bc == below {
					return true
				}
			}
		}
	}
	return false
}

// bfsFlood counts reachable cells from start, blocked by walls and blocked[].
func bfsFlood(g *Game, blocked, visited []bool, queue []int, start int) int {
	if start < 0 || start >= g.NCells || !g.Cell[start] || blocked[start] {
		return 0
	}
	queue[0] = start
	head, tail := 0, 1
	visited[start] = true

	for head < tail {
		cur := queue[head]
		head++
		for d := 0; d < 4; d++ {
			nb := g.Nbm[cur][d]
			if nb < 0 || blocked[nb] || visited[nb] {
				continue
			}
			visited[nb] = true
			queue[tail] = nb
			tail++
		}
	}

	for i := 0; i < tail; i++ {
		visited[queue[i]] = false
	}
	return tail
}
