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
	Apples     []ReachInfo // Layer 2: reachable apples from best landing surface
	TotalFirst int         // direction of first move
	BestApple  int         // best apple cell (-1 if none)
	BestDist   int         // combined dist to best apple
}

// BFSResult holds reach data computed by phaseBFS.
// Arrays indexed by snake slot (0..SNum-1), not by MySnakes/OpSnakes slot.
type BFSResult struct {
	Reach   [MaxASn][]ReachInfo // per snake, all reachable apples sorted by Dist
	SurfBFS [MaxASn][]SurfReach // body-sim paths to surfaces
	Plan    [MaxASn]SnakePlan   // combined layered plans
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
		d.BFS.Plan[i] = SnakePlan{BestApple: -1}
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
