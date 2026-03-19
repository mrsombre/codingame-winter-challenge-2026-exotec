package main

import "sort"

const maxAppleSurfaceHopDist = 7

// ReachInfo represents one snake's ability to reach one apple via surface graph.
type ReachInfo struct {
	Apple    int // apple cell index
	Dist     int // accumulated link.Len along surface path
	FirstDir int // direction of first step from head (neck-validated for friendly)
	EndSurf  int // surface ID the apple sits on
}

// SnakePlan is the combined Layer 1 (body-sim) + Layer 2 (surface-graph) result.
type SnakePlan struct {
	Apples       []ReachInfo // Layer 2: reachable apples from best landing surface
	TotalFirst   int         // direction of first move
	BestApple    int         // best apple cell (-1 if none)
	BestDist     int         // combined dist to best apple
	Conflicting  bool        // true if head path overlaps another snake
	ConflictWith int         // index of conflicting snake (-1 if none)
}

// BFSResult holds reach data computed by phaseBFS.
// Arrays indexed by snake slot (0..SNum-1), not by MySnakes/OpSnakes slot.
type BFSResult struct {
	Reach   [MaxASn][]ReachInfo // per snake, all reachable apples sorted by Dist
	SurfBFS [MaxASn][]SurfReach // body-sim paths to surfaces
	Plan    [MaxASn]SnakePlan   // combined layered plans
}

