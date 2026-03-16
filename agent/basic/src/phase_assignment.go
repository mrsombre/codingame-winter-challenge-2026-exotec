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

	// Greedy global: pick best (snake, apple) pair each round.
	// Score = BFS distance + penalty for apples the enemy reaches first.
	claimed := make(map[int]bool)
	for round := 0; round < n; round++ {
		bestSI := -1
		bestApple := -1
		bestScore := MaxCells
		bestDir := -1

		for si := 0; si < n; si++ {
			if d.Assigned[si] != -1 {
				continue
			}
			results := d.BFS[si]
			if results == nil {
				continue
			}
			for j := 0; j < g.ANum; j++ {
				ap := g.Ap[j]
				if claimed[ap] {
					continue
				}
				r := results[ap]
				if r.Dist < 0 {
					continue
				}
				score := r.Dist
				if inf := d.Influence[ap]; inf < 0 && r.Dist > 2 {
					score -= inf * 2 // penalize: enemy closer by |inf| turns
				}
				if score < bestScore {
					bestSI = si
					bestApple = ap
					bestScore = score
					bestDir = r.FirstDir
				}
			}
		}

		if bestSI == -1 {
			break
		}
		d.Assigned[bestSI] = bestApple
		d.AssignedDir[bestSI] = bestDir
		claimed[bestApple] = true
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
