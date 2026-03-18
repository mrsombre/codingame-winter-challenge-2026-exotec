package main

// --- Phase 4: Assignment (deconfliction only) ---
// Ensures no two friendly snakes target the same apple.
// Does NOT override AssignedDir — scoring's body-sim direction is authoritative.

func (d *Decision) phaseAssignment() {
	myN := len(d.MySnakes)
	if myN <= 1 {
		return // no conflicts possible
	}

	// Detect duplicate apple assignments.
	for i := 0; i < myN; i++ {
		if d.Assigned[i] < 0 {
			continue
		}
		for j := i + 1; j < myN; j++ {
			if d.Assigned[j] != d.Assigned[i] {
				continue
			}
			// Conflict: two snakes targeting the same apple.
			// The one with shorter BFS distance keeps it; the other gets next-best.
			distI := reachDistFor(d.BFS.MyReach[i], d.Assigned[i])
			distJ := reachDistFor(d.BFS.MyReach[j], d.Assigned[j])

			loser := j
			if distJ < distI {
				loser = i
			}

			// Reassign loser to next-best apple not already taken.
			taken := make(map[int]bool, myN)
			for k := 0; k < myN; k++ {
				if k != loser && d.Assigned[k] >= 0 {
					taken[d.Assigned[k]] = true
				}
			}

			d.Assigned[loser] = -1
			for _, ri := range d.BFS.MyReach[loser] {
				if !taken[ri.Apple] {
					d.Assigned[loser] = ri.Apple
					// Sync direction to match new target
					if ri.FirstDir >= 0 {
						d.AssignedDir[loser] = ri.FirstDir
					}
					break
				}
			}
		}
	}
}

// reachDistFor finds the BFS distance for a specific apple in a reach list.
func reachDistFor(reach []ReachInfo, apple int) int {
	for _, ri := range reach {
		if ri.Apple == apple {
			return ri.Dist
		}
	}
	return 999
}
