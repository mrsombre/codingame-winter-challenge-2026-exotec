package main

// --- Phase 3: Resource scoring ---
//
// For each (my snake, apple) pair compute a composite score (higher = better).
// Components:
//   base       — inverse BFS distance, dominant signal
//   safety     — Voronoi influence bonus/penalty
//   exclusive  — bonus when enemies can't reach the apple (or are far behind)
//   height     — small bonus for elevated apples (lower Y)
//   cluster    — bonus for apples near other uncollected apples
//   race       — penalty when an enemy reaches the apple faster

// Tunable weights — adjust to shift strategy priorities.
const (
	scoreBase      = 1500 // numerator for 1/(1+dist) base score
	scoreInfluence = 50   // per-turn influence advantage
	scoreExclusive = 300  // enemy can't reach apple at all
	scoreAdvantage = 150  // enemy can reach but is far behind (> margin turns)
	scoreMargin    = 3    // turns lead to trigger advantage bonus
	scoreHeight    = 10   // per-row elevation bonus
	scoreClusterR  = 5    // Manhattan radius to count nearby apples
	scoreCluster   = 80   // per nearby apple
	scoreRace      = 150  // per-turn enemy lead penalty
)

func (d *Decision) phaseScoring() {
	g := d.G
	numMy := len(d.MySnakes)
	numAp := g.ANum

	// Allocate score matrix.
	if cap(d.Scores) >= numMy {
		d.Scores = d.Scores[:numMy]
	} else {
		d.Scores = make([][]int, numMy)
	}
	for si := 0; si < numMy; si++ {
		if cap(d.Scores[si]) >= numAp {
			d.Scores[si] = d.Scores[si][:numAp]
		} else {
			d.Scores[si] = make([]int, numAp)
		}
	}

	if numAp == 0 {
		return
	}

	// --- Per-apple precomputation ---

	// Min enemy BFS distance to each apple.
	opMinDist := make([]int, numAp)
	for j := 0; j < numAp; j++ {
		ap := g.Ap[j]
		best := MaxCells
		for _, bfs := range d.OpBFS {
			if bfs != nil && bfs[ap].Dist >= 0 && bfs[ap].Dist < best {
				best = bfs[ap].Dist
			}
		}
		opMinDist[j] = best
	}

	// Clustering: count other apples within Manhattan radius.
	clusterCount := make([]int, numAp)
	for j := 0; j < numAp; j++ {
		for k := j + 1; k < numAp; k++ {
			if g.Manhattan(g.Ap[j], g.Ap[k]) <= scoreClusterR {
				clusterCount[j]++
				clusterCount[k]++
			}
		}
	}

	// --- Score each (snake, apple) pair ---

	for si := 0; si < numMy; si++ {
		bfs := d.BFS[si]
		if bfs == nil {
			for j := 0; j < numAp; j++ {
				d.Scores[si][j] = -1
			}
			continue
		}

		for j := 0; j < numAp; j++ {
			ap := g.Ap[j]
			r := bfs[ap]
			if r.Dist < 0 {
				d.Scores[si][j] = -1
				continue
			}

			dist := r.Dist

			// Base: inverse distance (closer = higher).
			base := scoreBase / (1 + dist)

			// Safety: influence map bonus/penalty.
			safety := d.Influence[ap] * scoreInfluence

			// Exclusivity: bonus if enemy can't compete.
			excl := 0
			opd := opMinDist[j]
			if opd >= MaxCells {
				excl = scoreExclusive
			} else if opd > dist+scoreMargin {
				excl = scoreAdvantage
			}

			// Height: elevated apples slightly preferred.
			_, ay := g.XY(ap)
			height := (g.H - 1 - ay) * scoreHeight

			// Clustering: nearby apples reward efficient collection.
			cluster := clusterCount[j] * scoreCluster

			// Race penalty: enemy reaches apple faster.
			race := 0
			if opd < MaxCells && opd < dist {
				race = (dist - opd) * scoreRace
			}

			s := base + safety + excl + height + cluster - race
			if s < 1 {
				s = 1 // reachable apples always score positive
			}
			d.Scores[si][j] = s
		}
	}
}
