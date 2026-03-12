package agentkit

import "time"

// SearchResult holds the outcome of a pathfinding search.
type SearchResult struct {
	Dir    Direction
	Target Point
	Steps  int
	Score  int
	Ok     bool
}

// EnemyInfo holds per-enemy data for danger/distance calculations.
type EnemyInfo struct {
	Head    Point
	Facing  Direction
	BodyLen int
	Body    []Point
}

// DirInfo holds flood-fill and distance data for one candidate direction.
type DirInfo struct {
	Flood int
	Dists []int
	Body  []Point
	Alive bool
}

// StateHash computes a hash of (facing, body) for BFS deduplication.
func StateHash(facing Direction, body []Point) uint64 {
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

// CalcDirInfo runs SimMove + FloodDist for every valid move from the body head.
func (s *State) CalcDirInfo(body []Point, facing Direction, occupied *BitGrid) map[Direction]*DirInfo {
	head := body[0]
	info := make(map[Direction]*DirInfo, 3)
	for _, dir := range s.VMoves(head, facing) {
		nb, _, alive, _, _ := s.SimMove(body, facing, dir, nil, occupied)
		di := &DirInfo{Alive: alive}
		if alive {
			di.Body = make([]Point, len(nb))
			copy(di.Body, nb)
			blocked := NewBG(s.Grid.Width, s.Grid.Height)
			copy(blocked.Bits, occupied.Bits)
			for _, p := range di.Body[1:] {
				blocked.Set(p)
			}
			di.Flood, di.Dists = s.FloodDist(di.Body[0], &blocked)
		}
		info[dir] = di
	}
	return info
}

// IsSafeDir returns true when dir leads to enough open space (flood >= bodyLen*2, min 4).
func IsSafeDir(dir Direction, dirInfo map[Direction]*DirInfo, bodyLen int) bool {
	di, ok := dirInfo[dir]
	if !ok || !di.Alive {
		return false
	}
	thresh := bodyLen * 2
	if thresh < 4 {
		thresh = 4
	}
	return di.Flood >= thresh
}

// BestSafeDir returns the alive direction with the highest flood count.
func BestSafeDir(dirInfo map[Direction]*DirInfo) (Direction, bool) {
	best := DirNone
	bestFlood := -1
	for dir, di := range dirInfo {
		if di.Alive && di.Flood > bestFlood {
			bestFlood = di.Flood
			best = dir
		}
	}
	return best, best != DirNone
}

// CalcEnemyDist returns per-cell minimum BFS distance from any enemy head.
func (s *State) CalcEnemyDist(enemies []EnemyInfo, allOcc *BitGrid) []int {
	n := s.Grid.Width * s.Grid.Height
	result := make([]int, n)
	for i := range result {
		result[i] = Unreachable
	}
	for _, e := range enemies {
		blocked := OccExcept(allOcc, e.Body)
		_, eDists := s.FloodDist(e.Head, &blocked)
		for i, d := range eDists {
			if d < result[i] {
				result[i] = d
			}
		}
	}
	return result
}

// FiltSrc removes sources reachable by an enemy at least 4 steps before us.
// Falls back to all sources if everything is filtered out.
func (s *State) FiltSrc(sources []Point, myDists, enemyDists []int) []Point {
	W := s.Grid.Width
	out := make([]Point, 0, len(sources))
	for _, src := range sources {
		si := src.Y*W + src.X
		md, ed := myDists[si], enemyDists[si]
		if md != Unreachable && ed != Unreachable && ed < md-3 {
			continue
		}
		out = append(out, src)
	}
	if len(out) == 0 {
		return sources
	}
	return out
}

// InstantEat checks whether any source is reachable in one step.
func (s *State) InstantEat(body []Point, facing Direction, sources []Point, srcBG, occupied *BitGrid) SearchResult {
	head := body[0]
	var best SearchResult
	for _, dir := range s.VMoves(head, facing) {
		target := Add(head, DirDelta[dir])
		if !srcBG.Has(target) {
			continue
		}
		_, _, alive, _, _ := s.SimMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}
		score := SrcScore(s.Grid, head, target)
		if !best.Ok || score < best.Score {
			best = SearchResult{Dir: dir, Target: target, Steps: 1, Score: score, Ok: true}
		}
	}
	return best
}