// surfaceReach computes reachable apples for a snake via the surface link graph.
// If neckCheck is true, blocks directions that would move the head into body[1].
// Returns []ReachInfo sorted by Dist ascending.
func surfaceReach(g *Game, sn *Snake, neckCheck bool) []ReachInfo {
	if sn == nil || len(sn.Body) == 0 {
		return nil
	}
	head := sn.Body[0]
	if head < 0 || head >= g.NCells || !g.IsInGrid(head) {
		return nil
	}
	sid := g.SurfAt[head]
	if sid < 0 || g.Surfs[sid].Type == SurfNone {
		return nil
	}

	surf := &g.Surfs[sid]
	hx, _ := g.XY(head)

	// Determine blocked direction (neck).
	blockedDir := -1
	if neckCheck && sn.Len > 1 {
		neck := sn.Body[1]
		for d := 0; d < 4; d++ {
			if g.Nbm[head][d] == neck {
				blockedDir = d
				break
			}
		}
	}

	// Intra-surface cost to edges.
	costToLeft := hx - surf.Left
	costToRight := surf.Right - hx

	// Maneuverability penalty on starting surface:
	// if surface is tiny and the blocked direction is toward an edge, penalize.
	if surf.Len < 4 {
		if costToRight > 0 && blockedDir == DR {
			costToRight += 2
		}
		if costToLeft > 0 && blockedDir == DL {
			costToLeft += 2
		}
	}

	leftBlocked := blockedDir == DL
	rightBlocked := blockedDir == DR

	appleAlive := make(map[int]bool, g.ANum)
	for i := 0; i < g.ANum; i++ {
		appleAlive[g.Ap[i]] = true
	}

	type dijkNode struct {
		surf     int
		dist     int
		firstDir int
	}

	bestDist := make(map[int]int) // surfID -> best dist
	bestApple := make(map[int]ReachInfo)

	var pq []dijkNode
	pqPush := func(n dijkNode) {
		pq = append(pq, n)
		for i := len(pq) - 1; i > 0 && pq[i].dist < pq[i-1].dist; i-- {
			pq[i], pq[i-1] = pq[i-1], pq[i]
		}
	}
	pqPop := func() dijkNode {
		n := pq[0]
		pq = pq[1:]
		return n
	}

	addApple := func(ri ReachInfo) {
		if prev, ok := bestApple[ri.Apple]; ok && prev.Dist <= ri.Dist {
			return
		}
		bestApple[ri.Apple] = ri
	}

	startAppleReach := func(link AppleLink) (ReachInfo, bool) {
		if !appleAlive[link.Apple] {
			return ReachInfo{}, false
		}
		if !canEatAppleLink(sn.Len, link) {
			return ReachInfo{}, false
		}

		sx, _ := g.XY(link.Start)
		moveX := sx - hx
		dist := link.Len
		firstDir := -1

		if moveX > 0 {
			if rightBlocked {
				return ReachInfo{}, false
			}
			dist += moveX
			firstDir = DR
		} else if moveX < 0 {
			if leftBlocked {
				return ReachInfo{}, false
			}
			dist -= moveX
			firstDir = DL
		} else {
			if len(link.Path) < 2 {
				return ReachInfo{}, false
			}
			dx, dy := dirBetween(g, link.Path[0], link.Path[1])
			firstDir = dirIndex(dx, dy)
			if firstDir == blockedDir {
				return ReachInfo{}, false
			}
		}

		return ReachInfo{
			Apple:    link.Apple,
			Dist:     dist,
			FirstDir: firstDir,
			EndSurf:  g.SurfAt[link.Apple],
		}, true
	}

	addAppleLinks := func(surfaceID, baseDist, firstDir int) {
		for _, link := range g.Surfs[surfaceID].Apples {
			if !appleAlive[link.Apple] {
				continue
			}
			if !canEatAppleLink(sn.Len, link) {
				continue
			}
			addApple(ReachInfo{
				Apple:    link.Apple,
				Dist:     baseDist + link.Len,
				FirstDir: firstDir,
				EndSurf:  g.SurfAt[link.Apple],
			})
		}
	}

	// Check apples from the starting surface with exact head-to-start cost.
	for _, link := range surf.Apples {
		if ri, ok := startAppleReach(link); ok {
			addApple(ri)
		}
	}

	// Seed Dijkstra from starting surface links.
	bestDist[sid] = 0
	for _, link := range surf.Links {
		if g.Surfs[link.To].Type == SurfNone {
			continue
		}
		if !canTraverseSurfaceLink(g, sn.Len, link) {
			continue
		}
		// link.Path[0] is always surf.Left or surf.Right (edge cell).
		p0x, _ := g.XY(link.Path[0])
		var edgeCost int
		var dir int
		if p0x == surf.Left {
			edgeCost = costToLeft
			dir = DL
			if leftBlocked {
				continue
			}
		} else {
			edgeCost = costToRight
			dir = DR
			if rightBlocked {
				continue
			}
		}

		// Maneuverability penalty when entering a small target surface.
		enterPenalty := 0
		if g.Surfs[link.To].Len < 4 {
			enterPenalty = 2
		}

		totalDist := edgeCost + link.Len + enterPenalty
		if totalDist > maxAppleSurfaceHopDist {
			continue
		}
		if prev, ok := bestDist[link.To]; ok && totalDist >= prev {
			continue
		}
		bestDist[link.To] = totalDist
		pqPush(dijkNode{surf: link.To, dist: totalDist, firstDir: dir})
	}

	// Process queue.
	for len(pq) > 0 {
		cur := pqPop()
		if cur.dist > maxAppleSurfaceHopDist {
			break
		}
		if prev, ok := bestDist[cur.surf]; ok && cur.dist > prev {
			continue // stale entry
		}

		addAppleLinks(cur.surf, cur.dist, cur.firstDir)

		// Expand links from this surface.
		s := &g.Surfs[cur.surf]
		for _, link := range s.Links {
			if g.Surfs[link.To].Type == SurfNone {
				continue
			}
			if !canTraverseSurfaceLink(g, sn.Len, link) {
				continue
			}
			enterPenalty := 0
			if g.Surfs[link.To].Len < 4 {
				enterPenalty = 2
			}
			newDist := cur.dist + link.Len + enterPenalty
			if newDist > maxAppleSurfaceHopDist {
				continue
			}
			if prev, ok := bestDist[link.To]; ok && newDist >= prev {
				continue
			}
			bestDist[link.To] = newDist
			pqPush(dijkNode{surf: link.To, dist: newDist, firstDir: cur.firstDir})
		}
	}

	result := make([]ReachInfo, 0, len(bestApple))
	for _, ri := range bestApple {
		result = append(result, ri)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Dist < result[j].Dist
	})

	return result
}

