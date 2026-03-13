package agentkit

import (
	"sort"
	"time"
)

type SupportJob struct {
	ClimberID int
	Apple     Point
	Cell      Point
	Score     int
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

func (s *State) PlanSupportJobs(mine []MyBotInfo, preferred [][]Point, sources []Point, botDists [][]int, deadline time.Time) map[int]SupportJob {
	if len(mine) < 2 || len(sources) == 0 || time.Until(deadline) < 18*time.Millisecond {
		return nil
	}

	srcBG := NewBG(s.Grid.Width, s.Grid.Height)
	FillBG(&srcBG, sources)

	hasReachable := make([]bool, len(mine))
	for i, bot := range mine {
		bodyLen := len(bot.Body)
		initRun := s.Terr.BodyInitRun(bot.Body)
		targets := limitedSupportTargets(preferred[i])
		if len(targets) == 0 {
			targets = limitedSupportTargets(sources)
		}
		if len(s.Terr.SupReachMulti(bot.Body[0], initRun, bodyLen, targets, &srcBG)) > 0 {
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
	W, H := s.Grid.Width, s.Grid.Height

	for supporter := range mine {
		if hasReachable[supporter] {
			continue
		}
		if time.Until(deadline) < 8*time.Millisecond {
			break
		}
		supporterLen := len(mine[supporter].Body)
		for climber := range mine {
			if climber == supporter || len(mine[climber].Body) <= supporterLen {
				continue
			}
			if time.Until(deadline) < 8*time.Millisecond {
				break
			}

			climberLen := len(mine[climber].Body)
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
				path := s.Terr.SupPathBFS(mine[climber].Body[0], s.Terr.BodyInitRun(mine[climber].Body), apple, &srcBG)
				if path != nil && path.MinLen <= climberLen {
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
						cell := Point{X: sx, Y: y}
						if s.Grid.IsWall(cell) {
							break
						}
						ci := cell.Y*W + cell.X
						if botDists[supporter][ci] == Unreachable {
							continue
						}
						minLen, climbDist := s.Terr.MinImmLen(cell, apple, &srcBG)
						if minLen == Unreachable || minLen > climberLen {
							continue
						}

						score := botDists[supporter][ci] * 20
						score += climbDist * 8
						score += MDist(mine[climber].Body[0], cell) * 6
						score -= apple.Y * 25
						score += abs(dx) * 10
						if s.Grid.WBelow(cell) {
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
		return mine[cands[i].supporter].ID < mine[cands[j].supporter].ID
	})

	usedSupporter := make([]bool, len(mine))
	usedClimber := make([]bool, len(mine))
	jobs := make(map[int]SupportJob, len(mine))
	for _, cand := range cands {
		if usedSupporter[cand.supporter] || usedClimber[cand.climber] {
			continue
		}
		usedSupporter[cand.supporter] = true
		usedClimber[cand.climber] = true
		jobs[mine[cand.supporter].ID] = SupportJob{
			ClimberID: mine[cand.climber].ID,
			Apple:     cand.apple,
			Cell:      cand.cell,
			Score:     cand.score,
		}
	}

	if len(jobs) == 0 {
		return nil
	}
	return jobs
}

func (s *State) BestGroundAction(body []Point, facing Direction, target Point,
	dirInfo map[Direction]*DirInfo, enemies []EnemyInfo,
	srcBG, occupied, danger *BitGrid) SearchResult {

	bodyLen := len(body)
	var best SearchResult
	for _, dir := range s.VMoves(body[0], facing) {
		nb, _, alive, ate, eatenAt := s.SimMove(body, facing, dir, srcBG, occupied)
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
		if ate && srcBG != nil && srcBG.Has(eatenAt) {
			score -= 60
		}

		below := Point{X: nb[0].X, Y: nb[0].Y + 1}
		if s.Grid.WBelow(nb[0]) || (srcBG != nil && srcBG.Has(below)) {
			score -= 10
		}

		if danger != nil && danger.Has(nb[0]) {
			dangerPen := 40
			if bodyLen <= 3 {
				dangerPen = 600
			} else if bodyLen <= 5 {
				dangerPen = 150
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
					dangerPen = -400
				}
			}
			score += dangerPen
		}

		if di != nil && di.Alive {
			if di.Flood < bodyLen {
				score += 2500
			} else if di.Flood < bodyLen*2 {
				score += 700
			}
		} else {
			score += 2000
		}

		cand := SearchResult{Dir: dir, Target: target, Score: score, Ok: true}
		if !best.Ok || cand.Score < best.Score {
			best = cand
		}
	}

	if best.Ok {
		return best
	}
	return SearchResult{Dir: facing, Target: target, Ok: true}
}
