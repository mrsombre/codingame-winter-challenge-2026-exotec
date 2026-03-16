package main

// --- Phase 2: Influence mapping ---

func (d *Decision) phaseInfluence() {
	g := d.G
	n := g.W * g.H

	if len(d.Influence) < n {
		d.Influence = make([]int, n)
	}

	for c := 0; c < n; c++ {
		myBest := MaxCells
		for _, bfs := range d.BFS {
			if bfs != nil && bfs[c].Dist >= 0 && bfs[c].Dist < myBest {
				myBest = bfs[c].Dist
			}
		}
		opBest := MaxCells
		for _, bfs := range d.OpBFS {
			if bfs != nil && bfs[c].Dist >= 0 && bfs[c].Dist < opBest {
				opBest = bfs[c].Dist
			}
		}
		d.Influence[c] = opBest - myBest
	}
}
