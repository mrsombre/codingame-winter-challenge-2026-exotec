package main

// --- Phase 2: Influence mapping ---

const influenceExclusiveMargin = 3 // turns advantage to classify as "exclusive"

// Apple contestation categories.
const (
	ContestMine   = 0 // only I can reach, or I'm >=margin turns faster
	ContestTheirs = 1 // only opponent, or they're >=margin turns faster
	ContestShared = 2 // both can reach within margin
	ContestNone   = 3 // nobody can reach within BFS horizon
)

// AppleContest holds per-apple contestation data.
type AppleContest struct {
	MyBest  int // dist from closest friendly snake (-1 = unreachable)
	OpBest  int // dist from closest enemy snake (-1 = unreachable)
	MySnake int // MySnakes slot index of closest friendly (-1 = none)
	OpSnake int // OpSnakes slot index of closest enemy (-1 = none)
	Cat     int // ContestMine/Theirs/Shared/None
}

func (d *Decision) phaseInfluence() {
	g := d.G

	// Reset cell lookup.
	for i := 0; i < g.NCells && i < MaxExpandedCells; i++ {
		d.ContestByCell[i] = ContestNone
	}

	for a := 0; a < g.ANum; a++ {
		appleCell := g.Ap[a]
		c := &d.Influence[a]
		c.MyBest = -1
		c.OpBest = -1
		c.MySnake = -1
		c.OpSnake = -1

		// Find closest friendly snake that can reach this apple.
		for si := 0; si < len(d.MySnakes); si++ {
			for _, ri := range d.BFS.MyReach[si] {
				if ri.Apple == appleCell {
					if c.MyBest < 0 || ri.Dist < c.MyBest {
						c.MyBest = ri.Dist
						c.MySnake = si
					}
					break // MyReach is sorted by Dist, first match is closest
				}
			}
		}

		// Find closest enemy snake that can reach this apple.
		for oi := 0; oi < len(d.OpSnakes); oi++ {
			for _, ri := range d.BFS.OpReach[oi] {
				if ri.Apple == appleCell {
					if c.OpBest < 0 || ri.Dist < c.OpBest {
						c.OpBest = ri.Dist
						c.OpSnake = oi
					}
					break
				}
			}
		}

		// Classify.
		switch {
		case c.MyBest < 0 && c.OpBest < 0:
			c.Cat = ContestNone
		case c.OpBest < 0:
			c.Cat = ContestMine
		case c.MyBest < 0:
			c.Cat = ContestTheirs
		case c.MyBest+influenceExclusiveMargin <= c.OpBest:
			c.Cat = ContestMine
		case c.OpBest+influenceExclusiveMargin <= c.MyBest:
			c.Cat = ContestTheirs
		default:
			c.Cat = ContestShared
		}

		// Fill cell lookup for scoring phase.
		if appleCell >= 0 && appleCell < MaxExpandedCells {
			d.ContestByCell[appleCell] = c.Cat
		}
	}
}
