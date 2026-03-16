package main

// --- Phase 5: Safety check ---

func (d *Decision) phaseSafety() {
	g := d.G
	n := g.W * g.H
	numMy := len(d.MySnakes)

	// Build body-cell bitmap (all alive snake bodies).
	bodyCell := make([]bool, n)
	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive {
			continue
		}
		for _, c := range sn.Body {
			if c >= 0 && c < n {
				bodyCell[c] = true
			}
		}
	}

	// --- Phase A: per-snake safety scoring ---

	// safeScore[si][dir] = -1 (lethal) or reachable cell count.
	safeScore := make([][4]int, numMy)

	for si := 0; si < numMy; si++ {
		snIdx := d.MySnakes[si]
		sn := &g.Sn[snIdx]
		head := sn.Body[0]
		neck := neckOf(sn.Body)
		ownTail := sn.Body[len(sn.Body)-1]

		// OOB head: emergency — prefer directions back inside the map.
		if head >= n {
			if head < g.NCells {
				bestDir := -1
				bestPriority := -1
				for dir := 0; dir < 4; dir++ {
					nc := g.Nb[head][dir]
					if nc == -1 || nc == neck {
						continue
					}
					if nc >= 0 && nc < n && bodyCell[nc] && nc != ownTail {
						continue
					}
					priority := 0
					if nc >= 0 && nc < n {
						priority = 2 // back inside map
					} else {
						priority = 1 // still OOB but alive
					}
					if priority > bestPriority {
						bestPriority = priority
						bestDir = dir
					}
				}
				if bestDir >= 0 {
					d.AssignedDir[si] = bestDir
				}
			}
			for dir := 0; dir < 4; dir++ {
				safeScore[si][dir] = -1
			}
			continue
		}

		if head < 0 {
			for dir := 0; dir < 4; dir++ {
				safeScore[si][dir] = -1
			}
			continue
		}

		bfs := d.BFS[si]

		// Count reachable cells per first-direction (flood fill proxy).
		var reach [4]int
		if bfs != nil {
			for c := 0; c < n; c++ {
				r := bfs[c]
				if r.Dist > 0 && r.FirstDir >= 0 && r.FirstDir < 4 {
					reach[r.FirstDir]++
				}
			}
		}

		for dir := 0; dir < 4; dir++ {
			safeScore[si][dir] = -1
			nc := g.Nb[head][dir]
			if nc == -1 || nc == neck {
				continue
			}
			// Body collision (own tail retracts → safe to enter).
			if nc >= 0 && nc < n && bodyCell[nc] && nc != ownTail {
				continue
			}
			safeScore[si][dir] = reach[dir]
		}

		// Override assigned direction if lethal or trap.
		bestDir := -1
		bestScore := -1
		for dir := 0; dir < 4; dir++ {
			if safeScore[si][dir] > bestScore {
				bestScore = safeScore[si][dir]
				bestDir = dir
			}
		}

		if bestDir == -1 {
			continue
		}

		assigned := d.AssignedDir[si]
		bodyLen := len(sn.Body)

		switch {
		case safeScore[si][assigned] < 0:
			d.AssignedDir[si] = bestDir
		case safeScore[si][assigned] < bodyLen && bestScore >= bodyLen:
			d.AssignedDir[si] = bestDir
		}
	}

	// --- Phase B: deconflict — no two of my snakes target the same cell ---

	for iter := 0; iter < numMy; iter++ {
		// Build target map: next-cell → first snake claiming it.
		targets := make(map[int]int) // nc → si
		conflictSI := -1

		for si := 0; si < numMy; si++ {
			snIdx := d.MySnakes[si]
			sn := &g.Sn[snIdx]
			head := sn.Body[0]
			if head < 0 || head >= n {
				continue
			}
			nc := g.Nb[head][d.AssignedDir[si]]
			if nc < 0 {
				continue
			}

			if _, ok := targets[nc]; ok {
				conflictSI = si // later snake loses the conflict
				break
			}
			targets[nc] = si
		}

		if conflictSI < 0 {
			break // no conflicts
		}

		// Redirect conflictSI to its best non-conflicting safe direction.
		si := conflictSI
		snIdx := d.MySnakes[si]
		head := g.Sn[snIdx].Body[0]
		bestAlt := -1
		bestAltScore := -1
		for dir := 0; dir < 4; dir++ {
			if dir == d.AssignedDir[si] {
				continue
			}
			if safeScore[si][dir] < 0 {
				continue
			}
			nc := g.Nb[head][dir]
			if nc < 0 {
				continue
			}
			if _, ok := targets[nc]; ok {
				continue // another snake already targets this cell
			}
			if safeScore[si][dir] > bestAltScore {
				bestAltScore = safeScore[si][dir]
				bestAlt = dir
			}
		}
		if bestAlt >= 0 {
			d.AssignedDir[si] = bestAlt
		} else {
			break // can't resolve without cascading — stop
		}
	}
}
