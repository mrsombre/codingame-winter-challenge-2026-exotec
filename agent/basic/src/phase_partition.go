package main

import "sort"

// --- Phase: Partition (replaces phaseScoring + phaseAssignment) ---
// Turn 1: sequential body-sim route planning for all bots.
// Later turns: validate plan, follow it, fall back to greedy when broken.

const (
	routeMaxDepth    = 5  // max apples per bot in initial sim plan
	routeMaxSteps    = 40 // max steps per bot route (safety cap)
	routeReplayDepth = 25 // max BFS depth in replayPath

	greedyK     = 5  // K nearest neighbors for density scoring
	greedyAlpha = 60 // density weight (percent of dist unit)
)

// phasePartition is the main dispatcher.
func (d *Decision) phasePartition() {
	p := d.P

	if !p.Initialized {
		d.planAllRoutes()
		p.Initialized = true
	} else {
		d.validateRoutes()
	}

	d.rebuildPlannedApples()
	d.executeRoutes()
}

// --- Greedy fallback ---

// greedyFallback assigns a target apple for snake slot si using density-aware greedy.
func (d *Decision) greedyFallback(si, snIdx int) {
	g := d.G
	p := d.P
	sn := &g.Sn[snIdx]

	reach := d.BFS.Reach[snIdx]
	if len(reach) == 0 {
		d.AssignedDir[si] = fallbackDir(g, sn)
		return
	}

	// Filter out apples claimed by other valid routes.
	var candidates []ReachInfo
	for _, ri := range reach {
		if ri.Apple >= 0 && ri.Apple < len(p.PlannedApples) && p.PlannedApples[ri.Apple] {
			continue
		}
		candidates = append(candidates, ri)
	}
	if len(candidates) == 0 {
		candidates = reach // all claimed; ignore exclusion
	}

	bestScore := -1 << 30
	bestIdx := 0

	for i, ri := range candidates {
		score := -ri.Dist * 100

		// Density bonus: prefer apples with nearby neighbors.
		score += densityBonus(g, ri.Apple, candidates)

		// Heat bonus: prefer apples where we have advantage.
		if ri.Apple >= 0 && ri.Apple < MaxExpandedCells {
			heat := d.HeatByCell[ri.Apple]
			if heat > 0 {
				score += heat * 10
			}
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}

	d.Assigned[si] = candidates[bestIdx].Apple
	d.AssignedDir[si] = candidates[bestIdx].FirstDir
}

// densityBonus returns a score bonus based on how many other reachable apples
// are close to the given apple. Prefers picking apples in dense areas.
func densityBonus(g *Game, apple int, candidates []ReachInfo) int {
	dists := make([]int, 0, len(candidates))
	for _, ri := range candidates {
		if ri.Apple == apple {
			continue
		}
		dists = append(dists, g.Manhattan(apple, ri.Apple))
	}
	sort.Ints(dists)

	bonus := 0
	for i := 0; i < greedyK && i < len(dists); i++ {
		bonus -= dists[i] * greedyAlpha / 100
	}
	return bonus
}

// --- Plan execution ---

// executeRoutes fills Assigned/AssignedDir from valid routes or greedy fallback.
func (d *Decision) executeRoutes() {
	g := d.G
	p := d.P

	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}

		route := &p.Routes[si]

		if route.Valid && route.StepCursor < len(route.Steps) {
			step := route.Steps[route.StepCursor]
			d.AssignedDir[si] = step.Dir
			// Set Assigned to the next uncollected apple in the sequence.
			d.Assigned[si] = nextRouteApple(route)
			route.StepCursor++
		} else {
			d.greedyFallback(si, snIdx)
		}
	}
}

// --- Plan validation ---

// validateRoutes checks each route's integrity and marks invalid if broken.
func (d *Decision) validateRoutes() {
	p := d.P
	g := d.G

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

		// Remove eaten apples from sequence.
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

		// Check if we're still on plan (head matches expected position).
		if route.StepCursor < len(route.Steps) {
			step := &route.Steps[route.StepCursor]
			if sn.Body[0] != step.ExpHead {
				// Position drifted (safety override or enemy interaction).
				// Try to re-plan from current position toward remaining apples.
				d.replanRoute(si, snIdx, route, appleAlive)
			}
		} else {
			// Steps exhausted but apples remain — re-plan.
			d.replanRoute(si, snIdx, route, appleAlive)
		}
	}
}

