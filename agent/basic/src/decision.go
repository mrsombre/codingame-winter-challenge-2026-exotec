package main

import (
	"sort"
	"time"
)

func srcScore(head, target Point) int {
	d := MDist(head, target)
	if target.Y < head.Y {
		d += head.Y - target.Y
	}
	if grid.WBelow(target) {
		d -= 3
	}
	return d
}

func calcDirInfo(body []Point, facing Direction, occupied *BitGrid) map[Direction]*DirInfo {
	head := body[0]
	info := make(map[Direction]*DirInfo, 3)
	for _, dir := range state.VMoves(head, facing) {
		nb, _, alive, _, _ := simMove(body, facing, dir, nil, occupied)
		di := &DirInfo{alive: alive}
		if alive {
			di.body = make([]Point, len(nb))
			copy(di.body, nb)
			blocked := NewBG(W, H)
			copy(blocked.Bits, occupied.Bits)
			for _, p := range di.body[1:] {
				blocked.Set(p)
			}
			di.flood, di.dists = floodDist(di.body[0], &blocked)
		}
		info[dir] = di
	}
	return info
}

func isSafeDir(dir Direction, dirInfo map[Direction]*DirInfo, bodyLen int) bool {
	di, ok := dirInfo[dir]
	if !ok || !di.alive {
		return false
	}
	thresh := bodyLen * 2
	if thresh < 4 {
		thresh = 4
	}
	return di.flood >= thresh
}

func bestSafeDir(dirInfo map[Direction]*DirInfo) (Direction, bool) {
	best := DirNone
	bestFlood := -1
	for dir, di := range dirInfo {
		if di.alive && di.flood > bestFlood {
			bestFlood = di.flood
			best = dir
		}
	}
	return best, best != DirNone
}

func instantEat(body []Point, facing Direction, sources []Point, srcBG, occupied *BitGrid) SearchResult {
	head := body[0]
	var best SearchResult
	for _, dir := range state.VMoves(head, facing) {
		target := Add(head, DirDelta[dir])
		if !srcBG.Has(target) {
			continue
		}
		nb, nf, alive, ate, eatenAt := simMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}
		if ate && !hasFollowupEscape(nb, nf, srcBG, occupied, eatenAt) {
			continue
		}
		score := srcScore(head, target)
		if !best.ok || score < best.score {
			best = SearchResult{dir: dir, target: target, steps: 1, score: score, ok: true}
		}
	}
	return best
}

func hasFollowupEscape(body []Point, facing Direction, sources, occupied *BitGrid, eatenAt Point) bool {
	nextSources := sources
	if sources != nil && sources.Has(eatenAt) {
		cloned := NewBG(sources.Width, sources.Height)
		copy(cloned.Bits, sources.Bits)
		cloned.Clear(eatenAt)
		nextSources = &cloned
	}

	head := body[0]
	for _, dir := range state.VMoves(head, facing) {
		_, _, alive, _, _ := simMove(body, facing, dir, nextSources, occupied)
		if alive {
			return true
		}
	}
	return false
}

func bestAction(body []Point, facing Direction, sources []Point,
	dirInfo map[Direction]*DirInfo, enemies []enemyInfo, enemyDists []int,
	srcBG, occupied, danger *BitGrid) SearchResult {

	if len(sources) == 0 {
		return SearchResult{dir: DirUp, ok: true}
	}
	head := body[0]
	bodyLen := len(body)

	initRun := state.Terr.BodyInitRun(body)
	reachable := state.Terr.SupReachMulti(head, initRun, bodyLen, sources, srcBG)

	var best SearchResult
	vd1, nd1 := validDirs(facing)
	for _, dir := range vd1[:nd1] {
		nb, _, alive, ate, eatenAt := simMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}

		di := dirInfo[dir]
		bestTarget := sources[0]
		bestDist := Unreachable

		if len(reachable) > 0 {
			useBFS := di != nil && di.alive && di.dists != nil
			for _, c := range reachable {
				var d int
				if useBFS {
					d = di.dists[c.Y*W+c.X]
				} else {
					d = srcScore(nb[0], c)
				}
				if d < bestDist {
					bestDist = d
					bestTarget = c
				}
			}
		} else {
			useBFS := di != nil && di.alive && di.dists != nil
			for _, s := range sources {
				var d int
				if useBFS {
					d = di.dists[s.Y*W+s.X]
				} else {
					d = srcScore(nb[0], s)
				}
				if d < bestDist {
					bestDist = d
					bestTarget = s
				}
			}
		}

		score := bestDist
		if bodyLen >= 8 && bestDist < Unreachable {
			score = bestDist / 2
		}
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
				evd, end := validDirs(e.facing)
				for _, edir := range evd[:end] {
					if Add(e.head, DirDelta[edir]) == nb[0] {
						canReach = true
						break
					}
				}
				if canReach && e.bodyLen <= 3 && bodyLen > 3 {
					dangerPen = -500
				}
			}
			score += dangerPen
		}

		if nb[0] == head {
			score += 200
		}
		for d := DirUp; d <= DirLeft; d++ {
			if grid.IsWall(Add(nb[0], DirDelta[d])) {
				score--
			}
		}

		if di != nil && di.alive {
			if di.flood < bodyLen {
				score += 2000
			} else if di.flood < bodyLen*2 {
				score += 500
			}
			if bodyLen >= 8 {
				spaceBonus := di.flood * 10 / bodyLen
				if spaceBonus > 200 {
					spaceBonus = 200
				}
				score -= spaceBonus
			}
		} else if di == nil {
			score += 1500
		}

		if grid.WBelow(nb[0]) {
			score -= 3
		}

		ti := bestTarget.Y*W + bestTarget.X
		if enemyDists[ti] != Unreachable {
			if bestDist < Unreachable && enemyDists[ti] < bestDist-3 {
				score += 50
			}
		}

		cand := SearchResult{dir: dir, target: bestTarget, score: score, ok: true}
		if !best.ok || cand.score < best.score {
			best = cand
		}
	}

	if best.ok {
		return best
	}
	return SearchResult{dir: facing, ok: true}
}

