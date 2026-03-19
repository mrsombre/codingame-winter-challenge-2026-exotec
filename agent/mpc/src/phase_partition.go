package main

// --- Phase: Partition (replaces phaseScoring + phaseAssignment) ---
// Turn 1: sequential body-sim route planning for all bots.
// Later turns: validate plan, follow it, fall back to greedy when broken.

const (
	routeTargetApples = 4  // desired apples per bot (dynamic cap)
	routeMaxSteps     = 40 // max steps per bot route (safety cap)
	routeReplayDepth  = 15 // max BFS depth in replayPath
)

// phasePartition is the main dispatcher.
func (d *Decision) phasePartition() {
	p := d.P

	if !p.Initialized {
		d.planAllRoutes()
		p.Initialized = true
	} else {
		d.validateRoutes()
		d.extendRoutes()
	}

	d.rebuildPlannedApples()
	d.preplanAdjacent()
	d.executeRoutes()
}

// --- Greedy fallback ---

// greedyFallback assigns the closest reachable apple not claimed by other routes.
func (d *Decision) greedyFallback(si, snIdx int) {
	g := d.G
	p := d.P
	sn := &g.Sn[snIdx]

	// Use SimBFS for ground-truth reachability (not surface graph).
	sim := NewSim(g)
	sim.RebuildAppleMap()
	targets := sim.SimBFSApples(sn)
	if len(targets) == 0 {
		d.AssignedDir[si] = fallbackDir(g, sn)
		return
	}

	// Pick closest not claimed by valid routes or other greedy bots.
	for _, t := range targets {
		if t.Apple >= 0 && t.Apple < len(p.PlannedApples) && p.PlannedApples[t.Apple] {
			continue
		}
		// Claim for deconfliction with subsequent greedy bots.
		if t.Apple >= 0 && t.Apple < len(p.PlannedApples) {
			p.PlannedApples[t.Apple] = true
		}
		d.Assigned[si] = t.Apple
		d.AssignedDir[si] = t.FirstDir
		return
	}

	// All claimed — pick absolute closest.
	d.Assigned[si] = targets[0].Apple
	d.AssignedDir[si] = targets[0].FirstDir
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
				// Reseed sim state from current real body.
				if route.Valid {
					route.SimBody = append(route.SimBody[:0], sn.Body[:sn.Len]...)
					route.SimDir = sn.Dir
					route.SimTurnNum = d.P.TurnCount
				}
			}
		} else {
			// Steps exhausted but apples remain — re-plan.
			d.replanRoute(si, snIdx, route, appleAlive)
			if route.Valid {
				route.SimBody = append(route.SimBody[:0], sn.Body[:sn.Len]...)
				route.SimDir = sn.Dir
				route.SimTurnNum = d.P.TurnCount
			}
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

// --- Adjacent grab (Phase 0) ---

// preplanAdjacent assigns adjacent apples instantly — no planning needed.
func (d *Decision) preplanAdjacent() {
	g := d.G
	sim := NewSim(g)
	sim.RebuildAppleMap()

	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}
		// Already has assignment from route.
		if d.Assigned[si] >= 0 {
			continue
		}
		head := sn.Body[0]
		if !g.IsInGrid(head) {
			continue
		}

		for dir := 0; dir < 4; dir++ {
			nc := g.Nbm[head][dir]
			if nc < 0 || !sim.isApple(nc) {
				continue
			}
			// Reject backward.
			if sn.Len > 1 && dir == Do[sn.Dir] {
				continue
			}
			// Simulate: move + copy + gravity.
			newBody, alive := sim.simulateMove(sn.Body[:sn.Len], dir)
			if !alive || len(newBody) < sn.Len {
				continue // dies or beheaded
			}
			bodycp := make([]int, len(newBody))
			copy(bodycp, newBody)
			sim.appleMap[nc] = false
			gravOk := sim.applyGravity(bodycp)
			sim.appleMap[nc] = true
			if !gravOk {
				continue
			}
			d.Assigned[si] = nc
			d.AssignedDir[si] = dir
			break
		}
	}
}

