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

// BFSResult holds reach data computed by phaseBFS.
type BFSResult struct {
	MyReach [MaxPSn][]ReachInfo // per friendly snake, sorted by Dist ascending
	OpReach [MaxPSn][]ReachInfo // per enemy snake, sorted by Dist ascending
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

func (d *Decision) phaseBFS() {
	g := d.G

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

	for i := range d.BFS.MyReach {
		d.BFS.MyReach[i] = nil
		d.BFS.OpReach[i] = nil
	}

	if cap(d.Assigned) < len(d.MySnakes) {
		d.Assigned = make([]int, len(d.MySnakes))
		d.AssignedDir = make([]int, len(d.MySnakes))
	} else {
		d.Assigned = d.Assigned[:len(d.MySnakes)]
		d.AssignedDir = d.AssignedDir[:len(d.MySnakes)]
	}

	for i, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		d.BFS.MyReach[i] = surfaceReach(g, sn, true)
		d.Assigned[i] = -1
		d.AssignedDir[i] = fallbackDir(g, sn)
		if len(d.BFS.MyReach[i]) > 0 {
			d.Assigned[i] = d.BFS.MyReach[i][0].Apple
			d.AssignedDir[i] = d.BFS.MyReach[i][0].FirstDir
		}
	}

	for i, snIdx := range d.OpSnakes {
		d.BFS.OpReach[i] = surfaceReach(g, &g.Sn[snIdx], false)
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
