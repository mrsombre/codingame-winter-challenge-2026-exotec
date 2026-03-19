package main

// --- Phase 5: Safety ---

const (
	safetyFloodMinAbs = 3 // absolute minimum flood to not be "dead end"
	safetyFloodCramped = 6 // below this = cramped
)

// SafetyScratch holds pre-allocated buffers for phaseSafety.
type SafetyScratch struct {
	blocked     []bool // NCells: walls + other snake bodies
	blockedPrev []int  // dirty-list for O(k) clear
	blockedN    int

	visited []bool // NCells: flood fill visited
	queue   []int  // NCells: flood fill queue

	floodByDir [4]int // per-direction flood result for current snake
	bodyBuf    []int  // MaxSeg: scratch for body copy
}

func (d *Decision) ensureSafety() {
	if d.Safety.blocked != nil {
		return
	}
	n := d.G.NCells
	d.Safety = SafetyScratch{
		blocked:     make([]bool, n),
		blockedPrev: make([]int, MaxASn*MaxSeg+MaxSeg),
		visited:     make([]bool, n),
		queue:       make([]int, n),
		bodyBuf:     make([]int, MaxSeg),
	}
}

// floodCount counts reachable free cells from start via BFS.
// Blocked by walls and sc.blocked[]. Self-cleans visited.
func floodCount(g *Game, sc *SafetyScratch, start int) int {
	if start < 0 || start >= g.NCells || !g.Cell[start] || sc.blocked[start] {
		return 0
	}

	q := sc.queue
	q[0] = start
	head, tail := 0, 1
	sc.visited[start] = true

	for head < tail {
		cur := q[head]
		head++
		for d := 0; d < 4; d++ {
			nb := g.Nbm[cur][d]
			if nb < 0 || sc.blocked[nb] || sc.visited[nb] {
				continue
			}
			sc.visited[nb] = true
			q[tail] = nb
			tail++
		}
	}

	// Clear visited (only cells we touched)
	for i := 0; i < tail; i++ {
		sc.visited[q[i]] = false
	}

	return tail // count of reachable cells
}

// buildBlockedForSnake marks all body cells EXCEPT snake snIdx as blocked.
// Excludes movable tails (tails that will vacate because snake won't eat).
func (d *Decision) buildBlockedForSnake(sim *Sim, sc *SafetyScratch, snIdx int) {
	g := d.G
	// Clear previous
	for i := 0; i < sc.blockedN; i++ {
		sc.blocked[sc.blockedPrev[i]] = false
	}
	sc.blockedN = 0

	for i := 0; i < g.SNum; i++ {
		other := &g.Sn[i]
		if i == snIdx || !other.Alive || other.Len == 0 {
			continue
		}
		tailIdx := other.Len - 1
		movable := sim.isTailMovable(other)
		for bi, c := range other.Body {
			if c < 0 || c >= g.NCells {
				continue
			}
			if bi == tailIdx && movable {
				continue
			}
			if sc.blocked[c] {
				continue
			}
			sc.blocked[c] = true
			sc.blockedPrev[sc.blockedN] = c
			sc.blockedN++
		}
	}
}

// isGroundedByOtherSnake checks if any cell in body has another snake's body below it.
func isGroundedByOtherSnake(g *Game, body []int, excludeSnIdx int) bool {
	for _, c := range body {
		if c < 0 || c >= g.NCells {
			continue
		}
		below := c + g.Stride
		if below < 0 || below >= g.NCells {
			continue
		}
		for i := 0; i < g.SNum; i++ {
			if i == excludeSnIdx {
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

func (d *Decision) phaseSafety() {
	d.ensureSafety()
	sim := NewSim(d.G)
	sim.RebuildAppleMap()

	d.phaseSafetyLayer1(sim)
	d.phaseSafetyLayer2()
	d.phaseSafetyFriendly()
	d.phaseSafetyValidate()
}

// phaseSafetyFriendly prevents our own snakes from colliding head-on.
// If two friendly snakes will move to the same cell, the one with worse
// flood count changes direction.
func (d *Decision) phaseSafetyFriendly() {
	g := d.G
	sc := &d.Safety
	myN := len(d.MySnakes)
	if myN <= 1 {
		return
	}

	// Compute next head cell for each snake
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
			// Collision! The snake with worse flood changes direction
			loser := j
			fi := sc.floodByDir[d.AssignedDir[i]]
			fj := sc.floodByDir[d.AssignedDir[j]]
			if fj > fi {
				loser = i
			}

			// Find best alternative for loser
			snIdx := d.MySnakes[loser]
			sn := &g.Sn[snIdx]
			head := sn.Body[0]
			neck := neckOf(sn.Body)
			bestDir := d.AssignedDir[loser]
			bestFlood := -1

			for dir := 0; dir < 4; dir++ {
				nb := g.Nbm[head][dir]
				if nb < 0 || nb == neck {
					continue
				}
				// Don't pick the collision cell
				if nb == nextHead[i] || nb == nextHead[j] {
					continue
				}
				flood := sc.floodByDir[dir]
				if flood > bestFlood {
					bestFlood = flood
					bestDir = dir
				}
			}

			d.AssignedDir[loser] = bestDir
			nextHead[loser] = g.Nbm[sn.Body[0]][bestDir]
		}
	}
}

// phaseSafetyValidate ensures no snake is assigned the backward direction.
// Backward = opposite of current facing. Engine ignores backward commands
// (treats as forward), which wastes a turn.
func (d *Decision) phaseSafetyValidate() {
	g := d.G
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}
		if d.AssignedDir[si] == Do[sn.Dir] {
			d.AssignedDir[si] = fallbackDir(g, sn)
		}
	}
}

