package main

// --- Phase: Partition (replaces phaseScoring + phaseAssignment) ---
// Turn 1: sequential body-sim route planning for all bots.
// Later turns: validate plan, follow it, fall back to greedy when broken.

const (
	routeMaxDepth    = 8  // max apples per bot in initial sim plan
	routeMaxSteps    = 40 // max steps per bot route (safety cap)
	routeReplayDepth = 15 // max BFS depth in replayPath
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

// greedyFallback assigns the closest reachable apple not claimed by other routes.
func (d *Decision) greedyFallback(si, snIdx int) {
	g := d.G
	p := d.P
	sn := &g.Sn[snIdx]

	reach := d.BFS.Reach[snIdx]
	if len(reach) == 0 {
		d.AssignedDir[si] = fallbackDir(g, sn)
		return
	}

	// Pick closest apple not claimed by other valid routes.
	for _, ri := range reach {
		if ri.Apple >= 0 && ri.Apple < len(p.PlannedApples) && p.PlannedApples[ri.Apple] {
			continue
		}
		d.Assigned[si] = ri.Apple
		d.AssignedDir[si] = ri.FirstDir
		return
	}

	// All claimed — just pick absolute closest.
	d.Assigned[si] = reach[0].Apple
	d.AssignedDir[si] = reach[0].FirstDir
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
		reach := d.BFS.Reach[snIdx]

		if route.Valid && route.StepCursor < len(route.Steps) {
			// Count steps until next apple in route.
			stepsToRouteApple := 0
			for k := route.StepCursor; k < len(route.Steps); k++ {
				stepsToRouteApple++
				if route.Steps[k].Apple >= 0 {
					break
				}
			}

			// If greedy BFS finds a closer apple than the route's
			// next target, override the route. This prevents bots from
			// walking past nearby apples to reach a distant planned target.
			if len(reach) > 0 && reach[0].Dist < stepsToRouteApple {
				d.Assigned[si] = reach[0].Apple
				d.AssignedDir[si] = reach[0].FirstDir
				route.Valid = false
				continue
			}

			step := route.Steps[route.StepCursor]
			d.AssignedDir[si] = step.Dir
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

// planAllRoutes assigns apples using iterative Voronoi simulation.
// Each round: all bots BFS to find their closest apple → Voronoi tiebreak
// (closest wins) → winner simulates eating (body grows, gravity) →
// apple removed → repeat with updated state.
func (d *Decision) planAllRoutes() {
	g := d.G
	myN := len(d.MySnakes)
	if myN == 0 {
		return
	}

	// Predict which apples the enemy will likely eat.
	// An apple is "enemy-certain" if the closest enemy reaches it
	// strictly faster than ANY of our bots (heat < 0 from influence).
	// Only remove apples with significant enemy advantage (heat <= -2).
	enemyWillEat := make(map[int]bool)
	for a := 0; a < g.ANum; a++ {
		inf := &d.Influence[a]
		if inf.Heat <= -5 && inf.OpBest >= 0 {
			enemyWillEat[g.Ap[a]] = true
		}
	}

	avail := make([]int, 0, g.ANum)
	for i := 0; i < g.ANum; i++ {
		if !enemyWillEat[g.Ap[i]] {
			avail = append(avail, g.Ap[i])
		}
	}
	sim := NewSim(g)

	// Per-bot simulated state: body evolves as apples are "eaten".
	type botState struct {
		body    []int
		dir     int
		turnNum int
		alive   bool
	}
	states := make([]botState, myN)
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		states[si] = botState{
			body:  append([]int(nil), sn.Body[:sn.Len]...),
			dir:   sn.Dir,
			alive: sn.Alive && sn.Len > 0,
		}
	}

	// Init routes.
	for si, snIdx := range d.MySnakes {
		r := &d.P.Routes[si]
		r.SnIdx = snIdx
		r.Valid = true
		r.StepCursor = 0
		r.AppleSeq = r.AppleSeq[:0]
		r.Steps = r.Steps[:0]
	}

	// Iterative assignment: one apple per bot per round.
	for round := 0; round < routeMaxDepth; round++ {
		if len(avail) == 0 {
			break
		}

		sim.rebuildAppleMapFrom(avail)

		// Each alive bot finds its closest reachable apple.
		type candidate struct {
			si    int
			apple int
			dist  int
			target SimTarget
		}
		var candidates []candidate

		for si, snIdx := range d.MySnakes {
			if !states[si].alive {
				continue
			}
			// Bot has enough apples — freeze it, leave apples for others.
			// Use lower threshold than routeMaxDepth so greedy bots don't hoard.
			if len(d.P.Routes[si].AppleSeq) >= 4 {
				continue
			}
			sn := &g.Sn[snIdx]
			tmpSn := &Snake{
				ID:    sn.ID,
				Owner: sn.Owner,
				Body:  states[si].body,
				Len:   len(states[si].body),
				Dir:   states[si].dir,
				Alive: true,
			}

			targets := sim.SimBFSApples(tmpSn)
			if len(targets) == 0 {
				continue
			}
			// Score each apple: lower = better.
			// dist is base cost. Negative heat (enemy closer) adds penalty.
			// Positive heat (we're closer) gives bonus.
			bestScore := 1 << 30
			bestIdx := 0
			for i, t := range targets {
				score := t.Dist * 10
				heat := 0
				if t.Apple >= 0 && t.Apple < MaxExpandedCells {
					h := d.HeatByCell[t.Apple]
					if h != heatUnreachable && h != heatExclusive {
						heat = h
					}
				}
				if heat < 0 {
					score += (-heat) * 5 // mild penalty for enemy-favored
				} else if heat > 0 {
					score -= heat * 5 // bonus for our advantage
				}
				if score < bestScore {
					bestScore = score
					bestIdx = i
				}
			}
			best := targets[bestIdx]
			candidates = append(candidates, candidate{si: si, apple: best.Apple, dist: best.Dist, target: best})
		}

		if len(candidates) == 0 {
			break
		}

		// Voronoi: for each apple, give to the bot with shortest dist.
		// Group by apple, pick winner.
		assigned := make(map[int]candidate) // apple → winning candidate
		for _, c := range candidates {
			if prev, ok := assigned[c.apple]; !ok || c.dist < prev.dist {
				assigned[c.apple] = c
			}
		}

		// Winners: simulate eating, record route steps.
		// Losers: skip this round (will try again next round with updated state).
		winners := make(map[int]bool) // bot slot → assigned this round
		for _, winner := range assigned {
			si := winner.si
			if winners[si] {
				continue // bot already won an apple this round
			}
			winners[si] = true

			sn := &g.Sn[d.MySnakes[si]]
			route := &d.P.Routes[si]

			// Rebuild obstacle map for this bot before replay.
			sim.buildObstacleMap(sn.ID)
			// Replay path to record steps.
			steps, finalBody := d.replayPath(sim, states[si].body, states[si].dir,
				winner.apple, states[si].turnNum, avail)
			if steps == nil {
				continue // unreachable after all
			}

			route.AppleSeq = append(route.AppleSeq, winner.apple)
			route.Steps = append(route.Steps, steps...)
			states[si].turnNum += len(steps)
			states[si].body = finalBody
			if len(finalBody) >= 2 {
				states[si].dir = g.DirFromTo(finalBody[1], finalBody[0])
			}
			_ = sn

			// Remove apple from avail.
			for j := 0; j < len(avail); j++ {
				if avail[j] == winner.apple {
					avail[j] = avail[len(avail)-1]
					avail = avail[:len(avail)-1]
					break
				}
			}
		}

		if len(winners) == 0 {
			break // no progress
		}
	}

	// Sequential fill: bots with < routeMaxDepth apples get more from remaining pool.
	// Process bots by fewest apples first (most hungry first).
	for fill := 0; fill < routeMaxDepth; fill++ {
		if len(avail) == 0 {
			break
		}
		progress := false
		for si, snIdx := range d.MySnakes {
			if len(d.P.Routes[si].AppleSeq) >= routeMaxDepth {
				continue
			}
			if !states[si].alive {
				continue
			}
			sn := &g.Sn[snIdx]

			sim.rebuildAppleMapFrom(avail)
			sim.buildObstacleMap(sn.ID)

			tmpSn := &Snake{
				ID:    sn.ID,
				Owner: sn.Owner,
				Body:  states[si].body,
				Len:   len(states[si].body),
				Dir:   states[si].dir,
				Alive: true,
			}

			targets := sim.SimBFSApples(tmpSn)
			if len(targets) == 0 {
				continue
			}

			best := targets[0]
			for _, t := range targets[1:] {
				if t.Dist < best.Dist {
					best = t
				}
			}

			steps, finalBody := d.replayPath(sim, states[si].body, states[si].dir,
				best.Apple, states[si].turnNum, avail)
			if steps == nil {
				continue
			}

			route := &d.P.Routes[si]
			route.AppleSeq = append(route.AppleSeq, best.Apple)
			route.Steps = append(route.Steps, steps...)
			states[si].turnNum += len(steps)
			states[si].body = finalBody
			if len(finalBody) >= 2 {
				states[si].dir = g.DirFromTo(finalBody[1], finalBody[0])
			}

			for j := 0; j < len(avail); j++ {
				if avail[j] == best.Apple {
					avail[j] = avail[len(avail)-1]
					avail = avail[:len(avail)-1]
					break
				}
			}
			progress = true
		}
		if !progress {
			break
		}
	}

	for si := range d.MySnakes {
		if len(d.P.Routes[si].AppleSeq) == 0 {
			d.P.Routes[si].Valid = false
		}
	}
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

		// Filter out already-claimed apples.
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

		// Pick closest apple — guaranteed points beat speculative plans.
		best := d.pickBestTarget(filtered)

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

// pickBestTarget picks the closest reachable apple. Always closest first.
func (d *Decision) pickBestTarget(targets []SimTarget) SimTarget {
	best := 0
	for i, t := range targets {
		if t.Dist < targets[best].Dist {
			best = i
		}
	}
	return targets[best]
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

	const replayMaxNodes = 2000 // cap BFS to prevent timeout with long bodies

	for qi := 0; qi < len(queue) && qi < replayMaxNodes; qi++ {
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
