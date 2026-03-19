package main

// --- Phase 2: Influence heat map ---
// Per-apple numeric advantage: positive = we're closer, negative = opponent closer.

const (
	heatUnreachable = -9999 // we can't reach this apple
	heatExclusive   = 9999  // we can reach, opponent can't
)

// AppleContest holds per-apple contestation data.
type AppleContest struct {
	MyBest  int // dist from closest friendly snake (-1 = unreachable)
	OpBest  int // dist from closest enemy snake (-1 = unreachable)
	MySnake int // MySnakes slot index of closest friendly (-1 = none)
	OpSnake int // OpSnakes slot index of closest enemy (-1 = none)
	Heat    int // opDist - myDist (positive = our advantage)
}

func (d *Decision) phaseInfluence() {
	g := d.G

	// Reset cell lookup.
	for i := 0; i < g.NCells && i < MaxExpandedCells; i++ {
		d.HeatByCell[i] = heatUnreachable
	}

	// Build per-apple best distance from all snakes' reach lists in one pass.
	// Each reach entry is already sorted by dist, so first hit per apple is best.
	type bestEntry struct {
		dist  int
		snake int
	}
	myBest := make(map[int]bestEntry, g.ANum)
	opBest := make(map[int]bestEntry, g.ANum)

	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive {
			continue
		}
		tbl := &opBest
		if sn.Owner == 0 {
			tbl = &myBest
		}
		for _, ri := range d.BFS.Reach[i] {
			if prev, ok := (*tbl)[ri.Apple]; !ok || ri.Dist < prev.dist {
				(*tbl)[ri.Apple] = bestEntry{dist: ri.Dist, snake: i}
			}
		}
	}

	for a := 0; a < g.ANum; a++ {
		appleCell := g.Ap[a]
		c := &d.Influence[a]
		c.MyBest = -1
		c.OpBest = -1
		c.MySnake = -1
		c.OpSnake = -1

		if e, ok := myBest[appleCell]; ok {
			c.MyBest = e.dist
			c.MySnake = e.snake
		}
		if e, ok := opBest[appleCell]; ok {
			c.OpBest = e.dist
			c.OpSnake = e.snake
		}

		switch {
		case c.MyBest < 0:
			c.Heat = heatUnreachable
		case c.OpBest < 0:
			c.Heat = heatExclusive
		default:
			c.Heat = c.OpBest - c.MyBest
		}

		if appleCell >= 0 && appleCell < MaxExpandedCells {
			d.HeatByCell[appleCell] = c.Heat
		}
	}
}
