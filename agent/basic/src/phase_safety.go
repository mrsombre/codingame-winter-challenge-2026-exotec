package main

// --- Phase 5: Safety ---

const (
	safetyFloodMinAbs       = 4 // absolute minimum flood count
	safetyFloodBodyMul      = 2 // flood must be >= bodyLen * this
	safetyCorridorMargin    = 2 // cramped if flood < bodyLen + this
	safetyCorridorEscapeMul = 3 // escape dir needs flood >= bodyLen * this
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

func (d *Decision) phaseSafety() {
	d.ensureSafety()
	sim := NewSim(d.G)
	sim.RebuildAppleMap()

	d.phaseSafetyLayer1(sim)
	d.phaseSafetyLayer2()
}

// phaseSafetyLayer1 overrides AssignedDir when flood fill detects cramped space.
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
		bodyLen := sn.Len

		// Flood threshold
		floodThresh := bodyLen * safetyFloodBodyMul
		if floodThresh < safetyFloodMinAbs {
			floodThresh = safetyFloodMinAbs
		}

		bestFlood := -1
		bestFloodDir := -1
		assignedFlood := -1

		for dir := 0; dir < 4; dir++ {
			sc.floodByDir[dir] = -1

			nb := g.Nbm[head][dir]
			if nb < 0 || nb == neck {
				continue
			}

			// Simulate move + gravity
			newBody, alive := sim.simulateMove(sn.Body, dir)
			if !alive {
				continue
			}

			// Detect beheading: simulateMove returns shorter body on wall/self collision
			beheaded := len(newBody) < sn.Len

			bodycp := sc.bodyBuf[:len(newBody)]
			copy(bodycp, newBody)

			if !sim.applyGravity(bodycp) {
				sc.floodByDir[dir] = 0
				continue
			}

			newHead := bodycp[0]

			// Temporarily block own post-move body[1:] for flood
			extraStart := sc.blockedN
			for _, c := range bodycp[1:] {
				if c >= 0 && c < g.NCells && !sc.blocked[c] {
					sc.blocked[c] = true
					sc.blockedPrev[sc.blockedN] = c
					sc.blockedN++
				}
			}

			flood := floodCount(g, sc, newHead)

			// Beheading moves lose a segment — never prefer them as "best flood"
			if beheaded {
				flood = 0
			}
			sc.floodByDir[dir] = flood

			// Unblock own body
			for i := extraStart; i < sc.blockedN; i++ {
				sc.blocked[sc.blockedPrev[i]] = false
			}
			sc.blockedN = extraStart

			if flood > bestFlood {
				bestFlood = flood
				bestFloodDir = dir
			}
			if dir == d.AssignedDir[si] {
				assignedFlood = flood
			}
		}

		if bestFloodDir < 0 {
			continue // no valid direction at all
		}

		// Override if assigned direction leads to cramped space
		if assignedFlood >= 0 && assignedFlood < floodThresh && bestFlood > assignedFlood {
			d.AssignedDir[si] = bestFloodDir
			assignedFlood = bestFlood
		}

		// Cramped corridor: if very tight and a much better option exists
		corridorThresh := bodyLen + safetyCorridorMargin
		escapeThresh := bodyLen * safetyCorridorEscapeMul
		if assignedFlood >= 0 && assignedFlood < corridorThresh && bestFlood >= escapeThresh {
			d.AssignedDir[si] = bestFloodDir
		}
	}
}

// phaseSafetyLayer2 checks for enemy head collisions and overrides if dangerous.
func (d *Decision) phaseSafetyLayer2() {
	g := d.G
	sc := &d.Safety

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

		// Our planned next head
		myNextHead := g.Nbm[head][assignedDir]
		if myNextHead < 0 {
			continue
		}

		// Check if any enemy can collide with our next head
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

			// Check head-on: enemy's possible next heads
			for dir := 0; dir < 4; dir++ {
				opNext := g.Nbm[opHead][dir]
				if opNext < 0 || opNext == opNeck {
					continue
				}
				if opNext == myNextHead {
					dangerous = true
					break
				}
			}
			if dangerous {
				break
			}

			// Check if our next head lands in enemy body (excl. movable tail)
			tailIdx := op.Len - 1
			for bi, c := range op.Body {
				if bi == 0 {
					continue // enemy head will move away
				}
				if bi == tailIdx {
					continue // tail will vacate (approximate)
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

		// Find safest alternative: highest flood that avoids collision
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

			// Check this alternative is also safe from enemy collision
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
						altDanger = true
						break
					}
				}
				if altDanger {
					break
				}
			}

			// Prefer non-dangerous with high flood
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
