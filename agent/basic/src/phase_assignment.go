package main

// --- Phase 4: Assignment ---

// simDist returns SimBFS distance from snake si to apple cell, or MaxCells if unreachable.
func (d *Decision) simDist(si int, apple int) int {
	for _, st := range d.SimTargets[si] {
		if st.Apple == apple {
			return st.Dist
		}
	}
	return MaxCells
}

func (d *Decision) phaseAssignment() {
	g := d.G

	n := len(d.MySnakes)
	d.Assigned = make([]int, n)
	d.AssignedDir = make([]int, n)
	for i := range d.Assigned {
		d.Assigned[i] = -1
		d.AssignedDir[i] = DU // fallback
	}

	if n == 0 {
		return
	}

	if g.ANum > 0 {
		// Step 1: each snake picks its best scored apple, but skip apples
		// where a teammate has a strictly shorter SimBFS distance.
		pick := make([]int, n)
		for si := 0; si < n; si++ {
			pick[si] = -1
			bestScore := -1
			for j := 0; j < g.ANum; j++ {
				if d.Scores[si][j] <= 0 {
					continue
				}
				// Check if any teammate is strictly closer to this apple.
				ap := g.Ap[j]
				myDist := d.simDist(si, ap)
				stolen := false
				for other := 0; other < n; other++ {
					if other == si {
						continue
					}
					if d.simDist(other, ap) < myDist {
						stolen = true
						break
					}
				}
				if stolen {
					continue
				}
				if d.Scores[si][j] > bestScore {
					bestScore = d.Scores[si][j]
					pick[si] = j
				}
			}
		}

		// Step 2: resolve remaining conflicts (two snakes same distance).
		// Greedy: higher score wins, loser re-picks from unclaimed.
		claimed := make([]bool, g.ANum)
		for iter := 0; iter < n*2; iter++ {
			conflict := false
			// Clear and rebuild claims.
			for j := range claimed {
				claimed[j] = false
			}
			owner := make([]int, g.ANum)
			for j := range owner {
				owner[j] = -1
			}
			for si := 0; si < n; si++ {
				j := pick[si]
				if j < 0 {
					continue
				}
				prev := owner[j]
				if prev == -1 {
					owner[j] = si
					claimed[j] = true
					continue
				}
				// Conflict: higher score wins.
				conflict = true
				loser := si
				if d.Scores[si][j] > d.Scores[prev][j] {
					loser = prev
					owner[j] = si
				}
				// Loser re-picks.
				pick[loser] = -1
				bestScore := -1
				for k := 0; k < g.ANum; k++ {
					if claimed[k] {
						continue
					}
					if d.Scores[loser][k] > bestScore {
						bestScore = d.Scores[loser][k]
						pick[loser] = k
					}
				}
			}
			if !conflict {
				break
			}
		}

		// Step 3: commit apple assignments using SimBFS direction.
		for si := 0; si < n; si++ {
			j := pick[si]
			if j < 0 {
				continue
			}
			ap := g.Ap[j]
			d.Assigned[si] = ap
			for _, st := range d.SimTargets[si] {
				if st.Apple == ap {
					d.AssignedDir[si] = st.FirstDir
					break
				}
			}
		}
	}

	// Step 4: fallback for unassigned snakes — target small opponents (len 3-4).
	for si := 0; si < n; si++ {
		if d.Assigned[si] != -1 {
			continue
		}
		snIdx := d.MySnakes[si]
		sn := &g.Sn[snIdx]
		head := sn.Body[0]
		if !g.IsInGrid(head) {
			continue
		}
		myLen := len(sn.Body)
		bfs := d.BFS[si]
		if bfs == nil {
			continue
		}

		bestDist := MaxCells
		bestDir := -1
		// Look for small enemies we can kill (head-on collision kills shorter snake).
		for _, opIdx := range d.OpSnakes {
			op := &g.Sn[opIdx]
			if !op.Alive || len(op.Body) == 0 {
				continue
			}
			opLen := len(op.Body)
			if opLen > 4 || opLen >= myLen {
				continue // only target smaller enemies (len 3-4)
			}
			opHead := op.Body[0]
			if !g.IsInGrid(opHead) {
				continue
			}
			r := bfs[opHead]
			if r.Dist >= 0 && r.Dist < bestDist {
				bestDist = r.Dist
				bestDir = r.FirstDir
			}
		}
		if bestDir >= 0 {
			d.AssignedDir[si] = bestDir
			continue
		}

		// Final fallback: move toward nearest higher surface.
		_, hy := g.XY(head)
		for _, surf := range d.P.Surfs {
			if surf.Y >= hy {
				continue
			}
			for _, x := range []int{surf.Left, surf.Right} {
				target := g.Idx(x, surf.Y)
				r := bfs[target]
				if r.Dist >= 0 && r.Dist < bestDist {
					bestDist = r.Dist
					d.AssignedDir[si] = r.FirstDir
				}
			}
		}
	}
}