// replanRoute rebuilds steps for a route from the bot's current position
// toward remaining apples in AppleSeq.
func (d *Decision) replanRoute(si, snIdx int, route *BotRoute, appleAlive []bool) {
	g := d.G
	sn := &g.Sn[snIdx]

	sim := NewSim(g)
	avail := make([]int, len(route.AppleSeq))
	copy(avail, route.AppleSeq)
	sim.rebuildAppleMapFrom(avail)

	curBody := make([]int, sn.Len)
	copy(curBody, sn.Body[:sn.Len])
	curDir := sn.Dir
	turnNum := d.P.TurnCount

	route.Steps = route.Steps[:0]
	route.StepCursor = 0
	var newSeq []int

	for _, ap := range route.AppleSeq {
		if len(route.Steps) >= routeMaxSteps {
			break
		}
		if !appleAlive[ap] {
			continue
		}

		steps, finalBody := d.replayPath(sim, curBody, curDir, ap, turnNum, avail)
		if steps == nil {
			continue // skip unreachable apple
		}

		newSeq = append(newSeq, ap)
		route.Steps = append(route.Steps, steps...)
		turnNum += len(steps)

		// Remove from avail.
		for j := 0; j < len(avail); j++ {
			if avail[j] == ap {
				avail[j] = avail[len(avail)-1]
				avail = avail[:len(avail)-1]
				break
			}
		}
		sim.rebuildAppleMapFrom(avail)
		curBody = finalBody
		if len(curBody) >= 2 {
			curDir = g.DirFromTo(curBody[1], curBody[0])
		}
	}

	route.AppleSeq = newSeq
	if len(route.AppleSeq) == 0 {
		route.Valid = false
	}
}

// rebuildPlannedApples rebuilds the PlannedApples set from valid routes.
func (d *Decision) rebuildPlannedApples() {
	p := d.P
	for i := range p.PlannedApples {
		p.PlannedApples[i] = false
	}
	for si := range d.MySnakes {
		r := &p.Routes[si]
		if !r.Valid {
			continue
		}
		for _, ap := range r.AppleSeq {
			if ap >= 0 && ap < len(p.PlannedApples) {
				p.PlannedApples[ap] = true
			}
		}
	}
}

// --- Route planning (turn 1) ---

// planAllRoutes runs sequential body-sim route planning for all my bots.
// Tries all bot orderings and keeps the one that claims the most apples.
func (d *Decision) planAllRoutes() {
	g := d.G
	myN := len(d.MySnakes)
	if myN == 0 {
		return
	}

	// Build available apple list.
	avail := make([]int, g.ANum)
	copy(avail, g.Ap[:g.ANum])

	sim := NewSim(g)

	// Try all permutations of bot ordering; keep best total.
	perms := permutations(myN)
	bestTotal := -1
	var bestRoutes [MaxPSn]BotRoute

	for _, perm := range perms {
		// Fresh apple set for this permutation.
		curAvail := make([]int, len(avail))
		copy(curAvail, avail)

		var routes [MaxPSn]BotRoute
		total := 0

		for _, si := range perm {
			snIdx := d.MySnakes[si]
			sn := &g.Sn[snIdx]
			if !sn.Alive || sn.Len == 0 {
				continue
			}

			route := &routes[si]
			route.SnIdx = snIdx
			route.Valid = true
			route.StepCursor = 0

			d.simRouteForBot(sim, sn, route, curAvail)

			// Remove claimed apples from available set.
			for _, ap := range route.AppleSeq {
				for j := 0; j < len(curAvail); j++ {
					if curAvail[j] == ap {
						curAvail[j] = curAvail[len(curAvail)-1]
						curAvail = curAvail[:len(curAvail)-1]
						break
					}
				}
			}
			total += len(route.AppleSeq)
		}

		if total > bestTotal {
			bestTotal = total
			bestRoutes = routes
		}
	}

	d.P.Routes = bestRoutes
}

