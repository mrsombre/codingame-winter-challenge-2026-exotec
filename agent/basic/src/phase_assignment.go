package main

// --- Phase 4: Assignment ---

func (d *Decision) phaseAssignment() {
	g := d.G

	n := len(d.MySnakes)
	d.Assigned = make([]int, n)
	d.AssignedDir = make([]int, n)
	for i := range d.Assigned {
		d.Assigned[i] = -1
		d.AssignedDir[i] = DU // fallback
	}

	// Greedy global: pick best (snake, apple) pair each round using
	// precomputed scores from Phase 3 (higher = better target).
	claimed := make([]bool, g.ANum)
	for round := 0; round < n; round++ {
		bestSI := -1
		bestJ := -1
		bestScore := -1
		bestDir := -1

		for si := 0; si < n; si++ {
			if d.Assigned[si] != -1 {
				continue
			}
			bfs := d.BFS[si]
			if bfs == nil {
				continue
			}
			for j := 0; j < g.ANum; j++ {
				if claimed[j] {
					continue
				}
				score := d.Scores[si][j]
				if score < 0 {
					continue
				}
				if score > bestScore {
					bestSI = si
					bestJ = j
					bestScore = score
					bestDir = bfs[g.Ap[j]].FirstDir
				}
			}
		}

		if bestSI == -1 {
			break
		}
		d.Assigned[bestSI] = g.Ap[bestJ]
		d.AssignedDir[bestSI] = bestDir
		claimed[bestJ] = true
	}

	// Fallback for unassigned snakes: nearest reachable higher surface.
	for si := 0; si < n; si++ {
		if d.Assigned[si] != -1 {
			continue
		}
		results := d.BFS[si]
		if results == nil {
			continue
		}
		snIdx := d.MySnakes[si]
		head := d.G.Sn[snIdx].Body[0]
		_, hy := g.XY(head)

		bestDist := MaxCells
		for _, surf := range d.P.Surfs {
			if surf.Y >= hy {
				continue // not above head
			}
			// Try left and right edges of the surface as targets
			for _, x := range []int{surf.Left, surf.Right} {
				target := g.Idx(x, surf.Y)
				r := results[target]
				if r.Dist >= 0 && r.Dist < bestDist {
					bestDist = r.Dist
					d.AssignedDir[si] = r.FirstDir
				}
			}
		}
	}
}
