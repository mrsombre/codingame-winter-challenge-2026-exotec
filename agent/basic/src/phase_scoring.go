package main

// --- Phase 3: Resource scoring ---

func (d *Decision) phaseScoring() {
	// TODO: for each agent-resource pair compute:
	// score = base_value / (1 + bfs_distance) × reachability_gate
	//       × safety_factor × clustering_bonus - opponent_penalty
	// Height bonus for elevated resources reachable only by longer snakes.
	// Clustering bonus for resources near other uncollected resources.
}
