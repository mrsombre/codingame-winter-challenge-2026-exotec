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
	head := sn.Body[0]
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
}
