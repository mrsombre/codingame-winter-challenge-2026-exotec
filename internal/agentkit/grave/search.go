package bot

import (
	"fmt"
	"time"

	"codingame/internal/agentkit/game"
)

// ActionString formats a bot action for output.
// If reason is non-empty it is appended (useful for debug builds).
func ActionString(id int, dir game.Direction, reason string) string {
	if reason != "" {
		return fmt.Sprintf("%d %s %s", id, game.DirName[dir], reason)
	}
	return fmt.Sprintf("%d %s", id, game.DirName[dir])
}

// SearchResult holds the outcome of a pathfinding search.
type SearchResult struct {
	Dir    game.Direction
	Target game.Point
	Steps  int
	Score  int
	Ok     bool
}

// EnemyInfo holds per-enemy data for danger/distance calculations.
type EnemyInfo struct {
	Head    game.Point
	Facing  game.Direction
	BodyLen int
	Body    []game.Point
}

// DirInfo holds flood-fill and distance data for one candidate direction.
type DirInfo struct {
	Flood int
	Dists []int
	Body  []game.Point
	Alive bool
}

// StateHash computes a hash of (facing, body) for BFS deduplication.
func StateHash(facing game.Direction, body []game.Point) uint64 {
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
func CalcDirInfo(s *game.State, body []game.Point, facing game.Direction, occupied *game.BitGrid) map[game.Direction]*DirInfo {
	head := body[0]
	info := make(map[game.Direction]*DirInfo, 3)
	for _, dir := range s.VMoves(head, facing) {
		nb, _, alive, _, _ := s.SimMove(body, facing, dir, nil, occupied)
		di := &DirInfo{Alive: alive}
		if alive {
			di.Body = make([]game.Point, len(nb))
			copy(di.Body, nb)
			blocked := game.NewBG(s.Grid.Width, s.Grid.Height)
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
func IsSafeDir(dir game.Direction, dirInfo map[game.Direction]*DirInfo, bodyLen int) bool {
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
func BestSafeDir(dirInfo map[game.Direction]*DirInfo) (game.Direction, bool) {
	best := game.DirNone
	bestFlood := -1
	for dir, di := range dirInfo {
		if di.Alive && di.Flood > bestFlood {
			bestFlood = di.Flood
			best = dir
		}
	}
	return best, best != game.DirNone
}

// CalcEnemyDist returns per-cell minimum BFS distance from any enemy head.
func CalcEnemyDist(s *game.State, enemies []EnemyInfo, allOcc *game.BitGrid) []int {
	n := s.Grid.Width * s.Grid.Height
	result := make([]int, n)
	for i := range result {
		result[i] = game.Unreachable
	}
	for _, e := range enemies {
		blocked := game.OccExcept(allOcc, e.Body)
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
func FiltSrc(s *game.State, sources []game.Point, myDists, enemyDists []int) []game.Point {
	W := s.Grid.Width
	out := make([]game.Point, 0, len(sources))
	for _, src := range sources {
		si := src.Y*W + src.X
		md, ed := myDists[si], enemyDists[si]
		if md != game.Unreachable && ed != game.Unreachable && ed < md-3 {
			continue
		}
		out = append(out, src)
	}
	if len(out) == 0 {
		return sources
	}
	return out
}

// HasFollowupEscape checks that after eating at eatenAt, at least one
// subsequent move keeps the bot alive.
func HasFollowupEscape(s *game.State, body []game.Point, facing game.Direction, sources, occupied *game.BitGrid, eatenAt game.Point) bool {
	nextSources := sources
	if sources != nil && sources.Has(eatenAt) {
		cloned := game.NewBG(sources.Width, sources.Height)
		copy(cloned.Bits, sources.Bits)
		cloned.Clear(eatenAt)
		nextSources = &cloned
	}
	head := body[0]
	for _, dir := range s.VMoves(head, facing) {
		_, _, alive, _, _ := s.SimMove(body, facing, dir, nextSources, occupied)
		if alive {
			return true
		}
	}
	return false
}

// InstantEat checks whether any source is reachable in one step.
// Rejects eat-moves that leave the bot with no follow-up escape.
func InstantEat(s *game.State, body []game.Point, facing game.Direction, sources []game.Point, srcBG, occupied *game.BitGrid) SearchResult {
	head := body[0]
	var best SearchResult
	for _, dir := range s.VMoves(head, facing) {
		target := game.Add(head, game.DirDelta[dir])
		if !srcBG.Has(target) {
			continue
		}
		nb, nf, alive, ate, eatenAt := s.SimMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}
		if ate {
			// nb aliases SimBuf; copy before HasFollowupEscape calls SimMove again.
			cp := make([]game.Point, len(nb))
			copy(cp, nb)
			if !HasFollowupEscape(s, cp, nf, srcBG, occupied, eatenAt) {
				continue
			}
		}
		score := game.SrcScore(s.Grid, head, target)
		if !best.Ok || score < best.Score {
			best = SearchResult{Dir: dir, Target: target, Steps: 1, Score: score, Ok: true}
		}
	}
	return best
}

// PathBFS searches for the best path to any source within maxDepth steps.
func PathBFS(s *game.State, body []game.Point, facing game.Direction, sources []game.Point,
	maxDepth int, dirInfo map[game.Direction]*DirInfo, enemyDists []int,
	srcBG, occupied *game.BitGrid, deadline time.Time) SearchResult {

	if len(sources) == 0 {
		return SearchResult{}
	}

	type qItem struct {
		body  []game.Point
		face  game.Direction
		first game.Direction
		depth int
	}

	startBody := make([]game.Point, len(body))
	copy(startBody, body)
	queue := []qItem{{body: startBody, face: facing, first: game.DirNone}}
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
			if first == game.DirNone {
				first = dir
			}
			if ate && srcBG.Has(eatenAt) {
				// nb aliases SimBuf; copy before HasFollowupEscape calls SimMove again.
				cp := make([]game.Point, len(nb))
				copy(cp, nb)
				if !HasFollowupEscape(s, cp, nf, srcBG, occupied, eatenAt) {
					continue
				}
				rawSteps := item.depth + 1
				score := rawSteps * 1000
				score += game.SrcScore(s.Grid, body[0], eatenAt)
				if di, ok := dirInfo[first]; ok && di.Alive {
					if di.Flood < bodyLen*2 {
						score += 3000
					} else if di.Flood < bodyLen*3 {
						score += 1000
					}
				}
				ei := eatenAt.Y*W + eatenAt.X
				if enemyDists[ei] != game.Unreachable {
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
			cp := make([]game.Point, len(nb))
			copy(cp, nb)
			queue = append(queue, qItem{body: cp, face: nf, first: first, depth: item.depth + 1})
		}
	}
	return best
}

// BestAction chooses the best direction when no BFS path was found.
func BestAction(s *game.State, body []game.Point, facing game.Direction, sources []game.Point,
	dirInfo map[game.Direction]*DirInfo, enemies []EnemyInfo, enemyDists []int,
	srcBG, occupied, danger *game.BitGrid) SearchResult {

	if len(sources) == 0 {
		return SearchResult{Dir: game.DirUp, Ok: true}
	}
	head := body[0]
	bodyLen := len(body)
	W := s.Grid.Width

	initRun := s.Terr.BodyInitRun(body)
	reachable := s.Terr.SupReachMulti(head, initRun, bodyLen, sources, srcBG)

	var best SearchResult
	for _, dir := range game.LegalDirs(facing) {
		nb, _, alive, ate, eatenAt := s.SimMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}

		di := dirInfo[dir]
		bestTarget := sources[0]
		bestDist := game.Unreachable

		if len(reachable) > 0 {
			useBFS := di != nil && di.Alive && di.Dists != nil
			for _, c := range reachable {
				var d int
				if useBFS {
					d = di.Dists[c.Y*W+c.X]
				} else {
					d = game.SrcScore(s.Grid, nb[0], c)
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
					d = game.SrcScore(s.Grid, nb[0], src)
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
				for _, edir := range game.LegalDirs(e.Facing) {
					if game.Add(e.Head, game.DirDelta[edir]) == nb[0] {
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
		for d := game.DirUp; d <= game.DirLeft; d++ {
			if s.Grid.IsWall(game.Add(nb[0], game.DirDelta[d])) {
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
		if enemyDists[ti] != game.Unreachable {
			if bestDist < game.Unreachable && enemyDists[ti] < bestDist-3 {
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