func bestGroundAction(body []Point, facing Direction, target Point,
	dirInfo map[Direction]*DirInfo, enemies []enemyInfo,
	srcBG, occupied, danger *BitGrid) SearchResult {

	bodyLen := len(body)
	var best SearchResult
	vd2, nd2 := validDirs(facing)
	for _, dir := range vd2[:nd2] {
		nb, _, alive, ate, eatenAt := simMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}

		di := dirInfo[dir]
		score := MDist(nb[0], target) * 12
		if nb[0].X == target.X {
			score -= 12
		}
		if nb[0].Y > target.Y {
			score -= 6
		}
		if nb[0] == target {
			score -= 120
		}
		if ate && srcBG.Has(eatenAt) {
			score -= 60
		}

		below := Point{nb[0].X, nb[0].Y + 1}
		if grid.WBelow(nb[0]) || (srcBG != nil && srcBG.Has(below)) {
			score -= 10
		}

		if danger.Has(nb[0]) {
			dangerPen := 40
			if bodyLen <= 3 {
				dangerPen = 600
			} else if bodyLen <= 5 {
				dangerPen = 150
			}
			for _, e := range enemies {
				canReach := false
				evd2, end2 := validDirs(e.facing)
				for _, edir := range evd2[:end2] {
					if Add(e.head, DirDelta[edir]) == nb[0] {
						canReach = true
						break
					}
				}
				if canReach && e.bodyLen <= 3 && bodyLen > 3 {
					dangerPen = -400
				}
			}
			score += dangerPen
		}

		if di != nil && di.alive {
			if di.flood < bodyLen {
				score += 2500
			} else if di.flood < bodyLen*2 {
				score += 700
			}
		} else {
			score += 2000
		}

		cand := SearchResult{dir: dir, target: target, score: score, ok: true}
		if !best.ok || cand.score < best.score {
			best = cand
		}
	}

	if best.ok {
		return best
	}
	return SearchResult{dir: facing, target: target, ok: true}
}

func calcEnemyDist(enemies []enemyInfo, allOcc *BitGrid) []int {
	n := W * H
	result := make([]int, n)
	for i := range result {
		result[i] = Unreachable
	}
	for _, e := range enemies {
		blocked := occExcept(allOcc, e.body)
		_, eDists := cmdFlood(e.body, e.facing, &blocked)
		for i, d := range eDists {
			if d < result[i] {
				result[i] = d
			}
		}
	}
	return result
}

func predictEnemyWalls(enemies []enemyInfo, sources []Point, allOcc *BitGrid) BitGrid {
	walls := NewBG(W, H)
	for _, e := range enemies {
		for _, p := range e.body {
			walls.Set(p)
		}
		bestDir := e.facing
		bestDist := Unreachable
		dirs, nd := validDirs(e.facing)
		for _, dir := range dirs[:nd] {
			nh := Add(e.head, DirDelta[dir])
			if grid.IsWall(nh) || allOcc.Has(nh) {
				continue
			}
			for _, s := range sources {
				if d := MDist(nh, s); d < bestDist {
					bestDist = d
					bestDir = dir
				}
			}
		}
		predicted := Add(e.head, DirDelta[bestDir])
		if !grid.IsWall(predicted) {
			walls.Set(predicted)
		}
	}
	return walls
}

