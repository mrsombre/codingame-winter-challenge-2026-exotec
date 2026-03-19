package main

const (
	mpcBeamWidth = 20 // wider beam to handle more candidates
)

// CandType classifies a candidate.
const (
	candRoute      = 0 // continue turn-1 route
	candSafe       = 1 // nearest apple with no enemy competition
	candDeny       = 2 // contested apple we can win
	candUnclaimed  = 3 // nearest apple nobody targets
	candReposition = 4 // move toward surface with apple access
)

// Candidate represents one possible move for a bot.
type Candidate struct {
	Apple    int // target apple cell (-1 = reposition)
	Dir      int // first direction to move
	ETA      int // turns to reach apple
	CandType int
	Depth2   int // follow-up score: how many apples reachable after eating this one
}

// beamEntry represents one partial joint plan in the beam.
type beamEntry struct {
	dirs   [MaxPSn]int // chosen dir per bot slot
	apples [MaxPSn]int // chosen apple per bot slot
	score  int
}

// --- Apple classification ---

const (
	appleClassSafe      = 0 // no enemy predicted near it
	appleClassContested = 1 // enemy ETA within ±2 of ours
	appleClassEnemy     = 2 // enemy significantly closer
)

// classifyApple returns how contested an apple is for a given bot distance.
func (d *Decision) classifyApple(apple, myDist int) int {
	p := d.P
	for _, snIdx := range d.OpSnakes {
		b := &p.Beliefs[snIdx]
		if b.Target != apple || b.Confidence == 0 {
			continue
		}
		if b.ETA <= myDist-2 {
			return appleClassEnemy
		}
		if b.ETA <= myDist+2 {
			return appleClassContested
		}
	}
	return appleClassSafe
}

// --- Candidate generation ---

// generateCandidates produces candidates for one bot from ALL reachable apples.
// Route continuation is injected as a candidate; SimBFS provides the rest.
func (d *Decision) generateCandidates(si int) []Candidate {
	g := d.G
	p := d.P
	snIdx := d.MySnakes[si]
	sn := &g.Sn[snIdx]

	if !sn.Alive || sn.Len == 0 {
		return nil
	}

	var cands []Candidate
	usedApple := make(map[int]bool)

	// Candidate 1: continue turn-1 route (if valid).
	route := &p.Routes[si]
	if route.Valid && route.StepCursor < len(route.Steps) {
		step := route.Steps[route.StepCursor]
		apple := nextRouteApple(route)
		cands = append(cands, Candidate{
			Apple:    apple,
			Dir:      step.Dir,
			ETA:      stepsToNextApple(route),
			CandType: candRoute,
		})
		if apple >= 0 {
			usedApple[apple] = true
		}
	}

	// All SimBFS targets as candidates — use the full budget.
	sim := NewSim(g)
	sim.RebuildAppleMap()
	targets := sim.SimBFSApples(sn)
	sortTargets(targets)

	for _, t := range targets {
		if usedApple[t.Apple] {
			continue
		}
		usedApple[t.Apple] = true

		cls := d.classifyApple(t.Apple, t.Dist)
		candType := candSafe
		if cls == appleClassContested {
			candType = candDeny
		} else if cls == appleClassEnemy {
			candType = candSafe // still a candidate, just penalized in scoring
		}
		// Check if friendly route claims it.
		if t.Apple >= 0 && t.Apple < len(p.PlannedApples) && p.PlannedApples[t.Apple] {
			candType = candUnclaimed // lower priority but still viable
		}

		cands = append(cands, Candidate{
			Apple: t.Apple, Dir: t.FirstDir,
			ETA: t.Dist, CandType: candType,
		})
	}

	// Fallback: reposition if no apple candidates.
	if len(cands) == 0 {
		bestDir := fallbackDir(g, sn)
		cands = append(cands, Candidate{
			Apple: -1, Dir: bestDir,
			ETA: 0, CandType: candReposition,
		})
	}

	// Depth-2: approximate follow-up potential using manhattan distance to other apples.
	// Cheap O(candidates * apples) — no body sim needed.
	for ci := range cands {
		c := &cands[ci]
		if c.Apple < 0 {
			continue
		}
		ax, ay := g.XY(c.Apple)
		d2 := 0
		for i := 0; i < g.ANum; i++ {
			other := g.Ap[i]
			if other == c.Apple {
				continue
			}
			ox, oy := g.XY(other)
			md := abs(ax-ox) + abs(ay-oy)
			if md <= 4 {
				d2 += 6
			} else if md <= 8 {
				d2 += 2
			}
		}
		c.Depth2 = d2
	}

	return cands
}

// stepsToNextApple counts steps until the next apple in a route.
func stepsToNextApple(route *BotRoute) int {
	count := 0
	for k := route.StepCursor; k < len(route.Steps); k++ {
		count++
		if route.Steps[k].Apple >= 0 {
			return count
		}
	}
	return count
}

// sortTargets sorts SimTarget slice by Dist ascending (insertion sort).
func sortTargets(t []SimTarget) {
	for i := 1; i < len(t); i++ {
		for j := i; j > 0 && t[j].Dist < t[j-1].Dist; j-- {
			t[j], t[j-1] = t[j-1], t[j]
		}
	}
}

// --- Scoring ---

