package main

const (
	greedyDFSDepth       = 4
	greedyEatScore       = 1000
	greedyReachBonusBase = 200
	greedyMobilityScore  = 12
	greedyNoopPenalty    = 150

	// Contestation bonuses applied in greedyHeuristic.
	// Contestation bonuses: kept small to only affect tie-breaking (dist*20 gap = 20 per hop).
	heuristicContestMineBonus = 15 // prefer exclusive apples at same distance
	heuristicContestSharedPen = 0  // no penalty for contested
	heuristicContestTheirsPen = -5 // slight depriority for exclusive-theirs
)

type greedyEval struct {
	score int
	apple int
}

func (d *Decision) phaseScoring() {
	g := d.G
	sim := NewSim(g)

	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		startSurfs := occupiedSurfaces(g, sn.Body)
		bestDir := d.AssignedDir[si]
		bestScore := -1 << 30
		bestApple := d.Assigned[si]

		for dir := 0; dir < 4; dir++ {
			eval, ok := d.greedyEvaluateMove(sim, sn, startSurfs, dir, greedyDFSDepth)
			if !ok {
				continue
			}
			if eval.score > bestScore {
				bestScore = eval.score
				bestDir = dir
				bestApple = eval.apple
			}
		}

		d.AssignedDir[si] = bestDir
		d.Assigned[si] = bestApple
	}
}

func (d *Decision) greedyEvaluateMove(sim *Sim, sn *Snake, startSurfs map[int]bool, dir, depth int) (greedyEval, bool) {
	body, apples, ateApple, ok := simulateSingleMove(sim, sn.Body, dir, d.G.Ap[:d.G.ANum])
	if !ok {
		return greedyEval{}, false
	}

	eval := d.greedyDFS(sim, body, apples, startSurfs, depth-1)
	if sameBody(sn.Body, body) {
		eval.score -= greedyNoopPenalty
	}
	if ateApple >= 0 {
		eval.score += greedyEatScore
		eval.apple = ateApple
	}
	return eval, true
}

func (d *Decision) greedyDFS(sim *Sim, body []int, apples []int, startSurfs map[int]bool, depth int) greedyEval {
	best := d.greedyHeuristic(body, apples)
	if depth <= 0 || !touchesAnySurface(d.G, body, startSurfs) {
		return best
	}

	head := body[0]
	neck := neckOf(body)

	for dir := 0; dir < 4; dir++ {
		nb := d.G.Nbm[head][dir]
		if nb < 0 || nb == neck {
			continue
		}

		nextBody, nextApples, ateApple, ok := simulateSingleMove(sim, body, dir, apples)
		if !ok {
			continue
		}

		child := d.greedyDFS(sim, nextBody, nextApples, startSurfs, depth-1)
		score := child.score
		apple := child.apple
		if sameBody(body, nextBody) {
			score -= greedyNoopPenalty
		}
		if ateApple >= 0 {
			score += greedyEatScore
			apple = ateApple
		}

		if score > best.score {
			best = greedyEval{score: score, apple: apple}
		}
	}

	return best
}

func (d *Decision) greedyHeuristic(body []int, apples []int) greedyEval {
	score := mobilityCount(d.G, body) * greedyMobilityScore
	apple := -1

	sn := &Snake{
		ID:    -1,
		Owner: 0,
		Body:  body,
		Len:   len(body),
		Alive: len(body) > 0,
	}

	reach := withApples(d.G, apples, func() []ReachInfo {
		return surfaceReach(d.G, sn, true)
	})

	// Pick best apple considering both distance and contestation.
	bestAppleScore := -1 << 30
	for _, ri := range reach {
		s := greedyReachBonusBase - ri.Dist*20
		if ri.Apple >= 0 && ri.Apple < MaxExpandedCells {
			switch d.ContestByCell[ri.Apple] {
			case ContestMine:
				s += heuristicContestMineBonus
			case ContestShared:
				s += heuristicContestSharedPen
			case ContestTheirs:
				s += heuristicContestTheirsPen
			}
		}
		if s > bestAppleScore {
			bestAppleScore = s
			apple = ri.Apple
		}
	}
	if bestAppleScore > -1<<30 {
		score += bestAppleScore
	} else if len(reach) > 0 {
		score += greedyReachBonusBase - reach[0].Dist*20
		apple = reach[0].Apple
	}

	return greedyEval{score: score, apple: apple}
}

func occupiedSurfaces(g *Game, body []int) map[int]bool {
	out := make(map[int]bool)
	for _, cell := range body {
		if !g.IsInGrid(cell) {
			continue
		}
		sid := g.SurfAt[cell]
		if sid < 0 || g.Surfs[sid].Type == SurfNone {
			continue
		}
		out[sid] = true
	}
	return out
}

func touchesAnySurface(g *Game, body []int, surfaces map[int]bool) bool {
	if len(surfaces) == 0 {
		return false
	}
	for _, cell := range body {
		if !g.IsInGrid(cell) {
			continue
		}
		sid := g.SurfAt[cell]
		if sid >= 0 && surfaces[sid] {
			return true
		}
	}
	return false
}

func sameBody(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func mobilityCount(g *Game, body []int) int {
	if len(body) == 0 {
		return 0
	}
	head := body[0]
	neck := neckOf(body)
	n := 0
	for dir := 0; dir < 4; dir++ {
		nb := g.Nbm[head][dir]
		if nb >= 0 && nb != neck {
			n++
		}
	}
	return n
}

func simulateSingleMove(sim *Sim, body []int, dir int, apples []int) ([]int, []int, int, bool) {
	g := sim.G
	if len(body) == 0 {
		return nil, nil, -1, false
	}

	sim.rebuildAppleMapFrom(apples)

	head := body[0]
	neck := neckOf(body)
	nc := g.Nbm[head][dir]
	if nc < 0 || nc == neck {
		return nil, nil, -1, false
	}

	newBody, alive := sim.simulateMove(body, dir)
	if !alive {
		return nil, nil, -1, false
	}

	bodycp := append([]int(nil), newBody...)
	ateApple := -1
	if sim.isApple(nc) {
		ateApple = nc
		apples = removeApple(apples, nc)
		sim.rebuildAppleMapFrom(apples)
	}

	if !sim.applyGravity(bodycp) {
		return nil, nil, -1, false
	}

	nextApples := append([]int(nil), apples...)
	return bodycp, nextApples, ateApple, true
}

func removeApple(apples []int, target int) []int {
	out := make([]int, 0, len(apples))
	removed := false
	for _, apple := range apples {
		if !removed && apple == target {
			removed = true
			continue
		}
		out = append(out, apple)
	}
	return out
}

func withApples[T any](g *Game, apples []int, fn func() T) T {
	prevAp := g.Ap
	prevANum := g.ANum
	g.Ap = apples
	g.ANum = len(apples)
	defer func() {
		g.Ap = prevAp
		g.ANum = prevANum
	}()
	return fn()
}
