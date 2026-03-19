package main

import (
	"math/rand"
	"sort"
)

// Focused per-turn planner.
// The core idea is to avoid long-lived route lock-in:
// 1. claim obvious short apples greedily;
// 2. build short candidate plans only for locally contested snakes;
// 3. use a tiny genetic search to pick a coherent set of those plans.

const (
	focusEasyDist       = 3
	focusEasyHeat       = 2
	focusHorizon        = 7
	focusReplayDepth    = 7
	focusMaxCandidates  = 8
	focusMaxSnakes      = 4
	focusPopulation     = 32
	focusGenerations    = 24
	focusTournamentSize = 3
	focusEliteCount     = 6
	focusOverrideMargin = 110
)

type tacticalPlan struct {
	Apple     int
	Dist      int
	FirstDir  int
	EndHead   int
	EndBody   []int
	Score     int
	Contested bool
	Steps     []RouteStep
}

type focusSnake struct {
	si       int
	snIdx    int
	priority int
	plans    []tacticalPlan
}

type focusGene struct {
	choice [MaxPSn]int
	score  int
}

type easyTarget struct {
	si    int
	snIdx int
	apple int
	dir   int
	dist  int
	score int
}

func (d *Decision) phasePartition() {
	d.phasePartitionBase()
	claimedSnake := make([]bool, len(d.MySnakes))
	claimedApple := make(map[int]bool, d.G.ANum)

	for si, snIdx := range d.MySnakes {
		if d.shouldFocusOverride(si, snIdx) {
			continue
		}
		claimedSnake[si] = true
		if d.Assigned[si] >= 0 {
			claimedApple[d.Assigned[si]] = true
		}
	}

	focus := d.buildFocusSnakes(claimedSnake, claimedApple)
	d.runFocusedSearch(focus, claimedSnake, claimedApple)
}

func (d *Decision) assignEasyTargets(claimedSnake []bool, claimedApple map[int]bool) {
	var all []easyTarget

	for si, snIdx := range d.MySnakes {
		reach := d.BFS.Reach[snIdx]
		for _, ri := range reach {
			if !d.isEasyTarget(ri) {
				continue
			}
			score := 2000 - ri.Dist*140
			if contest, ok := d.contestForApple(ri.Apple); ok {
				switch {
				case contest.OpBest < 0:
					score += 300
				default:
					score += contest.Heat * 70
				}
			}
			if d.enemyHeadNear(ri.Apple, focusEasyDist+1) {
				score -= 250
			}
			all = append(all, easyTarget{
				si: si, snIdx: snIdx, apple: ri.Apple, dir: ri.FirstDir, dist: ri.Dist, score: score,
			})
			break
		}
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].score != all[j].score {
			return all[i].score > all[j].score
		}
		if all[i].dist != all[j].dist {
			return all[i].dist < all[j].dist
		}
		return all[i].apple < all[j].apple
	})

	for _, tgt := range all {
		if claimedSnake[tgt.si] || claimedApple[tgt.apple] {
			continue
		}
		claimedSnake[tgt.si] = true
		claimedApple[tgt.apple] = true
		d.Assigned[tgt.si] = tgt.apple
		d.AssignedDir[tgt.si] = tgt.dir
		d.storeGreedyRoute(tgt.si, tgt.snIdx, tgt.apple)
	}
}

func (d *Decision) isEasyTarget(ri ReachInfo) bool {
	if ri.Apple < 0 || ri.Dist <= 0 {
		return false
	}
	if ri.Dist > focusEasyDist {
		return false
	}
	contest, ok := d.contestForApple(ri.Apple)
	if !ok {
		return false
	}
	if contest.OpBest < 0 {
		return true
	}
	return contest.Heat >= focusEasyHeat
}

func (d *Decision) greedyFallback(si, snIdx int) {
	g := d.G
	sn := &g.Sn[snIdx]

	reach := d.BFS.Reach[snIdx]
	if len(reach) == 0 {
		d.Assigned[si] = -1
		d.AssignedDir[si] = fallbackDir(g, sn)
		d.storeStepRoute(si, snIdx, d.AssignedDir[si], -1)
		return
	}

	used := make(map[int]bool, len(d.MySnakes))
	for oi := range d.Assigned {
		if oi != si && d.Assigned[oi] >= 0 {
			used[d.Assigned[oi]] = true
		}
	}

	chosen := reach[0]
	for _, ri := range reach {
		if !used[ri.Apple] {
			chosen = ri
			break
		}
	}

	d.Assigned[si] = chosen.Apple
	d.AssignedDir[si] = chosen.FirstDir
	d.storeGreedyRoute(si, snIdx, chosen.Apple)
}