func canEatAppleLink(snLen int, link AppleLink) bool {
	return snLen >= link.Len
}

func canTraverseSurfaceLink(g *Game, snLen int, link SurfLink) bool {
	if isStraightFallLink(g, link) {
		return true
	}
	return snLen >= link.Len
}

func isStraightFallLink(g *Game, link SurfLink) bool {
	if len(link.Path) < 3 {
		return false
	}

	x0, y0 := g.XY(link.Path[0])
	x1, y1 := g.XY(link.Path[1])
	if y1 != y0 || (x1 != x0-1 && x1 != x0+1) {
		return false
	}

	for i := 2; i < len(link.Path); i++ {
		px, py := g.XY(link.Path[i-1])
		cx, cy := g.XY(link.Path[i])
		if cx != px || cy != py+1 {
			return false
		}
	}
	return true
}

func dirBetween(g *Game, from, to int) (int, int) {
	fx, fy := g.XY(from)
	tx, ty := g.XY(to)
	return tx - fx, ty - fy
}

func dirIndex(dx, dy int) int {
	switch {
	case dx == 0 && dy == -1:
		return DU
	case dx == 1 && dy == 0:
		return DR
	case dx == 0 && dy == 1:
		return DD
	case dx == -1 && dy == 0:
		return DL
	default:
		return -1
	}
}

// headsOverlap checks if two head traces occupy the same cell at the same turn.
func headsOverlap(a, b []int) bool {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for t := 0; t < n; t++ {
		if a[t] == b[t] {
			return true
		}
	}
	return false
}

const maxSurfReachDist = 999 // no cap — find all apples reachable via surface graph