// simRouteForBot plans a route for one bot through up to routeMaxDepth apples.
func (d *Decision) simRouteForBot(sim *Sim, sn *Snake, route *BotRoute, avail []int) {
	g := d.G

	curBody := make([]int, sn.Len)
	copy(curBody, sn.Body[:sn.Len])
	curDir := sn.Dir
	turnNum := d.P.TurnCount

	route.AppleSeq = route.AppleSeq[:0]
	route.Steps = route.Steps[:0]

	for depth := 0; depth < routeMaxDepth; depth++ {
		if len(route.Steps) >= routeMaxSteps {
			break
		}

		sim.rebuildAppleMapFrom(avail)

		// Build a temporary snake for SimBFSApples.
		tmpSn := &Snake{
			ID:    sn.ID,
			Owner: sn.Owner,
			Body:  curBody,
			Len:   len(curBody),
			Dir:   curDir,
			Alive: true,
		}

		targets := sim.SimBFSApples(tmpSn)
		if len(targets) == 0 {
			break
		}

		// Filter out already-claimed apples from targets.
		var filtered []SimTarget
		for _, t := range targets {
			dup := false
			for _, ap := range route.AppleSeq {
				if t.Apple == ap {
					dup = true
					break
				}
			}
			if !dup {
				filtered = append(filtered, t)
			}
		}
		if len(filtered) == 0 {
			break
		}

		// Pick best target using density scoring.
		best := d.pickBestTarget(filtered, avail)

		// Replay path from current body to this apple.
		steps, finalBody := d.replayPath(sim, curBody, curDir, best.Apple, turnNum, avail)
		if steps == nil {
			break // can't reach
		}

		route.AppleSeq = append(route.AppleSeq, best.Apple)
		route.Steps = append(route.Steps, steps...)
		turnNum += len(steps)

		// Remove eaten apple from available set.
		for j := 0; j < len(avail); j++ {
			if avail[j] == best.Apple {
				avail[j] = avail[len(avail)-1]
				avail = avail[:len(avail)-1]
				break
			}
		}

		curBody = finalBody
		if len(curBody) >= 2 {
			curDir = g.DirFromTo(curBody[1], curBody[0])
		}
	}

	if len(route.AppleSeq) == 0 {
		route.Valid = false
	}
}

// pickBestTarget scores SimTargets with density awareness.
func (d *Decision) pickBestTarget(targets []SimTarget, avail []int) SimTarget {
	g := d.G
	bestScore := -1 << 30
	bestIdx := 0

	for i, t := range targets {
		score := -t.Dist * 100

		// Density: count how many available apples are near this target.
		for _, ap := range avail {
			if ap == t.Apple {
				continue
			}
			md := g.Manhattan(t.Apple, ap)
			if md <= greedyK {
				score += (greedyK - md) * greedyAlpha / 100
			}
		}

		if score > bestScore {
			bestScore = score
			bestIdx = i
		}
	}
	return targets[bestIdx]
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

	for qi := 0; qi < len(queue); qi++ {
		cur := queue[qi]
		if cur.dist >= routeReplayDepth {
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
			// Reject backward.
			if cur.prevDir >= 0 && dir == Do[cur.prevDir] {
				continue
			}

			newBody, alive := sim.simulateMove(cur.body, dir)
			if !alive {
				continue
			}
			if len(newBody) < len(cur.body) {
				continue // beheading
			}

			bodycp := make([]int, len(newBody))
			copy(bodycp, newBody)

			eating := (nc == targetApple)

			// Temporarily remove apple for gravity check.
			if eating {
				sim.appleMap[nc] = false
			}
			gravOk := sim.applyGravity(bodycp)
			if eating {
				sim.appleMap[nc] = true
			}
			if !gravOk {
				continue
			}

			// Obstacle check.
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
				// Found! Forward-replay dirs to build RouteSteps.
				return d.buildRouteSteps(sim, startBody, startDir, newDirs, targetApple, startTurn, avail), bodycp
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
	startDir int,
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
		bodycp := make([]int, len(newBody))
		copy(bodycp, newBody)

		nc := d.G.Nbm[curBody[0]][dir]
		eating := (nc == targetApple)

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

// --- Helpers ---

// nextRouteApple returns the first apple in the route's remaining sequence.
func nextRouteApple(route *BotRoute) int {
	if len(route.AppleSeq) > 0 {
		return route.AppleSeq[0]
	}
	return -1
}

// permutations returns all orderings of [0..n-1] for n <= 4.
func permutations(n int) [][]int {
	if n <= 0 {
		return nil
	}
	if n == 1 {
		return [][]int{{0}}
	}
	var result [][]int
	var perm func([]int, int)
	perm = func(arr []int, k int) {
		if k == 1 {
			cp := make([]int, len(arr))
			copy(cp, arr)
			result = append(result, cp)
			return
		}
		for i := 0; i < k; i++ {
			perm(arr, k-1)
			if k%2 == 0 {
				arr[i], arr[k-1] = arr[k-1], arr[i]
			} else {
				arr[0], arr[k-1] = arr[k-1], arr[0]
			}
		}
	}
	arr := make([]int, n)
	for i := range arr {
		arr[i] = i
	}
	perm(arr, n)
	return result
}