func filtSrc(sources []Point, myDists, enemyDists []int) []Point {
	out := make([]Point, 0, len(sources))
	for _, s := range sources {
		si := s.Y*W + s.X
		md, ed := myDists[si], enemyDists[si]
		if md != Unreachable && ed != Unreachable && ed < md-3 {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return sources
	}
	return out
}

func limitedSupportTargets(targets []Point) []Point {
	if len(targets) <= 4 {
		return targets
	}
	cp := append([]Point(nil), targets...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Y != cp[j].Y {
			return cp[i].Y > cp[j].Y
		}
		return cp[i].X < cp[j].X
	})
	return cp[:4]
}

func planSupportJobs(mine []botEntry, preferred [][]Point, sources []Point, botDists [][]int, deadline time.Time) map[int]supportJob {
	if len(mine) < 2 || len(sources) == 0 || time.Until(deadline) < 20*time.Millisecond {
		return nil
	}

	srcBG := NewBG(W, H)
	fillBG(&srcBG, sources)

	hasReachable := make([]bool, len(mine))
	for i, bot := range mine {
		bodyLen := len(bot.body)
		initRun := state.Terr.BodyInitRun(bot.body)
		targets := limitedSupportTargets(preferred[i])
		if len(targets) == 0 {
			targets = limitedSupportTargets(sources)
		}
		if len(state.Terr.SupReachMulti(bot.body[0], initRun, bodyLen, targets, &srcBG)) > 0 {
			hasReachable[i] = true
		}
	}

	type supportCand struct {
		supporter int
		climber   int
		apple     Point
		cell      Point
		score     int
	}
	cands := make([]supportCand, 0, len(mine)*len(sources))

	for supporter := range mine {
		if hasReachable[supporter] {
			continue
		}
		if time.Until(deadline) < 8*time.Millisecond {
			break
		}
		supporterLen := len(mine[supporter].body)
		for climber := range mine {
			if climber == supporter || len(mine[climber].body) <= supporterLen {
				continue
			}
			if time.Until(deadline) < 8*time.Millisecond {
				break
			}

			climberLen := len(mine[climber].body)
			targets := limitedSupportTargets(preferred[climber])
			if len(targets) == 0 {
				targets = limitedSupportTargets(sources)
			}
			bestScore := Unreachable
			var bestApple Point
			var bestCell Point

			for _, apple := range targets {
				if time.Until(deadline) < 8*time.Millisecond {
					break
				}
				minLen := state.Terr.SupPathBFS(mine[climber].body[0], state.Terr.BodyInitRun(mine[climber].body), apple, &srcBG)
				if minLen <= climberLen {
					continue
				}

				maxY := apple.Y + 6
				if maxY >= H {
					maxY = H - 1
				}
				for dx := -1; dx <= 1; dx++ {
					sx := apple.X + dx
					if sx < 0 || sx >= W {
						continue
					}
					for y := apple.Y + 1; y <= maxY; y++ {
						cell := Point{sx, y}
						if grid.IsWall(cell) {
							break
						}
						ci := cell.Y*W + cell.X
						if botDists[supporter][ci] == Unreachable {
							continue
						}
						minLen, climbDist := state.Terr.MinImmLen(cell, apple, &srcBG)
						if minLen == Unreachable || minLen > climberLen {
							continue
						}

						score := botDists[supporter][ci] * 20
						score += climbDist * 8
						score += MDist(mine[climber].body[0], cell) * 6
						score -= apple.Y * 25
						score += abs(dx) * 10
						if grid.WBelow(cell) {
							score -= 15
						}
						if score < bestScore {
							bestScore = score
							bestApple = apple
							bestCell = cell
						}
					}
				}
			}

			if bestScore != Unreachable {
				cands = append(cands, supportCand{
					supporter: supporter,
					climber:   climber,
					apple:     bestApple,
					cell:      bestCell,
					score:     bestScore,
				})
			}
		}
	}

	if len(cands) == 0 {
		return nil
	}

	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score < cands[j].score
		}
		if cands[i].apple.Y != cands[j].apple.Y {
			return cands[i].apple.Y > cands[j].apple.Y
		}
		return mine[cands[i].supporter].id < mine[cands[j].supporter].id
	})

	usedSupporter := make([]bool, len(mine))
	usedClimber := make([]bool, len(mine))
	jobs := make(map[int]supportJob, len(mine))
	for _, cand := range cands {
		if usedSupporter[cand.supporter] || usedClimber[cand.climber] {
			continue
		}
		usedSupporter[cand.supporter] = true
		usedClimber[cand.climber] = true
		jobs[mine[cand.supporter].id] = supportJob{
			climberID: mine[cand.climber].id,
			apple:     cand.apple,
			cell:      cand.cell,
			score:     cand.score,
		}
	}

	if len(jobs) == 0 {
		return nil
	}
	return jobs
}