// extendRoutes extends valid routes that have fewer than routeTargetApples apples.
// Runs 1 Voronoi round from saved sim state, competing with enemies.
func (d *Decision) extendRoutes() {
	g := d.G
	p := d.P

	// Case C: bots with empty routes get a fresh re-plan.
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}
		route := &p.Routes[si]
		if !route.Valid && len(route.SimBody) > 0 {
			// Re-plan from current body. Uses SimBFS so only truly reachable targets.
			sim := NewSim(g)
			sim.RebuildAppleMap()
			sim.buildObstacleMap(sn.ID)
			targets := sim.SimBFSApples(sn)
			if len(targets) == 0 {
				continue
			}
			// Pick closest unclaimed.
			for _, t := range targets {
				if t.Apple >= 0 && t.Apple < len(p.PlannedApples) && p.PlannedApples[t.Apple] {
					continue
				}
				steps, finalBody := d.replayPath(sim, sn.Body[:sn.Len], sn.Dir,
					t.Apple, p.TurnCount, g.Ap[:g.ANum])
				if steps == nil {
					continue
				}
				route.Valid = true
				route.AppleSeq = append(route.AppleSeq[:0], t.Apple)
				route.Steps = append(route.Steps[:0], steps...)
				route.StepCursor = 0
				route.SimBody = append(route.SimBody[:0], finalBody...)
				if len(finalBody) >= 2 {
					route.SimDir = g.DirFromTo(finalBody[1], finalBody[0])
				}
				route.SimTurnNum = p.TurnCount + len(steps)
				break
			}
		}
	}

	// Case A: extend valid routes that have room.
	var extendable []int // mySlot indices
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}
		route := &p.Routes[si]
		if route.Valid && len(route.AppleSeq) < routeTargetApples && len(route.SimBody) > 0 {
			extendable = append(extendable, si)
		}
	}
	if len(extendable) == 0 {
		return
	}

	// Build avail: current apples minus all routes' AppleSeq minus EnemyClaimed.
	claimed := make([]bool, g.NCells)
	for si := range d.MySnakes {
		r := &p.Routes[si]
		if !r.Valid {
			continue
		}
		for _, ap := range r.AppleSeq {
			if ap >= 0 && ap < g.NCells {
				claimed[ap] = true
			}
		}
	}
	avail := make([]int, 0, g.ANum)
	for i := 0; i < g.ANum; i++ {
		ap := g.Ap[i]
		if !claimed[ap] && !(ap >= 0 && ap < len(p.EnemyClaimed) && p.EnemyClaimed[ap]) {
			avail = append(avail, ap)
		}
	}
	if len(avail) == 0 {
		return
	}

	sim := NewSim(g)
	sim.rebuildAppleMapFrom(avail)

	// Build candidates: our extendable bots + all alive enemies.
	type candidate struct {
		mySlot int // -1 for enemy
		snIdx  int
		apple  int
		dist   int
	}
	var candidates []candidate

	for _, si := range extendable {
		route := &p.Routes[si]
		snIdx := d.MySnakes[si]
		sn := &g.Sn[snIdx]
		simBodyCopy := append([]int(nil), route.SimBody...)
		tmpSn := &Snake{
			ID: sn.ID, Owner: sn.Owner,
			Body: simBodyCopy, Len: len(simBodyCopy),
			Dir: route.SimDir, Alive: true,
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
		candidates = append(candidates, candidate{
			mySlot: si, snIdx: snIdx, apple: best.Apple, dist: best.Dist,
		})
	}

	// Add enemy candidates from their current BFS.Reach.
	for _, snIdx := range d.OpSnakes {
		reach := d.BFS.Reach[snIdx]
		if len(reach) == 0 {
			continue
		}
		// Enemy's best unclaimed apple.
		for _, ri := range reach {
			if !claimed[ri.Apple] {
				candidates = append(candidates, candidate{
					mySlot: -1, snIdx: snIdx, apple: ri.Apple, dist: ri.Dist,
				})
				break
			}
		}
	}

	// Voronoi: closest wins.
	assigned := make(map[int]candidate) // apple → winner
	for _, c := range candidates {
		if prev, ok := assigned[c.apple]; !ok || c.dist < prev.dist {
			assigned[c.apple] = c
		}
	}

	// Process our winners only.
	for _, w := range assigned {
		if w.mySlot < 0 {
			continue // enemy won, skip
		}
		si := w.mySlot
		route := &p.Routes[si]
		sn := &g.Sn[w.snIdx]

		sim.buildObstacleMap(sn.ID)
		steps, finalBody := d.replayPath(sim, route.SimBody, route.SimDir,
			w.apple, route.SimTurnNum, avail)
		if steps == nil {
			continue
		}

		route.AppleSeq = append(route.AppleSeq, w.apple)
		route.Steps = append(route.Steps, steps...)
		route.SimBody = append(route.SimBody[:0], finalBody...)
		if len(finalBody) >= 2 {
			route.SimDir = g.DirFromTo(finalBody[1], finalBody[0])
		}
		route.SimTurnNum += len(steps)
	}
}