// surfGraphReach performs pass 2: Dijkstra over surface links from multiple
// entry points (SurfBFS results). Finds closest apples with time-aware
// invalidation — eating an apple at time T removes its SurfApple surface
// for subsequent traversals. headCell is used to derive firstDir for
// on-surface starts (entry.Dist==0, entry.FirstDir==-1).
// Total distance (entry.Dist + graph hops) is capped at maxSurfReachDist.
func surfGraphReach(g *Game, entries []SurfReach, snLen int, headCell int, snakeDir ...int) []ReachInfo {
	// Optional snake facing direction to prevent backward firstDir
	backwardDir := -1
	if len(snakeDir) > 0 && snakeDir[0] >= 0 {
		backwardDir = Do[snakeDir[0]]
	}
	type dijkNode struct {
		surf     int
		dist     int
		firstDir int
		landing  int // cell where we arrived on this surface
	}

	bestDist := make(map[int]int) // surfID -> best dist
	bestApple := make(map[int]ReachInfo)

	// alive apples
	appleAlive := make(map[int]bool, g.ANum)
	for i := 0; i < g.ANum; i++ {
		appleAlive[g.Ap[i]] = true
	}

	// track eaten apples -> invalidated surfaces
	eatenAppleSurf := make(map[int]bool) // surfID of apple surfaces whose apple was eaten

	var pq []dijkNode
	pqPush := func(n dijkNode) {
		pq = append(pq, n)
		for i := len(pq) - 1; i > 0 && pq[i].dist < pq[i-1].dist; i-- {
			pq[i], pq[i-1] = pq[i-1], pq[i]
		}
	}

	addApple := func(ri ReachInfo) {
		if prev, ok := bestApple[ri.Apple]; ok && prev.Dist <= ri.Dist {
			return
		}
		bestApple[ri.Apple] = ri

		// if this apple backs a SurfApple surface, mark it eaten
		ax, ay := g.XY(ri.Apple)
		above := g.Idx(ax, ay-1)
		if above >= 0 && above < g.NCells {
			sid := g.SurfAt[above]
			if sid >= 0 && g.Surfs[sid].Type == SurfApple {
				eatenAppleSurf[sid] = true
			}
		}
	}

	// dirForTarget derives first-move direction from head to a target cell.
	// Used when entry.FirstDir == -1 (on-surface start).
	dirForTarget := func(entry SurfReach, targetCell int) int {
		if entry.FirstDir >= 0 {
			return entry.FirstDir
		}
		// Pick neighbor cell closest to target (excluding backward)
		bestDir := -1
		bestDist := 1 << 30
		for d := 0; d < 4; d++ {
			if d == backwardDir {
				continue
			}
			nb := g.Nbm[headCell][d]
			if nb < 0 {
				continue
			}
			dist := g.Manhattan(nb, targetCell)
			if dist < bestDist {
				bestDist = dist
				bestDir = d
			}
		}
		if bestDir >= 0 {
			return bestDir
		}
		return DU // fallback
	}

	// Seed from all entry points
	// Each entry lands at a specific cell on a surface. We need the
	// intra-surface walk cost from landing to the apple-link start or
	// to the surface edges where outgoing links originate.
	for _, entry := range entries {
		sid := entry.SurfID
		s := &g.Surfs[sid]
		if s.Type == SurfNone {
			continue
		}

		lx, _ := g.XY(entry.Landing)
		costToLeft := lx - s.Left
		costToRight := s.Right - lx

		// For Dijkstra expansion, the effective dist at this surface
		// must include the walk to the edge where each link starts.
		// We don't seed a single bestDist — instead we directly seed
		// outgoing links with proper edge costs.

		// check apples on the entry surface itself
		for _, al := range s.Apples {
			sx, _ := g.XY(al.Start)
			walkCost := sx - lx
			if walkCost < 0 {
				walkCost = -walkCost
			}
			totalDist := entry.Dist + walkCost + al.Len
			if totalDist > maxSurfReachDist {
				continue
			}
			if !appleAlive[al.Apple] {
				continue
			}
			fd := dirForTarget(entry, al.Apple)
			addApple(ReachInfo{
				Apple:    al.Apple,
				Dist:     totalDist,
				FirstDir: fd,
				EndSurf:  g.SurfAt[al.Apple],
			})
		}

		// seed outgoing surface links with edge walk cost
		for _, link := range s.Links {
			ts := &g.Surfs[link.To]
			if ts.Type == SurfNone {
				continue
			}
			if !canTraverseSurfaceLink(g, snLen, link) {
				continue
			}
			// link.Path[0] is the edge cell (Left or Right of this surface)
			p0x, _ := g.XY(link.Path[0])
			var edgeCost int
			if p0x == s.Left {
				edgeCost = costToLeft
			} else {
				edgeCost = costToRight
			}
			newDist := entry.Dist + edgeCost + link.Len
			if newDist > maxSurfReachDist {
				continue
			}
			if prev, ok := bestDist[link.To]; ok && newDist >= prev {
				continue
			}
			bestDist[link.To] = newDist
			fd := entry.FirstDir
			if fd < 0 {
				if p0x <= lx {
					fd = DL
				} else {
					fd = DR
				}
			}
			pqPush(dijkNode{surf: link.To, dist: newDist, firstDir: fd, landing: link.Landing})
		}
	}

	// Dijkstra over surface links
	for len(pq) > 0 {
		cur := pq[0]
		pq = pq[1:]

		if cur.dist > maxSurfReachDist {
			break
		}
		if prev, ok := bestDist[cur.surf]; ok && cur.dist > prev {
			continue
		}

		s := &g.Surfs[cur.surf]
		clx, _ := g.XY(cur.landing)
		curToLeft := clx - s.Left
		curToRight := s.Right - clx

		// check apples on this surface (with walk cost from landing)
		for _, al := range s.Apples {
			sx, _ := g.XY(al.Start)
			walkCost := sx - clx
			if walkCost < 0 {
				walkCost = -walkCost
			}
			appleDist := cur.dist + walkCost + al.Len
			if appleDist > maxSurfReachDist {
				continue
			}
			if !appleAlive[al.Apple] {
				continue
			}
			fd := cur.firstDir
			addApple(ReachInfo{
				Apple:    al.Apple,
				Dist:     appleDist,
				FirstDir: fd,
				EndSurf:  g.SurfAt[al.Apple],
			})
		}

		// expand outgoing links (with walk cost from landing to edge)
		for _, link := range s.Links {
			ts := &g.Surfs[link.To]
			if ts.Type == SurfNone {
				continue
			}
			if eatenAppleSurf[link.To] {
				continue
			}
			if !canTraverseSurfaceLink(g, snLen, link) {
				continue
			}
			p0x, _ := g.XY(link.Path[0])
			var edgeCost int
			if p0x == s.Left {
				edgeCost = curToLeft
			} else {
				edgeCost = curToRight
			}
			newDist := cur.dist + edgeCost + link.Len
			if newDist > maxSurfReachDist {
				continue
			}
			if prev, ok := bestDist[link.To]; ok && newDist >= prev {
				continue
			}
			bestDist[link.To] = newDist
			fd := cur.firstDir
			if fd < 0 {
				if p0x <= clx {
					fd = DL
				} else {
					fd = DR
				}
			}
			pqPush(dijkNode{surf: link.To, dist: newDist, firstDir: fd, landing: link.Landing})
		}
	}

	result := make([]ReachInfo, 0, len(bestApple))
	for _, ri := range bestApple {
		result = append(result, ri)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Dist < result[j].Dist
	})
	return result
}

