package main

import "time"

func floodDist(start Point, blocked *BitGrid) (int, []int) {
	n := W * H
	dist := make([]int, n)
	for i := range dist {
		dist[i] = Unreachable
	}
	if grid.IsWall(start) || (blocked != nil && blocked.Has(start)) {
		return 0, dist
	}
	dist[start.Y*W+start.X] = 0
	q := state.DistQ[:0]
	q = append(q, start)
	count := 0
	for i := 0; i < len(q); i++ {
		p := q[i]
		count++
		d := dist[p.Y*W+p.X]
		for dir := DirUp; dir <= DirLeft; dir++ {
			np := Add(p, DirDelta[dir])
			if grid.IsWall(np) {
				continue
			}
			ni := np.Y*W + np.X
			if dist[ni] != Unreachable || (blocked != nil && blocked.Has(np)) {
				continue
			}
			dist[ni] = d + 1
			q = append(q, np)
		}
	}
	state.DistQ = q
	return count, dist
}

func cmdFlood(body []Point, facing Direction, occupied *BitGrid) (int, []int) {
	n := W * H
	dist := make([]int, n)
	for i := range dist {
		dist[i] = Unreachable
	}
	head := body[0]
	if grid.IsWall(head) || (occupied != nil && occupied.Has(head)) {
		return 0, dist
	}
	dist[head.Y*W+head.X] = 0
	type landing struct {
		pos  Point
		dist int
	}
	var landings []landing
	landings = append(landings, landing{pos: head, dist: 0})
	for _, dir := range state.VMoves(head, facing) {
		nb, _, alive, _, _ := simMove(body, facing, dir, nil, occupied)
		if !alive {
			continue
		}
		nh := nb[0]
		ni := nh.Y*W + nh.X
		if ni >= 0 && ni < n && dist[ni] == Unreachable {
			dist[ni] = 1
			landings = append(landings, landing{pos: nh, dist: 1})
		}
	}
	q := state.DistQ[:0]
	for _, l := range landings {
		q = append(q, l.pos)
	}
	count := 0
	for i := 0; i < len(q); i++ {
		p := q[i]
		count++
		d := dist[p.Y*W+p.X]
		for dir := DirUp; dir <= DirLeft; dir++ {
			np := Add(p, DirDelta[dir])
			if grid.IsWall(np) {
				continue
			}
			ni := np.Y*W + np.X
			if dist[ni] != Unreachable || (occupied != nil && occupied.Has(np)) {
				continue
			}
			dist[ni] = d + 1
			q = append(q, np)
		}
	}
	state.DistQ = q
	return count, dist
}

func stateHash(facing Direction, body []Point) uint64 {
	h := uint64(14695981039346656037)
	h ^= uint64(facing)
	h *= 1099511628211
	for _, p := range body {
		h ^= uint64(p.X)
		h *= 1099511628211
		h ^= uint64(p.Y)
		h *= 1099511628211
	}
	return h
}

func cmdBFS(body []Point, facing Direction, sources []Point,
	maxDepth int, dirInfo map[Direction]*DirInfo, enemyDists []int,
	srcBG, occupied *BitGrid, deadline time.Time) SearchResult {

	if len(sources) == 0 {
		return SearchResult{}
	}

	appleIdx := make(map[Point]int, len(sources))
	for i, s := range sources {
		if i >= 64 {
			break
		}
		appleIdx[s] = i
	}

	type qItem struct {
		body     []Point
		face     Direction
		first    Direction
		depth    int
		eatenSet uint64
	}

	startBody := make([]Point, len(body))
	copy(startBody, body)
	queue := []qItem{{body: startBody, face: facing, first: DirNone}}
	seen := map[uint64]bool{stateHash(facing, body): true}
	best := SearchResult{}
	iters := 0
	bodyLen := len(body)

	for qi := 0; qi < len(queue); qi++ {
		item := queue[qi]
		if item.depth >= maxDepth {
			continue
		}
		iters++
		if iters&255 == 0 && time.Now().After(deadline) {
			break
		}

		head := item.body[0]

		var restored []Point
		if item.eatenSet != 0 {
			for s, idx := range appleIdx {
				if item.eatenSet&(1<<uint(idx)) != 0 {
					if srcBG.Has(s) {
						srcBG.Clear(s)
						restored = append(restored, s)
					}
				}
			}
		}

		for _, dir := range state.VMoves(head, item.face) {
			nb, nf, alive, ate, eatenAt := simMove(item.body, item.face, dir, srcBG, occupied)
			if !alive {
				continue
			}
			first := item.first
			if first == DirNone {
				first = dir
			}

			newEaten := item.eatenSet
			if ate && srcBG.Has(eatenAt) {
				if !hasFollowupEscape(nb, nf, srcBG, occupied, eatenAt) {
					continue
				}
				if idx, ok := appleIdx[eatenAt]; ok {
					newEaten |= 1 << uint(idx)
				}

				rawSteps := item.depth + 1
				score := rawSteps * 1000
				score += srcScore(body[0], eatenAt)
				if di, ok := dirInfo[first]; ok && di.alive && rawSteps == 1 {
					if di.flood < bodyLen*2 {
						score += 3000
					} else if di.flood < bodyLen*3 {
						score += 1000
					}
				}
				ei := eatenAt.Y*W + eatenAt.X
				if enemyDists[ei] != Unreachable {
					ed := enemyDists[ei]
					if rawSteps <= ed {
						score -= 300
					} else if rawSteps <= ed+2 {
						score += 500
					} else {
						score += 2000
					}
				}
				if minLen := state.Terr.MinBodyLen(body, eatenAt); minLen <= bodyLen {
					surplus := bodyLen - minLen
					if surplus > 4 {
						surplus = 4
					}
					score -= 50 + surplus*50
				}
				for _, s := range sources {
					if s != eatenAt && MDist(eatenAt, s) <= 4 {
						score -= 100
					}
				}
				cand := SearchResult{dir: first, target: eatenAt, steps: rawSteps, score: score, ok: true}
				if !best.ok || cand.score < best.score {
					best = cand
				}
				h := stateHash(nf, nb)
				if !seen[h] {
					seen[h] = true
					cp := make([]Point, len(nb))
					copy(cp, nb)
					queue = append(queue, qItem{body: cp, face: nf, first: first, depth: item.depth + 1, eatenSet: newEaten})
				}
				continue
			}
			if best.ok && item.depth+1 >= best.steps {
				continue
			}
			h := stateHash(nf, nb)
			if seen[h] {
				continue
			}
			seen[h] = true
			cp := make([]Point, len(nb))
			copy(cp, nb)
			queue = append(queue, qItem{body: cp, face: nf, first: first, depth: item.depth + 1, eatenSet: newEaten})
		}

		for _, s := range restored {
			srcBG.Set(s)
		}
	}
	return best
}