// --- Route planning (turn 1) ---

// planAllRoutes assigns apples using unified Voronoi with enemy bots.
// All 8 snakes (4 my + 4 enemy) compete. Enemies win apples but don't get routes.
func (d *Decision) planAllRoutes() {
	g := d.G
	p := d.P
	myN := len(d.MySnakes)
	if myN == 0 {
		return
	}

	// Build full apple pool (no enemyWillEat hack — Voronoi handles it).
	avail := make([]int, 0, g.ANum)
	for i := 0; i < g.ANum; i++ {
		avail = append(avail, g.Ap[i])
	}
	sim := NewSim(g)

	// Reset enemy claimed.
	for i := range p.EnemyClaimed {
		p.EnemyClaimed[i] = false
	}

	// All snakes participate: my (slots 0..myN-1) then enemy.
	// allSnakes[i] = g.Sn index. isMy[i] = true for our bots.
	type botState struct {
		body    []int
		dir     int
		turnNum int
		alive   bool
		snIdx   int
		isMy    bool
		mySlot  int // index into d.MySnakes (-1 for enemy)
	}

	var allBots []botState
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		allBots = append(allBots, botState{
			body:    append([]int(nil), sn.Body[:sn.Len]...),
			dir:     sn.Dir,
			turnNum: p.TurnCount,
			alive:   sn.Alive && sn.Len > 0,
			snIdx:   snIdx,
			isMy:    true,
			mySlot:  si,
		})
	}
	for _, snIdx := range d.OpSnakes {
		sn := &g.Sn[snIdx]
		allBots = append(allBots, botState{
			body:   append([]int(nil), sn.Body[:sn.Len]...),
			dir:    sn.Dir,
			alive:  sn.Alive && sn.Len > 0,
			snIdx:  snIdx,
			isMy:   false,
			mySlot: -1,
		})
	}

	// Init routes for our bots.
	for si, snIdx := range d.MySnakes {
		r := &p.Routes[si]
		r.SnIdx = snIdx
		r.Valid = true
		r.StepCursor = 0
		r.AppleSeq = r.AppleSeq[:0]
		r.Steps = r.Steps[:0]
	}

	// Iterative Voronoi: our bots compete, keep going until all have enough or stuck.
	// Dynamic target: each bot aims for routeTargetApples.
	maxRounds := g.ANum // safety cap: can't plan more apples than exist
	for round := 0; round < maxRounds; round++ {
		if len(avail) == 0 {
			break
		}
		sim.rebuildAppleMapFrom(avail)

		type candidate struct {
			botIdx int // index into allBots
			apple  int
			dist   int
			target SimTarget
		}
		var candidates []candidate

		for bi := range allBots {
			bot := &allBots[bi]
			if !bot.alive || !bot.isMy {
				continue
			}
			if len(p.Routes[bot.mySlot].AppleSeq) >= routeTargetApples {
				continue
			}

			sn := &g.Sn[bot.snIdx]
			tmpSn := &Snake{
				ID:    sn.ID,
				Owner: sn.Owner,
				Body:  bot.body,
				Len:   len(bot.body),
				Dir:   bot.dir,
				Alive: true,
			}

			targets := sim.SimBFSApples(tmpSn)
			if len(targets) == 0 {
				continue
			}

			// Score: dist * 10 + heat bias for our bots.
			bestScore := 1 << 30
			bestIdx := 0
			for i, t := range targets {
				score := t.Dist * 10
				// Our bots get heat bias: prefer apples where we have advantage.
				if bot.isMy && t.Apple >= 0 && t.Apple < MaxExpandedCells {
					h := d.HeatByCell[t.Apple]
					if h != heatUnreachable && h != heatExclusive {
						if h < 0 {
							score += (-h) * 5 // penalty: enemy closer
						} else if h > 0 {
							score -= h * 5 // bonus: we're closer
						}
					}
				}
				if score < bestScore {
					bestScore = score
					bestIdx = i
				}
			}
			best := targets[bestIdx]
			candidates = append(candidates, candidate{
				botIdx: bi, apple: best.Apple, dist: best.Dist, target: best,
			})
		}

		if len(candidates) == 0 {
			break
		}

		// Voronoi: closest bot wins each apple.
		assigned := make(map[int]candidate) // apple → winning candidate
		for _, c := range candidates {
			if prev, ok := assigned[c.apple]; !ok || c.dist < prev.dist {
				assigned[c.apple] = c
			}
		}

		winners := make(map[int]bool) // botIdx → won this round
		for _, winner := range assigned {
			bi := winner.botIdx
			if winners[bi] {
				continue
			}
			winners[bi] = true
			bot := &allBots[bi]

			sn := &g.Sn[bot.snIdx]
			sim.buildObstacleMap(sn.ID)
			steps, finalBody := d.replayPath(sim, bot.body, bot.dir,
				winner.apple, bot.turnNum, avail)
			if steps == nil {
				continue
			}
			route := &p.Routes[bot.mySlot]
			route.AppleSeq = append(route.AppleSeq, winner.apple)
			route.Steps = append(route.Steps, steps...)
			bot.turnNum += len(steps)
			bot.body = finalBody
			if len(finalBody) >= 2 {
				bot.dir = g.DirFromTo(finalBody[1], finalBody[0])
			}

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
			break
		}
	}

	// Greedy fill: under-planned bots get more apples without enemy competition.
	for fill := 0; fill < routeTargetApples; fill++ {
		if len(avail) == 0 {
			break
		}
		progress := false
		for bi := range allBots {
			bot := &allBots[bi]
			if !bot.isMy || !bot.alive {
				continue
			}
			route := &p.Routes[bot.mySlot]
			if len(route.AppleSeq) >= routeTargetApples {
				continue
			}

			sim.rebuildAppleMapFrom(avail)
			sn := &g.Sn[bot.snIdx]
			sim.buildObstacleMap(sn.ID)

			tmpSn := &Snake{
				ID: sn.ID, Owner: sn.Owner,
				Body: bot.body, Len: len(bot.body),
				Dir: bot.dir, Alive: true,
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

			steps, finalBody := d.replayPath(sim, bot.body, bot.dir,
				best.Apple, bot.turnNum, avail)
			if steps == nil {
				continue
			}

			route.AppleSeq = append(route.AppleSeq, best.Apple)
			route.Steps = append(route.Steps, steps...)
			bot.turnNum += len(steps)
			bot.body = finalBody
			if len(finalBody) >= 2 {
				bot.dir = g.DirFromTo(finalBody[1], finalBody[0])
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

	// Save sim state for rolling horizon. Init routeless bots.
	for si, snIdx := range d.MySnakes {
		route := &p.Routes[si]
		if len(route.AppleSeq) == 0 {
			route.Valid = false
			// Init sim state from real body for future extension.
			sn := &g.Sn[snIdx]
			route.SimBody = append(route.SimBody[:0], sn.Body[:sn.Len]...)
			route.SimDir = sn.Dir
			route.SimTurnNum = p.TurnCount
		} else {
			// Find the bot in allBots to get final sim state.
			for bi := range allBots {
				if allBots[bi].isMy && allBots[bi].mySlot == si {
					route.SimBody = append(route.SimBody[:0], allBots[bi].body...)
					route.SimDir = allBots[bi].dir
					route.SimTurnNum = allBots[bi].turnNum
					break
				}
			}
		}
	}
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

// findSp returns the index of the nearest supported segment from head.
func findSp(g *Game, body []int) int {
	for i, c := range body {
		if c < 0 || c >= g.NCells {
			continue
		}
		below := c + g.Stride
		if below >= g.NCells || !g.Cell[below] {
			return i // cell below is wall or bottom edge — supported
		}
	}
	return -1
}

// isSurfaceTrapped checks if a body resting on a surface is too long to escape.
// Uses body[sp] (support segment) to look up the surface via SurfAt, then checks
// if all outgoing links are shorter than body length — body blocks every exit.
func isSurfaceTrapped(g *Game, body []int, sp int) bool {
	if len(body) == 0 || g.SurfAt == nil || sp < 0 || sp >= len(body) {
		return false
	}

	cell := body[sp]
	if cell < 0 || cell >= g.NCells {
		return false
	}
	sid := g.SurfAt[cell]
	if sid < 0 || sid >= len(g.Surfs) {
		return false
	}

	s := &g.Surfs[sid]
	bodyLen := len(body)
	if len(s.Links) == 0 {
		return true
	}
	for _, link := range s.Links {
		if link.Len >= bodyLen {
			return false
		}
	}
	return true
}

// nextRouteApple returns the first apple in the route's remaining sequence.
func nextRouteApple(route *BotRoute) int {
	if len(route.AppleSeq) > 0 {
		return route.AppleSeq[0]
	}
	return -1
}
