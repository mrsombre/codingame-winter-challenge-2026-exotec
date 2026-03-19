package main

import "sort"


const (
	// Constellation scoring weights.
	// Distance penalizes once (closest apple), not per apple.
	// When distance is similar, bigger + less contested clusters win.
	scorePerReachable  = 30  // per apple reachable via sim/surface graph
	scoreClosestPen    = 40  // per turn to closest apple (single penalty)
	scoreHeatScale     = 15  // per turn of heat advantage per apple
	scoreHeatMax       = 50  // cap heat bonus per apple
	scoreSizeBonus     = 20  // per apple in constellation
	scoreUnreachPenMax = 500 // heavy penalty if no apples reachable
)

// constScore scores a constellation for a snake.
// Distance penalizes once (closest apple). Size and heat matter when distance is similar.
func (d *Decision) constScore(snIdx int, cl *Constellation) int {
	g := d.G
	reach := d.BFS.Reach[snIdx]

	inCluster := make(map[int]bool, cl.Size)
	for _, ap := range cl.Apples {
		inCluster[ap] = true
	}

	// Base: cluster size
	score := cl.Size * scoreSizeBonus

	// Find reachable apples and closest distance
	found := 0
	bestDist := 999
	totalHeat := 0
	for _, ri := range reach {
		if !inCluster[ri.Apple] {
			continue
		}
		found++
		if ri.Dist < bestDist {
			bestDist = ri.Dist
		}
		// Accumulate heat
		if ri.Apple >= 0 && ri.Apple < MaxExpandedCells {
			heat := d.HeatByCell[ri.Apple]
			bonus := heat * scoreHeatScale
			if bonus > scoreHeatMax {
				bonus = scoreHeatMax
			} else if bonus < -scoreHeatMax {
				bonus = -scoreHeatMax
			}
			totalHeat += bonus
		}
	}

	if found > 0 {
		score += found * scorePerReachable // more reachable = better
		score -= bestDist * scoreClosestPen // single distance penalty
		score += totalHeat                  // contestation
	} else {
		// Can't reach any apple — heavy penalty + Manhattan distance
		head := g.Sn[snIdx].Body[0]
		minDist := 999
		for _, ap := range cl.Apples {
			md := g.Manhattan(head, ap)
			if md < minDist {
				minDist = md
			}
		}
		score -= scoreUnreachPenMax + minDist*scoreClosestPen
	}

	return score
}

// constBestApple finds the closest reachable apple in a constellation.
// Sticky: if previous target is still alive and reachable in this cluster, keep it.
// Returns (apple cell, firstDir, found).
func (d *Decision) constBestApple(snIdx int, cl *Constellation, prevTarget int) (int, int, bool) {
	reach := d.BFS.Reach[snIdx]

	inCluster := make(map[int]bool, cl.Size)
	for _, ap := range cl.Apples {
		inCluster[ap] = true
	}

	// Sticky: if previous target is still in this cluster and reachable, keep it
	if prevTarget >= 0 && inCluster[prevTarget] {
		for _, ri := range reach {
			if ri.Apple == prevTarget {
				return ri.Apple, ri.FirstDir, true
			}
		}
	}

	// Otherwise: closest cluster apple
	for _, ri := range reach {
		if !inCluster[ri.Apple] {
			continue
		}
		return ri.Apple, ri.FirstDir, true
	}
	return -1, -1, false
}

// constNavigate finds the direction to navigate toward a constellation
// when no apple is directly reachable. Uses surface graph: find the SurfBFS
// entry whose surface is closest (via surface links) to any surface holding
// a constellation apple.
func (d *Decision) constNavigate(snIdx int, cl *Constellation) int {
	g := d.G
	surfEntries := d.BFS.SurfBFS[snIdx]
	if len(surfEntries) == 0 {
		return -1
	}

	// Build target surfaces: surfaces that have apple links to cluster apples
	inCluster := make(map[int]bool, cl.Size)
	for _, ap := range cl.Apples {
		inCluster[ap] = true
	}
	targetSurfs := make(map[int]bool)
	for si := range g.Surfs {
		s := &g.Surfs[si]
		if s.Type == SurfNone {
			continue
		}
		for _, al := range s.Apples {
			if inCluster[al.Apple] {
				targetSurfs[si] = true
				break
			}
		}
	}
	// Also add surfaces that apples sit on directly
	for _, ap := range cl.Apples {
		if sid := g.SurfAt[ap]; sid >= 0 {
			targetSurfs[sid] = true
		}
	}

	if len(targetSurfs) == 0 {
		return -1
	}

	// For each SurfBFS entry (reachable surface), find shortest surface-link
	// path to any target surface via BFS over surface links.
	bestDist := 999
	bestDir := -1

	for _, entry := range surfEntries {
		sid := entry.SurfID
		if targetSurfs[sid] {
			// Already on a target surface
			if entry.Dist < bestDist {
				bestDist = entry.Dist
				bestDir = entry.FirstDir
			}
			continue
		}

		// BFS over surface links from this entry surface
		dist := surfLinkDist(g, sid, targetSurfs)
		if dist < 0 {
			continue
		}
		totalDist := entry.Dist + dist
		if totalDist < bestDist {
			bestDist = totalDist
			bestDir = entry.FirstDir
		}
	}

	return bestDir
}