// mergeSimApples runs body-simulation BFS for nearby apples and merges
// results into reach list. SimBFS distances replace surface graph estimates
// where shorter; new apples are added.
func mergeSimApples(g *Game, sim *Sim, sn *Snake, reach []ReachInfo) []ReachInfo {
	head := sn.Body[0]

	// Quick check: any apples within Manhattan simAppleMaxDepth?
	hasNearby := false
	for i := 0; i < g.ANum; i++ {
		if g.Manhattan(head, g.Ap[i]) <= simAppleMaxDepth {
			hasNearby = true
			break
		}
	}
	if !hasNearby {
		return reach
	}

	targets := sim.SimBFSApples(sn)
	if len(targets) == 0 {
		return reach
	}

	// Build lookup of existing reach by apple cell
	byApple := make(map[int]int, len(reach))
	for i, ri := range reach {
		byApple[ri.Apple] = i
	}

	for _, t := range targets {
		if idx, ok := byApple[t.Apple]; ok {
			if t.Dist < reach[idx].Dist {
				reach[idx].Dist = t.Dist
				reach[idx].FirstDir = t.FirstDir
			}
		} else {
			reach = append(reach, ReachInfo{
				Apple:    t.Apple,
				Dist:     t.Dist,
				FirstDir: t.FirstDir,
				EndSurf:  g.SurfAt[t.Apple],
			})
			byApple[t.Apple] = len(reach) - 1
		}
	}

	sort.Slice(reach, func(i, j int) bool {
		return reach[i].Dist < reach[j].Dist
	})
	return reach
}