// Direction safety tier (lower = worse).
const (
	tierDead     = 0 // falls off map, len-3 beheading, no valid moves
	tierBehead   = 1 // beheading (lose segment) — big no
	tierBlocked  = 2 // moves into other snake body
	tierDeadEnd  = 3 // survives but 0 flood (no next move)
	tierCramped  = 4 // survives but very tight space
	tierSafe     = 5 // good flood, safe move
)

type dirEval struct {
	tier  int
	flood int
}

// phaseSafetyLayer1 evaluates each direction by safety tier and overrides
// AssignedDir when the assigned direction is worse than alternatives.
func (d *Decision) phaseSafetyLayer1(sim *Sim) {
	g := d.G
	sc := &d.Safety

	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if sn.Len == 0 || len(sn.Body) == 0 {
			continue
		}
		head := sn.Body[0]
		if head < 0 || head >= g.NCells {
			continue
		}

		d.buildBlockedForSnake(sim, sc, snIdx)

		neck := neckOf(sn.Body)
		floodThresh := safetyFloodCramped

		var evals [4]dirEval
		for dir := 0; dir < 4; dir++ {
			sc.floodByDir[dir] = -1
			evals[dir] = dirEval{tier: tierDead, flood: -1}

			nb := g.Nbm[head][dir]
			if nb < 0 || nb == neck {
				continue
			}

			// Out of grid
			if !g.IsInGrid(nb) {
				evals[dir] = dirEval{tier: tierDead, flood: 0}
				sc.floodByDir[dir] = 0
				continue
			}

			// Other snake body collision (before gravity)
			if sc.blocked[nb] {
				evals[dir] = dirEval{tier: tierBlocked, flood: 0}
				sc.floodByDir[dir] = 0
				continue
			}

			// Simulate move
			newBody, alive := sim.simulateMove(sn.Body, dir)
			if !alive {
				continue
			}

			beheaded := len(newBody) < sn.Len

			// Beheading kills len-3 snake
			if beheaded && sn.Len <= 3 {
				evals[dir] = dirEval{tier: tierDead, flood: 0}
				sc.floodByDir[dir] = 0
				continue
			}

			// Beheading for longer snakes — big no, avoid if possible
			if beheaded {
				evals[dir] = dirEval{tier: tierBehead, flood: 0}
				sc.floodByDir[dir] = 0
				continue
			}

			bodycp := sc.bodyBuf[:len(newBody)]
			copy(bodycp, newBody)

			if !sim.applyGravity(bodycp) {
				if !isGroundedByOtherSnake(g, bodycp, snIdx) {
					evals[dir] = dirEval{tier: tierDead, flood: 0}
					sc.floodByDir[dir] = 0
					continue
				}
				copy(bodycp, newBody)
			}

			newHead := bodycp[0]

			// Flood fill with own post-move body blocked
			extraStart := sc.blockedN
			for _, c := range bodycp[1:] {
				if c >= 0 && c < g.NCells && !sc.blocked[c] {
					sc.blocked[c] = true
					sc.blockedPrev[sc.blockedN] = c
					sc.blockedN++
				}
			}

			flood := floodCount(g, sc, newHead)

			sc.floodByDir[dir] = flood

			for i := extraStart; i < sc.blockedN; i++ {
				sc.blocked[sc.blockedPrev[i]] = false
			}
			sc.blockedN = extraStart

			// Classify tier
			switch {
			case flood == 0:
				evals[dir] = dirEval{tier: tierDeadEnd, flood: 0}
			case flood < floodThresh:
				evals[dir] = dirEval{tier: tierCramped, flood: flood}
			default:
				evals[dir] = dirEval{tier: tierSafe, flood: flood}
			}
		}

		// Only override if assigned direction is dangerous (tier < tierSafe)
		assigned := d.AssignedDir[si]
		assignedEval := evals[assigned]

		if assignedEval.tier >= tierSafe {
			continue // assigned direction is fine, don't override
		}

		// Find safest alternative
		bestDir := assigned
		bestEval := assignedEval

		for dir := 0; dir < 4; dir++ {
			e := evals[dir]
			if e.tier > bestEval.tier ||
				(e.tier == bestEval.tier && e.flood > bestEval.flood) {
				bestEval = e
				bestDir = dir
			}
		}

		d.AssignedDir[si] = bestDir

		// Tail chase: when all options are cramped or worse, prefer tail direction
		if bestEval.tier <= tierCramped && sn.Len > 1 {
			tail := sn.Body[sn.Len-1]
			tailAdj := -1
			for dir := 0; dir < 4; dir++ {
				if g.Nbm[head][dir] == tail {
					tailAdj = dir
					break
				}
			}
			if tailAdj >= 0 && evals[tailAdj].tier >= tierDeadEnd {
				d.AssignedDir[si] = tailAdj
			} else {
				tailDir := g.DirFromTo(head, tail)
				if tailDir >= 0 && evals[tailDir].tier > bestEval.tier {
					d.AssignedDir[si] = tailDir
				}
			}
		}
	}
}