func (d *Decision) buildFocusSnakes(claimedSnake []bool, claimedApple map[int]bool) []focusSnake {
	var focus []focusSnake

	for si, snIdx := range d.MySnakes {
		if claimedSnake[si] {
			continue
		}

		plans := d.buildCandidatePlans(si, snIdx, claimedApple)
		if len(plans) == 0 {
			continue
		}

		priority := plans[0].Score
		head := d.G.Sn[snIdx].Body[0]
		if d.enemyHeadNear(head, focusHorizon) {
			priority += 250
		}
		if plans[0].Contested {
			priority += 180
		}

		hot := plans[0].Contested || d.enemyHeadNear(head, focusHorizon)
		if !hot {
			continue
		}

		focus = append(focus, focusSnake{
			si: si, snIdx: snIdx, priority: priority, plans: plans,
		})
	}

	sort.Slice(focus, func(i, j int) bool {
		return focus[i].priority > focus[j].priority
	})
	if len(focus) > focusMaxSnakes {
		focus = focus[:focusMaxSnakes]
	}
	return focus
}

func (d *Decision) buildCandidatePlans(si, snIdx int, claimedApple map[int]bool) []tacticalPlan {
	g := d.G
	sn := &g.Sn[snIdx]
	sim := NewSim(g)
	sim.buildObstacleMap(sn.ID)

	var plans []tacticalPlan
	seenApple := make(map[int]bool, focusMaxCandidates)
	avail := g.Ap[:g.ANum]

	if basePlan, ok := d.buildAssignedPlan(si, snIdx); ok {
		plans = append(plans, basePlan)
		if basePlan.Apple >= 0 {
			seenApple[basePlan.Apple] = true
		}
	}

	for _, ri := range d.BFS.Reach[snIdx] {
		if len(plans) >= focusMaxCandidates {
			break
		}
		if ri.Apple < 0 || ri.Dist <= 0 || ri.Dist > focusHorizon {
			continue
		}
		if claimedApple[ri.Apple] || seenApple[ri.Apple] {
			continue
		}

		contest, ok := d.contestForApple(ri.Apple)
		contested := ok && contest.OpBest >= 0 && absInt(contest.Heat) <= 3
		if !contested && !d.enemyHeadNear(ri.Apple, focusHorizon) {
			continue
		}

		steps, endBody := d.replayPath(sim, sn.Body[:sn.Len], sn.Dir, ri.Apple, d.P.TurnCount, avail)
		if steps == nil || len(steps) == 0 || len(steps) > focusHorizon {
			continue
		}

		plan := tacticalPlan{
			Apple:     ri.Apple,
			Dist:      len(steps),
			FirstDir:  steps[0].Dir,
			EndHead:   endBody[0],
			EndBody:   append([]int(nil), endBody...),
			Contested: contested,
			Steps:     append([]RouteStep(nil), steps...),
		}
		plan.Score = d.scorePlan(snIdx, &plan)
		plans = append(plans, plan)
		seenApple[ri.Apple] = true
	}

	fallback := d.buildFallbackPlans(snIdx)
	plans = append(plans, fallback...)

	sort.Slice(plans, func(i, j int) bool {
		if plans[i].Score != plans[j].Score {
			return plans[i].Score > plans[j].Score
		}
		return plans[i].Dist < plans[j].Dist
	})
	if len(plans) > focusMaxCandidates {
		plans = plans[:focusMaxCandidates]
	}
	return plans
}

func (d *Decision) buildAssignedPlan(si, snIdx int) (tacticalPlan, bool) {
	g := d.G
	sn := &g.Sn[snIdx]
	dir := d.AssignedDir[si]
	if dir < 0 || dir > 3 {
		return tacticalPlan{}, false
	}

	sim := NewSim(g)
	sim.RebuildAppleMap()
	sim.buildObstacleMap(sn.ID)

	newBody, alive := sim.simulateMove(sn.Body[:sn.Len], dir)
	if !alive {
		return tacticalPlan{}, false
	}
	bodycp := append([]int(nil), newBody...)
	eating := false
	apple := d.Assigned[si]
	if apple >= 0 {
		nb := g.Nbm[sn.Body[0]][dir]
		eating = nb == apple
		if eating {
			sim.appleMap[apple] = false
		}
	}
	gravOK := sim.applyGravity(bodycp)
	if eating {
		sim.appleMap[apple] = true
	}
	if !gravOK && !isGroundedByOtherSnake(g, bodycp, snIdx) {
		return tacticalPlan{}, false
	}

	plan := tacticalPlan{
		Apple:    apple,
		Dist:     1,
		FirstDir: dir,
		EndHead:  bodycp[0],
		EndBody:  bodycp,
		Steps: []RouteStep{{
			Apple:   apple,
			Dir:     dir,
			ExpHead: bodycp[0],
			TurnNum: d.P.TurnCount,
		}},
	}
	if contest, ok := d.contestForApple(apple); ok {
		plan.Contested = contest.OpBest >= 0 && absInt(contest.Heat) <= 3
	}
	plan.Score = d.scorePlan(snIdx, &plan) + 60
	return plan, true
}