func (d *Decision) phaseBFS() {
	g := d.G

	// 1. Partition alive snakes
	d.MySnakes = d.MySnakes[:0]
	d.OpSnakes = d.OpSnakes[:0]
	for i := 0; i < g.SNum; i++ {
		if !g.Sn[i].Alive || g.Sn[i].Len == 0 {
			continue
		}
		if g.Sn[i].Owner == 0 {
			d.MySnakes = append(d.MySnakes, i)
		} else {
			d.OpSnakes = append(d.OpSnakes, i)
		}
	}

	for i := range d.BFS.Reach {
		d.BFS.Reach[i] = nil
		d.BFS.SurfBFS[i] = nil
		d.BFS.Plan[i] = SnakePlan{BestApple: -1, ConflictWith: -1}
	}

	if cap(d.Assigned) < len(d.MySnakes) {
		d.Assigned = make([]int, len(d.MySnakes))
		d.AssignedDir = make([]int, len(d.MySnakes))
	} else {
		d.Assigned = d.Assigned[:len(d.MySnakes)]
		d.AssignedDir = d.AssignedDir[:len(d.MySnakes)]
	}

	sim := NewSim(g)
	sim.RebuildAppleMap()

	// Compute reach for ALL snakes (my + enemy)
	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive || sn.Len == 0 {
			continue
		}

		plan := &d.BFS.Plan[i]
		plan.BestApple = -1
		plan.ConflictWith = -1

		head := sn.Body[0]
		onSurface := g.IsInGrid(head) && g.SurfAt[head] >= 0 && sn.Sp == 0

		if onSurface {
			sid := g.SurfAt[head]
			d.BFS.SurfBFS[i] = []SurfReach{{
				SurfID: sid, Dist: 0, FirstDir: -1, Landing: head,
			}}
			plan.Apples = surfGraphReach(g, d.BFS.SurfBFS[i], sn.Len, sn.Body[0], sn.Dir)
			if len(plan.Apples) > 0 {
				plan.TotalFirst = plan.Apples[0].FirstDir
				plan.BestApple = plan.Apples[0].Apple
				plan.BestDist = plan.Apples[0].Dist
			}
		} else {
			d.BFS.SurfBFS[i] = sim.SurfBFS(sn)
			plan.Apples = surfGraphReach(g, d.BFS.SurfBFS[i], sn.Len, sn.Body[0], sn.Dir)
			if len(d.BFS.SurfBFS[i]) > 0 {
				plan.TotalFirst = d.BFS.SurfBFS[i][0].FirstDir
			}
			if len(plan.Apples) > 0 {
				plan.BestApple = plan.Apples[0].Apple
				plan.BestDist = plan.Apples[0].Dist
			}
		}

		plan.Apples = mergeSimApples(g, sim, sn, plan.Apples)
		if len(plan.Apples) > 0 {
			plan.BestApple = plan.Apples[0].Apple
			plan.BestDist = plan.Apples[0].Dist
			plan.TotalFirst = plan.Apples[0].FirstDir
		}

		d.BFS.Reach[i] = plan.Apples
	}

	// Init assignment slots for my snakes
	for i, snIdx := range d.MySnakes {
		d.Assigned[i] = -1
		d.AssignedDir[i] = fallbackDir(g, &g.Sn[snIdx])
	}

	// Conflict detection between my snakes
	for i := 0; i < len(d.MySnakes); i++ {
		si := d.MySnakes[i]
		if len(d.BFS.SurfBFS[si]) == 0 {
			continue
		}
		for j := i + 1; j < len(d.MySnakes); j++ {
			sj := d.MySnakes[j]
			if len(d.BFS.SurfBFS[sj]) == 0 {
				continue
			}
			if headsOverlap(d.BFS.SurfBFS[si][0].Heads, d.BFS.SurfBFS[sj][0].Heads) {
				d.BFS.Plan[si].Conflicting = true
				d.BFS.Plan[si].ConflictWith = j
				d.BFS.Plan[sj].Conflicting = true
				d.BFS.Plan[sj].ConflictWith = i
			}
		}
	}
}

func fallbackDir(g *Game, sn *Snake) int {
	if sn == nil || sn.Len == 0 || len(sn.Body) == 0 {
		return DU
	}
	head := sn.Body[0]
	if head < 0 || head >= g.NCells {
		return DU
	}
	neck := -1
	if sn.Len > 1 {
		neck = sn.Body[1]
	}
	for dir := 0; dir < 4; dir++ {
		nb := g.Nbm[head][dir]
		if nb >= 0 && nb != neck {
			return dir
		}
	}
	return DU
}