// phaseSafetyLayer2 checks for enemy head collisions and overrides if dangerous.
// Exceptions: allows head-on when we survive and they die, allows apple contests.
func (d *Decision) phaseSafetyLayer2() {
	g := d.G
	sc := &d.Safety
	sim := NewSim(g)
	sim.RebuildAppleMap()

	if len(d.OpSnakes) == 0 {
		return
	}

	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if sn.Len == 0 || len(sn.Body) == 0 {
			continue
		}

		head := sn.Body[0]
		if head < 0 || head >= g.NCells {
			continue
		}
		assignedDir := d.AssignedDir[si]

		myNextHead := g.Nbm[head][assignedDir]
		if myNextHead < 0 {
			continue
		}

		// Apple contest: if our target cell is an apple, allow collision
		// (deny free eat to enemy)
		if sim.isApple(myNextHead) {
			continue
		}

		dangerous := false
		for _, opIdx := range d.OpSnakes {
			op := &g.Sn[opIdx]
			if !op.Alive || op.Len == 0 {
				continue
			}
			opHead := op.Body[0]
			if opHead < 0 || opHead >= g.NCells || !g.IsInGrid(opHead) {
				continue
			}
			opNeck := neckOf(op.Body)

			// Head-on check: enemy's possible next heads
			for dir := 0; dir < 4; dir++ {
				opNext := g.Nbm[opHead][dir]
				if opNext < 0 || opNext == opNeck {
					continue
				}
				if opNext == myNextHead {
					// Allow if we're >3 and enemy is <=3: they die, we survive
					if sn.Len > 3 && op.Len <= 3 {
						continue
					}
					dangerous = true
					break
				}
			}
			if dangerous {
				break
			}

			// Body collision check (excl. head and movable tail)
			tailIdx := op.Len - 1
			for bi, c := range op.Body {
				if bi == 0 {
					continue
				}
				if bi == tailIdx {
					continue
				}
				if c == myNextHead {
					dangerous = true
					break
				}
			}
			if dangerous {
				break
			}
		}

		if !dangerous {
			continue
		}

		// Find safest alternative
		neck := neckOf(sn.Body)
		bestDir := assignedDir
		bestFlood := sc.floodByDir[assignedDir]

		for dir := 0; dir < 4; dir++ {
			if dir == assignedDir {
				continue
			}
			if sc.floodByDir[dir] <= 0 {
				continue
			}
			nb := g.Nbm[head][dir]
			if nb < 0 || nb == neck {
				continue
			}

			// Check this alternative is safe from enemy collision
			altDanger := false
			for _, opIdx := range d.OpSnakes {
				op := &g.Sn[opIdx]
				if !op.Alive || op.Len == 0 {
					continue
				}
				opHead := op.Body[0]
				if opHead < 0 || opHead >= g.NCells || !g.IsInGrid(opHead) {
					continue
				}
				opNeck := neckOf(op.Body)
				for od := 0; od < 4; od++ {
					opNext := g.Nbm[opHead][od]
					if opNext < 0 || opNext == opNeck {
						continue
					}
					if opNext == nb {
						// Same exception: allow if we survive
						if sn.Len > 3 && op.Len <= 3 {
							continue
						}
						altDanger = true
						break
					}
				}
				if altDanger {
					break
				}
			}

			flood := sc.floodByDir[dir]
			if !altDanger && flood > bestFlood {
				bestFlood = flood
				bestDir = dir
			}
		}

		if bestDir != assignedDir {
			d.AssignedDir[si] = bestDir
		}
	}
}
