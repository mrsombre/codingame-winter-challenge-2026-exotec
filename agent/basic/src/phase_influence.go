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

	for a := 0; a < g.ANum; a++ {
		appleCell := g.Ap[a]
		c := &d.Influence[a]
		c.MyBest = -1
		c.OpBest = -1
		c.MySnake = -1
		c.OpSnake = -1

		// Find closest friendly and enemy snake that can reach this apple.
		for i := 0; i < g.SNum; i++ {
			sn := &g.Sn[i]
			if !sn.Alive {
				continue
			}
			for _, ri := range d.BFS.Reach[i] {
				if ri.Apple != appleCell {
					continue
				}
				if sn.Owner == 0 {
					if c.MyBest < 0 || ri.Dist < c.MyBest {
						c.MyBest = ri.Dist
						c.MySnake = i
					}
				} else {
					if c.OpBest < 0 || ri.Dist < c.OpBest {
						c.OpBest = ri.Dist
						c.OpSnake = i
					}
				}
				break
			}
		}

		// Compute heat: opDist - myDist
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