// PathBFS searches for the best path to any source within maxDepth steps.
func (s *State) PathBFS(body []Point, facing Direction, sources []Point,
	maxDepth int, dirInfo map[Direction]*DirInfo, enemyDists []int,
	srcBG, occupied *BitGrid, deadline time.Time) SearchResult {

	if len(sources) == 0 {
		return SearchResult{}
	}

	type qItem struct {
		body  []Point
		face  Direction
		first Direction
		depth int
	}

	startBody := make([]Point, len(body))
	copy(startBody, body)
	queue := []qItem{{body: startBody, face: facing, first: DirNone}}
	seen := map[uint64]bool{StateHash(facing, body): true}
	best := SearchResult{}
	iters := 0
	bodyLen := len(body)
	W := s.Grid.Width

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
		for _, dir := range s.VMoves(head, item.face) {
			nb, nf, alive, ate, eatenAt := s.SimMove(item.body, item.face, dir, srcBG, occupied)
			if !alive {
				continue
			}
			first := item.first
			if first == DirNone {
				first = dir
			}
			if ate && srcBG.Has(eatenAt) {
				rawSteps := item.depth + 1
				score := rawSteps * 1000
				score += SrcScore(s.Grid, body[0], eatenAt)
				if di, ok := dirInfo[first]; ok && di.Alive {
					if di.Flood < bodyLen*2 {
						score += 3000
					} else if di.Flood < bodyLen*3 {
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
				if s.Terr.MinBodyLen(body, eatenAt) <= bodyLen {
					score -= 200
				}
				cand := SearchResult{Dir: first, Target: eatenAt, Steps: rawSteps, Score: score, Ok: true}
				if !best.Ok || cand.Score < best.Score {
					best = cand
				}
				continue
			}
			if best.Ok && item.depth+1 >= best.Steps {
				continue
			}
			h := StateHash(nf, nb)
			if seen[h] {
				continue
			}
			seen[h] = true
			cp := make([]Point, len(nb))
			copy(cp, nb)
			queue = append(queue, qItem{body: cp, face: nf, first: first, depth: item.depth + 1})
		}
	}
	return best
}

// BestAction chooses the best direction when no BFS path was found.
func (s *State) BestAction(body []Point, facing Direction, sources []Point,
	dirInfo map[Direction]*DirInfo, enemies []EnemyInfo, enemyDists []int,
	srcBG, occupied, danger *BitGrid) SearchResult {

	if len(sources) == 0 {
		return SearchResult{Dir: DirUp, Ok: true}
	}
	head := body[0]
	bodyLen := len(body)
	W := s.Grid.Width

	initRun := s.Terr.BodyInitRun(body)
	reachable := s.Terr.SupReachMulti(head, initRun, bodyLen, sources, srcBG)

	var best SearchResult
	for _, dir := range LegalDirs(facing) {
		nb, _, alive, ate, eatenAt := s.SimMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}

		di := dirInfo[dir]
		bestTarget := sources[0]
		bestDist := Unreachable

		if len(reachable) > 0 {
			useBFS := di != nil && di.Alive && di.Dists != nil
			for _, c := range reachable {
				var d int
				if useBFS {
					d = di.Dists[c.Y*W+c.X]
				} else {
					d = SrcScore(s.Grid, nb[0], c)
				}
				if d < bestDist {
					bestDist = d
					bestTarget = c
				}
			}
		} else {
			useBFS := di != nil && di.Alive && di.Dists != nil
			for _, src := range sources {
				var d int
				if useBFS {
					d = di.Dists[src.Y*W+src.X]
				} else {
					d = SrcScore(s.Grid, nb[0], src)
				}
				if d < bestDist {
					bestDist = d
					bestTarget = src
				}
			}
		}

		score := bestDist
		if ate && srcBG.Has(eatenAt) {
			score = -1000
			bestTarget = eatenAt
		}

		expectedLen := bodyLen
		if ate {
			expectedLen++
		}
		if len(nb) < expectedLen {
			if bodyLen <= 5 {
				score += 1000
			} else {
				score += 300
			}
		}

		if danger.Has(nb[0]) {
			dangerPen := 20
			if bodyLen <= 3 {
				dangerPen = 500
			} else if bodyLen <= 5 {
				dangerPen = 100
			}
			for _, e := range enemies {
				canReach := false
				for _, edir := range LegalDirs(e.Facing) {
					if Add(e.Head, DirDelta[edir]) == nb[0] {
						canReach = true
						break
					}
				}
				if canReach && e.BodyLen <= 3 && bodyLen > 3 {
					dangerPen = -500
				}
			}
			score += dangerPen
		}

		if nb[0] == head {
			score += 200
		}
		for d := DirUp; d <= DirLeft; d++ {
			if s.Grid.IsWall(Add(nb[0], DirDelta[d])) {
				score--
			}
		}

		if di != nil && di.Alive {
			if di.Flood < bodyLen {
				score += 2000
			} else if di.Flood < bodyLen*2 {
				score += 500
			}
		} else if di == nil {
			score += 1500
		}

		if s.Grid.WBelow(nb[0]) {
			score -= 3
		}

		ti := bestTarget.Y*W + bestTarget.X
		if enemyDists[ti] != Unreachable {
			if bestDist < Unreachable && enemyDists[ti] < bestDist-3 {
				score += 50
			}
		}

		cand := SearchResult{Dir: dir, Target: bestTarget, Score: score, Ok: true}
		if !best.Ok || cand.Score < best.Score {
			best = cand
		}
	}

	if best.Ok {
		return best
	}
	return SearchResult{Dir: facing, Ok: true}
}