// scoreCandidate scores a single candidate in the context of a joint plan.
func (d *Decision) scoreCandidate(c *Candidate, friendlyApples map[int]int, si int) int {
	score := 0

	switch c.CandType {
	case candRoute:
		score += 10
	case candSafe:
		score += 5
	case candDeny:
		score += 25
	case candUnclaimed:
		score += 5
	case candReposition:
		score -= 5
	}

	if c.Apple >= 0 {
		if c.ETA <= 1 {
			score += 100
		} else {
			score -= c.ETA * 12
		}

		cls := d.classifyApple(c.Apple, c.ETA)
		if cls == appleClassEnemy {
			score -= 20
		}

		if prev, ok := friendlyApples[c.Apple]; ok && prev != si {
			score -= 30
		}

		// Depth-2 follow-up potential.
		score += c.Depth2
	} else {
		score -= 12
	}

	return score
}

// --- Beam search ---

// beamSearch finds the best joint plan for all bots.
func (d *Decision) beamSearch(allCands [][]Candidate) []Candidate {
	myN := len(d.MySnakes)
	if myN == 0 {
		return nil
	}

	// Initialize beam with bot 0's candidates.
	if len(allCands) == 0 || len(allCands[0]) == 0 {
		var e beamEntry
		for i := range e.dirs {
			e.dirs[i] = DU
			e.apples[i] = -1
		}
		return []Candidate{{Dir: DU, Apple: -1}}
	}

	var beam []beamEntry
	for _, c := range allCands[0] {
		var e beamEntry
		for i := range e.apples {
			e.apples[i] = -1
		}
		e.dirs[0] = c.Dir
		e.apples[0] = c.Apple
		friendlyApples := map[int]int{}
		if c.Apple >= 0 {
			friendlyApples[c.Apple] = 0
		}
		e.score = d.scoreCandidate(&c, friendlyApples, 0)
		beam = append(beam, e)
	}

	// Extend beam for bots 1..N-1.
	for si := 1; si < myN; si++ {
		if si >= len(allCands) || len(allCands[si]) == 0 {
			for bi := range beam {
				beam[bi].dirs[si] = DU
			}
			continue
		}

		var nextBeam []beamEntry
		for _, entry := range beam {
			friendlyApples := make(map[int]int)
			for prev := 0; prev < si; prev++ {
				if entry.apples[prev] >= 0 {
					friendlyApples[entry.apples[prev]] = prev
				}
			}

			for _, c := range allCands[si] {
				var ne beamEntry
				ne = entry
				ne.dirs[si] = c.Dir
				ne.apples[si] = c.Apple
				ne.score = entry.score + d.scoreCandidate(&c, friendlyApples, si)
				nextBeam = append(nextBeam, ne)
			}
		}

		// Prune to beam width: partial selection sort for top entries.
		if len(nextBeam) > mpcBeamWidth {
			for i := 0; i < mpcBeamWidth; i++ {
				for j := i + 1; j < len(nextBeam); j++ {
					if nextBeam[j].score > nextBeam[i].score {
						nextBeam[i], nextBeam[j] = nextBeam[j], nextBeam[i]
					}
				}
			}
			nextBeam = nextBeam[:mpcBeamWidth]
		}
		beam = nextBeam
	}

	// Pick best.
	bestIdx := 0
	for i := 1; i < len(beam); i++ {
		if beam[i].score > beam[bestIdx].score {
			bestIdx = i
		}
	}

	best := beam[bestIdx]
	result := make([]Candidate, myN)
	for si := 0; si < myN; si++ {
		result[si] = Candidate{
			Dir:   best.dirs[si],
			Apple: best.apples[si],
		}
	}
	return result
}

// --- Main MPC phase ---

// phaseMPC is the main MPC dispatcher. Replaces phasePartition.
func (d *Decision) phaseMPC() {
	p := d.P

	// Turn 1: run initial partition as seed.
	if !p.Initialized {
		d.planAllRoutes()
		p.Initialized = true
	} else {
		d.advanceRoutes()
	}
	d.rebuildPlannedApples()

	// Update enemy predictions.
	d.updateEnemyBeliefs()

	// Generate candidates for each bot.
	myN := len(d.MySnakes)
	allCands := make([][]Candidate, myN)
	for si := range d.MySnakes {
		allCands[si] = d.generateCandidates(si)
	}

	// Beam search for best joint plan.
	plan := d.beamSearch(allCands)

	// Emit first moves.
	for si, snIdx := range d.MySnakes {
		sn := &d.G.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}
		if si < len(plan) {
			d.AssignedDir[si] = plan[si].Dir
			d.Assigned[si] = plan[si].Apple
		}
	}
}

// advanceRoutes checks head position against route expectation.
// If head matches, advance cursor. If not, invalidate.
func (d *Decision) advanceRoutes() {
	g := d.G
	p := d.P

	appleAlive := make([]bool, g.NCells)
	for i := 0; i < g.ANum; i++ {
		if g.Ap[i] >= 0 && g.Ap[i] < g.NCells {
			appleAlive[g.Ap[i]] = true
		}
	}

	for si, snIdx := range d.MySnakes {
		route := &p.Routes[si]
		if !route.Valid {
			continue
		}

		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			route.Valid = false
			continue
		}

		// Remove eaten apples.
		var alive []int
		for _, ap := range route.AppleSeq {
			if ap >= 0 && ap < g.NCells && appleAlive[ap] {
				alive = append(alive, ap)
			}
		}
		route.AppleSeq = alive
		if len(route.AppleSeq) == 0 {
			route.Valid = false
			continue
		}

		// Check position matches and advance cursor.
		if route.StepCursor < len(route.Steps) {
			step := &route.Steps[route.StepCursor]
			if sn.Body[0] == step.ExpHead {
				route.StepCursor++ // consumed this step
			} else {
				route.Valid = false // position drifted
			}
		} else {
			route.Valid = false // steps exhausted
		}
	}
}