func (d *Decision) buildFallbackPlans(snIdx int) []tacticalPlan {
	g := d.G
	sn := &g.Sn[snIdx]
	sim := NewSim(g)
	sim.RebuildAppleMap()
	sim.buildObstacleMap(sn.ID)

	var plans []tacticalPlan
	head := sn.Body[0]
	neck := neckOf(sn.Body)

	for dir := 0; dir < 4; dir++ {
		nb := g.Nbm[head][dir]
		if nb < 0 || nb == neck || dir == Do[sn.Dir] {
			continue
		}

		newBody, alive := sim.simulateMove(sn.Body[:sn.Len], dir)
		if !alive || len(newBody) < sn.Len {
			continue
		}

		bodycp := append([]int(nil), newBody...)
		if !sim.applyGravity(bodycp) && !isGroundedByOtherSnake(g, bodycp, snIdx) {
			continue
		}

		blocked := false
		for _, c := range bodycp {
			if c >= 0 && c < g.NCells && sim.obstacleMap[c] {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		step := RouteStep{
			Apple:   -1,
			Dir:     dir,
			ExpHead: bodycp[0],
			TurnNum: d.P.TurnCount,
		}
		plan := tacticalPlan{
			Apple:    -1,
			Dist:     1,
			FirstDir: dir,
			EndHead:  bodycp[0],
			EndBody:  bodycp,
			Steps:    []RouteStep{step},
		}
		plan.Score = d.scorePlan(snIdx, &plan)
		plans = append(plans, plan)
	}

	return plans
}

func (d *Decision) scorePlan(snIdx int, plan *tacticalPlan) int {
	g := d.G
	sn := &g.Sn[snIdx]

	score := 0
	score += minInt(d.endFlood(snIdx, plan.EndBody), 32) * 10
	score -= plan.Dist * 90

	if plan.Apple >= 0 {
		score += 650
		if contest, ok := d.contestForApple(plan.Apple); ok {
			switch {
			case contest.OpBest < 0:
				score += 220
			default:
				score += contest.Heat * 55
				if absInt(contest.Heat) <= 2 {
					score += 120
				}
			}
		}
		nextDist := d.nearestAppleDistance(plan.EndHead, plan.Apple)
		if nextDist < 99 {
			score += (12 - minInt(nextDist, 12)) * 12
		}
	} else if d.enemyHeadNear(plan.EndHead, 4) {
		score += 60
	}

	if !g.IsInGrid(plan.EndHead) {
		score -= 400
	}
	if len(plan.EndBody) < crampedThreshold(sn.Len) {
		score -= 120
	}
	if d.enemyHeadNear(plan.EndHead, 2) {
		score -= 80
	}

	return score
}

func (d *Decision) endFlood(snIdx int, body []int) int {
	g := d.G
	if len(body) == 0 {
		return 0
	}

	sim := NewSim(g)
	blocked := make([]bool, g.NCells)
	visited := make([]bool, g.NCells)
	queue := make([]int, g.NCells)
	clearBlocked(blocked, g)
	markOtherBodies(blocked, g, sim, snIdx)
	for _, c := range body[1:] {
		if c >= 0 && c < g.NCells {
			blocked[c] = true
		}
	}
	return bfsFlood(g, blocked, visited, queue, body[0])
}

func (d *Decision) nearestAppleDistance(cell, exclude int) int {
	best := 99
	for i := 0; i < d.G.ANum; i++ {
		ap := d.G.Ap[i]
		if ap == exclude {
			continue
		}
		dist := d.G.Manhattan(cell, ap)
		if dist < best {
			best = dist
		}
	}
	return best
}

func (d *Decision) runFocusedSearch(focus []focusSnake, claimedSnake []bool, claimedApple map[int]bool) {
	if len(focus) == 0 {
		return
	}
	if len(focus) == 1 {
		if len(focus[0].plans) > 1 && focus[0].plans[1].Score > focus[0].plans[0].Score+focusOverrideMargin {
			d.applyPlan(focus[0], focus[0].plans[1], claimedSnake, claimedApple)
		}
		return
	}

	rng := rand.New(rand.NewSource(d.focusSeed(focus)))
	pop := make([]focusGene, focusPopulation)
	base := focusGene{}
	for i := range focus {
		base.choice[i] = 0
	}
	base.score = d.evaluateGene(focus, base)
	pop[0] = base

	for i := 1; i < len(pop); i++ {
		pop[i] = d.randomGene(focus, rng)
		pop[i].score = d.evaluateGene(focus, pop[i])
	}

	for gen := 0; gen < focusGenerations; gen++ {
		sort.Slice(pop, func(i, j int) bool {
			return pop[i].score > pop[j].score
		})

		next := make([]focusGene, 0, len(pop))
		elite := minInt(focusEliteCount, len(pop))
		next = append(next, pop[:elite]...)

		for len(next) < len(pop) {
			a := pop[d.tournament(pop, rng)]
			b := pop[d.tournament(pop, rng)]
			child := a

			if len(focus) > 1 {
				cut := rng.Intn(len(focus))
				for i := cut; i < len(focus); i++ {
					child.choice[i] = b.choice[i]
				}
			}

			for i := range focus {
				if rng.Intn(100) < 35 {
					child.choice[i] = rng.Intn(len(focus[i].plans))
				}
			}
			child.score = d.evaluateGene(focus, child)
			next = append(next, child)
		}
		pop = next
	}

	sort.Slice(pop, func(i, j int) bool {
		return pop[i].score > pop[j].score
	})
	best := pop[0]
	if best.score <= base.score+focusOverrideMargin {
		return
	}
	for i, fs := range focus {
		if best.choice[i] == 0 {
			continue
		}
		d.applyPlan(fs, fs.plans[best.choice[i]], claimedSnake, claimedApple)
	}
}

func (d *Decision) randomGene(focus []focusSnake, rng *rand.Rand) focusGene {
	var gene focusGene
	for i := range focus {
		gene.choice[i] = rng.Intn(len(focus[i].plans))
	}
	return gene
}

func (d *Decision) tournament(pop []focusGene, rng *rand.Rand) int {
	best := rng.Intn(len(pop))
	for k := 1; k < focusTournamentSize; k++ {
		idx := rng.Intn(len(pop))
		if pop[idx].score > pop[best].score {
			best = idx
		}
	}
	return best
}

func (d *Decision) evaluateGene(focus []focusSnake, gene focusGene) int {
	score := 0
	firstHeads := make(map[int]int, len(focus))
	apples := make(map[int]int, len(focus))
	firstLens := make(map[int]int, len(focus))

	for i, fs := range focus {
		plan := fs.plans[gene.choice[i]]
		score += plan.Score

		if len(plan.Steps) > 0 {
			firstHeads[plan.Steps[0].ExpHead]++
			firstLens[plan.Steps[0].ExpHead] = len(plan.EndBody)
		}
		if plan.Apple >= 0 {
			apples[plan.Apple]++
			if plan.Contested {
				score += 50
			}
		}
	}

	for _, n := range firstHeads {
		if n > 1 {
			score -= 1200 * (n - 1)
		}
	}
	for _, n := range apples {
		if n > 1 {
			score -= 450 * (n - 1)
		}
	}
	score += d.enemyPressureScore(focus, gene, firstHeads, firstLens)

	for i := 0; i < len(focus); i++ {
		pi := focus[i].plans[gene.choice[i]]
		for j := i + 1; j < len(focus); j++ {
			pj := focus[j].plans[gene.choice[j]]
			if pi.EndHead == pj.EndHead {
				score -= 600
				continue
			}
			dist := d.G.Manhattan(pi.EndHead, pj.EndHead)
			if dist <= 1 {
				score -= 120
			} else {
				score += minInt(dist, 4) * 12
			}
		}
	}

	return score
}

func (d *Decision) enemyPressureScore(
	focus []focusSnake,
	gene focusGene,
	firstHeads map[int]int,
	firstLens map[int]int,
) int {
	g := d.G
	score := 0

	blocked := make([]bool, g.NCells)
	for cell := range firstHeads {
		if cell >= 0 && cell < g.NCells {
			blocked[cell] = true
		}
	}

	for _, opIdx := range d.OpSnakes {
		op := &g.Sn[opIdx]
		if !op.Alive || op.Len == 0 {
			continue
		}

		head := op.Body[0]
		if head < 0 || head >= g.NCells {
			continue
		}
		neck := neckOf(op.Body)
		origMoves := 0
		remainingMoves := 0
		bestEnemyNext := -1
		bestEnemyAppleDist := 1 << 30

		for dir := 0; dir < 4; dir++ {
			nb := g.Nbm[head][dir]
			if nb < 0 || nb == neck {
				continue
			}
			origMoves++
			if !blocked[nb] {
				remainingMoves++
			}

			appleDist := 1 << 30
			for _, ri := range d.BFS.Reach[opIdx] {
				if ri.FirstDir == dir && ri.Dist < appleDist {
					appleDist = ri.Dist
				}
			}
			if appleDist < bestEnemyAppleDist {
				bestEnemyAppleDist = appleDist
				bestEnemyNext = nb
			}
		}

		score += (origMoves - remainingMoves) * 120

		if bestEnemyNext >= 0 {
			if myLen, ok := firstLens[bestEnemyNext]; ok {
				switch {
				case myLen > op.Len:
					score += 260
				case myLen < op.Len:
					score -= 260
				default:
					score -= 60
				}
			}
		}

		for i := range focus {
			plan := focus[i].plans[gene.choice[i]]
			if len(plan.Steps) == 0 {
				continue
			}
			myHead := plan.Steps[0].ExpHead
			dist := g.Manhattan(myHead, head)
			switch {
			case dist == 1 && len(plan.EndBody) > op.Len:
				score += 70
			case dist == 1 && len(plan.EndBody) <= op.Len:
				score -= 40
			case dist == 2 && len(plan.EndBody) > op.Len:
				score += 20
			}
		}
	}

	return score
}

func (d *Decision) applyPlan(fs focusSnake, plan tacticalPlan, claimedSnake []bool, claimedApple map[int]bool) {
	d.Assigned[fs.si] = plan.Apple
	d.AssignedDir[fs.si] = plan.FirstDir
	claimedSnake[fs.si] = true
	if plan.Apple >= 0 {
		claimedApple[plan.Apple] = true
	}
}

func (d *Decision) storeGreedyRoute(si, snIdx, apple int) {
	g := d.G
	sn := &g.Sn[snIdx]
	sim := NewSim(g)
	sim.buildObstacleMap(sn.ID)

	steps, _ := d.replayPath(sim, sn.Body[:sn.Len], sn.Dir, apple, d.P.TurnCount, g.Ap[:g.ANum])
	if len(steps) == 0 {
		d.storeStepRoute(si, snIdx, d.AssignedDir[si], apple)
		return
	}

	route := &d.P.Routes[si]
	route.SnIdx = snIdx
	route.AppleSeq = route.AppleSeq[:0]
	route.AppleSeq = append(route.AppleSeq, apple)
	route.Steps = append(route.Steps[:0], steps...)
	route.StepCursor = 0
	route.Valid = true
}

func (d *Decision) storeStepRoute(si, snIdx, dir, apple int) {
	g := d.G
	sn := &g.Sn[snIdx]
	head := sn.Body[0]
	expHead := g.Nbm[head][dir]
	if expHead < 0 {
		expHead = head
	}

	route := &d.P.Routes[si]
	route.SnIdx = snIdx
	route.AppleSeq = route.AppleSeq[:0]
	if apple >= 0 {
		route.AppleSeq = append(route.AppleSeq, apple)
	}
	route.Steps = route.Steps[:0]
	route.Steps = append(route.Steps, RouteStep{
		Apple:   apple,
		Dir:     dir,
		ExpHead: expHead,
		TurnNum: d.P.TurnCount,
	})
	route.StepCursor = 0
	route.Valid = true
}

func (d *Decision) contestForApple(apple int) (AppleContest, bool) {
	for i := 0; i < d.G.ANum; i++ {
		if d.G.Ap[i] == apple {
			return d.Influence[i], true
		}
	}
	return AppleContest{}, false
}

func (d *Decision) enemyHeadNear(cell, maxDist int) bool {
	for _, snIdx := range d.OpSnakes {
		sn := &d.G.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}
		if d.G.Manhattan(cell, sn.Body[0]) <= maxDist {
			return true
		}
	}
	return false
}

func (d *Decision) focusSeed(focus []focusSnake) int64 {
	seed := int64(1469598103934665603)
	seed ^= int64(d.P.TurnCount + 1)
	for _, fs := range focus {
		head := d.G.Sn[fs.snIdx].Body[0]
		seed = seed*1099511628211 + int64(head+17)
	}
	return seed
}

func (d *Decision) shouldFocusOverride(si, snIdx int) bool {
	head := d.G.Sn[snIdx].Body[0]
	if d.enemyHeadNear(head, 5) {
		return true
	}
	apple := d.Assigned[si]
	if apple < 0 {
		return false
	}
	contest, ok := d.contestForApple(apple)
	if !ok {
		return false
	}
	if contest.OpBest < 0 {
		return false
	}
	return absInt(contest.Heat) <= 3 || contest.Heat < 0
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// replayPath does body-sim BFS from startBody to targetApple, recording
// full direction sequence. Returns RouteStep slice and final body state.
func (d *Decision) replayPath(
	sim *Sim,
	startBody []int,
	startDir int,
	targetApple int,
	startTurn int,
	avail []int,
) ([]RouteStep, []int) {
	g := d.G
	sim.rebuildAppleMapFrom(avail)

	type rpNode struct {
		body    []int
		dist    int
		prevDir int
		dirs    []int
	}

	startCopy := make([]int, len(startBody))
	copy(startCopy, startBody)

	visited := make(map[uint64]bool)
	visited[bodyHash(startCopy)] = true

	queue := []rpNode{{
		body:    startCopy,
		prevDir: startDir,
	}}

	const replayMaxNodes = 2000

	for qi := 0; qi < len(queue) && qi < replayMaxNodes; qi++ {
		cur := queue[qi]
		if cur.dist >= focusReplayDepth {
			continue
		}

		curHead := cur.body[0]
		if !g.IsInGrid(curHead) {
			continue
		}
		neck := neckOf(cur.body)

		for dir := 0; dir < 4; dir++ {
			nc := g.Nbm[curHead][dir]
			if nc < 0 || nc == neck {
				continue
			}
			if cur.prevDir >= 0 && dir == Do[cur.prevDir] {
				continue
			}

			newBody, alive := sim.simulateMove(cur.body, dir)
			if !alive || len(newBody) < len(cur.body) {
				continue
			}

			bodycp := append([]int(nil), newBody...)
			eating := nc == targetApple
			if eating {
				sim.appleMap[nc] = false
			}
			gravOK := sim.applyGravity(bodycp)
			if eating {
				sim.appleMap[nc] = true
			}
			if !gravOK {
				continue
			}

			blocked := false
			for _, c := range bodycp {
				if c >= 0 && c < g.NCells && sim.obstacleMap[c] {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}

			h := bodyHash(bodycp)
			if visited[h] {
				continue
			}
			visited[h] = true

			newDirs := make([]int, len(cur.dirs)+1)
			copy(newDirs, cur.dirs)
			newDirs[len(cur.dirs)] = dir

			if eating {
				return d.buildRouteSteps(sim, startBody, newDirs, targetApple, startTurn, avail), bodycp
			}

			queue = append(queue, rpNode{
				body:    bodycp,
				dist:    cur.dist + 1,
				prevDir: dir,
				dirs:    newDirs,
			})
		}
	}
	return nil, nil
}

// buildRouteSteps forward-replays a direction sequence to produce RouteSteps
// with ExpHead for each step.
func (d *Decision) buildRouteSteps(
	sim *Sim,
	startBody []int,
	dirs []int,
	targetApple int,
	startTurn int,
	avail []int,
) []RouteStep {
	sim.rebuildAppleMapFrom(avail)

	curBody := make([]int, len(startBody))
	copy(curBody, startBody)

	steps := make([]RouteStep, len(dirs))
	for i, dir := range dirs {
		newBody, _ := sim.simulateMove(curBody, dir)
		bodycp := append([]int(nil), newBody...)

		nc := d.G.Nbm[curBody[0]][dir]
		eating := nc == targetApple
		if eating {
			sim.appleMap[nc] = false
		}
		sim.applyGravity(bodycp)
		if eating {
			sim.appleMap[nc] = true
		}

		apple := -1
		if i == len(dirs)-1 {
			apple = targetApple
		}
		steps[i] = RouteStep{
			Apple:   apple,
			Dir:     dir,
			ExpHead: bodycp[0],
			TurnNum: startTurn + i,
		}
		curBody = bodycp
	}

	return steps
}