// surfLinkDist returns shortest surface-link hop count from src to any target surface.
// Returns -1 if unreachable.
func surfLinkDist(g *Game, src int, targets map[int]bool) int {
	if targets[src] {
		return 0
	}
	type node struct {
		sid  int
		dist int
	}
	visited := make(map[int]bool)
	visited[src] = true
	queue := []node{{src, 0}}
	for qi := 0; qi < len(queue); qi++ {
		cur := queue[qi]
		s := &g.Surfs[cur.sid]
		for _, link := range s.Links {
			if g.Surfs[link.To].Type == SurfNone || visited[link.To] {
				continue
			}
			nd := cur.dist + link.Len
			if targets[link.To] {
				return nd
			}
			visited[link.To] = true
			queue = append(queue, node{link.To, nd})
		}
	}
	return -1
}

func (d *Decision) phaseScoring() {
	g := d.G
	myN := len(d.MySnakes)
	if myN == 0 {
		return
	}

	nClusters := len(g.Clusters)

	// Constellation assignment for spreading bots
	assignedCluster := make([]int, myN)
	for i := range assignedCluster {
		assignedCluster[i] = -1
	}

	if nClusters > 0 {
		type assignment struct {
			si, ci, score int
		}
		var candidates []assignment
		for si, snIdx := range d.MySnakes {
			sn := &g.Sn[snIdx]
			if !sn.Alive || sn.Len == 0 {
				continue
			}
			for ci := range g.Clusters {
				s := d.constScore(snIdx, &g.Clusters[ci])
				candidates = append(candidates, assignment{si: si, ci: ci, score: s})
			}
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].score > candidates[j].score
		})
		usedSnake := make(map[int]bool, myN)
		usedCluster := make(map[int]bool, nClusters)
		for _, c := range candidates {
			if usedSnake[c.si] || usedCluster[c.ci] {
				continue
			}
			usedSnake[c.si] = true
			usedCluster[c.ci] = true
			assignedCluster[c.si] = c.ci
		}
		for si, snIdx := range d.MySnakes {
			if usedSnake[si] {
				continue
			}
			sn := &g.Sn[snIdx]
			if !sn.Alive || sn.Len == 0 {
				continue
			}
			bestCI := -1
			bestScore := -1 << 30
			for ci := range g.Clusters {
				s := d.constScore(snIdx, &g.Clusters[ci])
				if s > bestScore {
					bestScore = s
					bestCI = ci
				}
			}
			assignedCluster[si] = bestCI
		}
	}

	// For each snake: cluster apple first, fallback to any apple, then navigate
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		if !sn.Alive || sn.Len == 0 {
			continue
		}

		reach := d.BFS.Reach[snIdx]
		prevTarget := d.P.PrevAssign[si]

		// 1. Cluster apple (sticky)
		ci := assignedCluster[si]
		if ci >= 0 {
			cl := &g.Clusters[ci]
			apple, dir, found := d.constBestApple(snIdx, cl, prevTarget)
			if found {
				d.Assigned[si] = apple
				d.AssignedDir[si] = dir
				continue
			}
		}

		// 2. Any closest apple
		if len(reach) > 0 {
			if prevTarget >= 0 {
				for _, ri := range reach {
					if ri.Apple == prevTarget {
						d.Assigned[si] = ri.Apple
						d.AssignedDir[si] = ri.FirstDir
						break
					}
				}
			}
			if d.Assigned[si] < 0 {
				d.Assigned[si] = reach[0].Apple
				d.AssignedDir[si] = reach[0].FirstDir
			}
			continue
		}

		// 3. Navigate toward cluster
		if ci >= 0 {
			dir := d.constNavigate(snIdx, &g.Clusters[ci])
			if dir >= 0 {
				d.AssignedDir[si] = dir
			}
		}
	}

	for si := range d.MySnakes {
		d.P.PrevAssign[si] = d.Assigned[si]
	}
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
