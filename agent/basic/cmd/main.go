package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"codingame/internal/agentkit/bot"
	"codingame/internal/agentkit/game"
)

const debug = true

var (
	scanner *bufio.Scanner
	grid    *game.AGrid
	state   game.State
	rsc     bot.RefScratch
	W, H    int
)

func readline() string {
	if !scanner.Scan() {
		os.Exit(0)
	}
	line := scanner.Text()
	if debug {
		fmt.Fprintln(os.Stderr, line)
	}
	return line
}

func parseBody(s string) []game.Point {
	parts := strings.Split(s, ":")
	pts := make([]game.Point, len(parts))
	for i, p := range parts {
		fmt.Sscanf(p, "%d,%d", &pts[i].X, &pts[i].Y)
	}
	return pts
}

func main() {
	scanner = bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	readline()
	fmt.Sscan(readline(), &W)
	fmt.Sscan(readline(), &H)

	walls := make(map[game.Point]bool)
	for y := 0; y < H; y++ {
		row := readline()
		for x, ch := range row {
			if ch == '#' {
				walls[game.Point{X: x, Y: y}] = true
			}
		}
	}
	grid = game.NewAG(W, H, walls)
	state = game.NewState(grid)
	rsc = bot.NewRefScratch(W, H)

	var botsPerPlayer int
	fmt.Sscan(readline(), &botsPerPlayer)
	myBots := make(map[int]bool)
	for i := 0; i < botsPerPlayer; i++ {
		var id int
		fmt.Sscan(readline(), &id)
		myBots[id] = true
	}
	for i := 0; i < botsPerPlayer; i++ {
		readline()
	}

	turn := 0
	for {
		var srcN int
		fmt.Sscan(readline(), &srcN)
		sources := make([]game.Point, srcN)
		for i := range sources {
			fmt.Sscan(readline(), &sources[i].X, &sources[i].Y)
		}

		var botN int
		fmt.Sscan(readline(), &botN)

		allOcc := game.NewBG(W, H)
		var mine []bot.MyBotInfo
		var enemies []bot.EnemyInfo

		for i := 0; i < botN; i++ {
			var id int
			var body string
			fmt.Sscan(readline(), &id, &body)
			pts := parseBody(body)
			for _, p := range pts {
				allOcc.Set(p)
			}
			f := game.DirUp
			if len(pts) >= 2 {
				f = game.FacingPts(pts[0], pts[1])
			}
			if myBots[id] {
				mine = append(mine, bot.MyBotInfo{ID: id, Body: pts})
			} else {
				enemies = append(enemies, bot.EnemyInfo{Head: pts[0], Facing: f, BodyLen: len(pts), Body: pts})
			}
		}

		budget := 45 * time.Millisecond
		if turn == 0 {
			budget = 900 * time.Millisecond
		}
		turnDeadline := time.Now().Add(budget)

		eDanger := game.NewBG(W, H)
		for _, e := range enemies {
			for _, d := range game.LegalDirs(e.Facing) {
				eDanger.Set(game.Add(e.Head, game.DirDelta[d]))
			}
		}

		enemyDists := bot.CalcEnemyDist(&state, enemies, &allOcc)

		sort.Slice(mine, func(i, j int) bool {
			di, dj := game.Unreachable, game.Unreachable
			for _, s := range sources {
				if d := game.MDist(mine[i].Body[0], s); d < di {
					di = d
				}
				if d := game.MDist(mine[j].Body[0], s); d < dj {
					dj = d
				}
			}
			if di != dj {
				return di < dj
			}
			return mine[i].ID < mine[j].ID
		})

		vsrc := make([][]game.Point, len(mine))
		botDists := make([][]int, len(mine))
		for i, b := range mine {
			occ := game.OccExcept(&allOcc, b.Body)
			_, botDists[i] = state.FloodDist(b.Body[0], &occ)
		}
		for _, s := range sources {
			si := s.Y*W + s.X
			bestBot := -1
			bestDist := game.Unreachable
			for i := range mine {
				d := botDists[i][si]
				if d < bestDist {
					bestDist = d
					bestBot = i
				}
			}
			if bestBot >= 0 {
				vsrc[bestBot] = append(vsrc[bestBot], s)
			}
		}
		supportJobs := bot.PlanSupportJobs(&state, mine, vsrc, sources, botDists, turnDeadline)

		plans := make([]bot.BotPlan, 0, len(mine))
		plannedHeads := game.NewBG(W, H)

		for botIdx, b := range mine {
			body := b.Body
			head := body[0]
			facing := game.BodyFacing(body)
			bodyLen := len(body)

			otherOcc := game.OccExcept(&allOcc, body)
			for i := range otherOcc.Bits {
				otherOcc.Bits[i] |= plannedHeads.Bits[i]
			}

			dirInfo := bot.CalcDirInfo(&state, body, facing, &otherOcc)
			_, myDists := state.FloodDist(head, &otherOcc)

			srcBG := game.NewBG(W, H)
			game.FillBG(&srcBG, sources)
			allCompetitive := bot.FiltSrc(&state, sources, myDists, enemyDists)
			plan := bot.InstantEat(&state, body, facing, allCompetitive, &srcBG, &otherOcc)
			isInstantEat := false
			reason := ""

			if plan.Ok {
				di := dirInfo[plan.Dir]
				if di != nil && di.Alive {
					isInstantEat = true
					reason = "eat"
				} else {
					altPlan := bot.InstantEat(&state, body, facing, sources, &srcBG, &otherOcc)
					if altPlan.Ok {
						altDi := dirInfo[altPlan.Dir]
						if altDi != nil && altDi.Alive {
							plan = altPlan
							isInstantEat = true
							reason = "eat"
						} else {
							plan.Ok = false
						}
					} else {
						plan.Ok = false
					}
				}
			}

			available := vsrc[botIdx]
			if len(available) == 0 {
				available = sources
			}
			competitive := bot.FiltSrc(&state, available, myDists, enemyDists)
			if len(competitive) == 0 {
				competitive = available
			}

			if !plan.Ok {
				game.FillBG(&srcBG, competitive)

				maxDepth := 8
				if bodyLen <= 5 {
					maxDepth = 12
				}
				remaining := time.Until(turnDeadline)
				if remaining < 15*time.Millisecond {
					maxDepth = 4
				} else if remaining < 25*time.Millisecond {
					maxDepth = 6
				}

				plan = bot.PathBFS(&state, body, facing, competitive, maxDepth, dirInfo, enemyDists, &srcBG, &otherOcc, turnDeadline)
				if plan.Ok {
					reason = "bfs"
				}

				if plan.Ok && !bot.IsSafeDir(plan.Dir, dirInfo, bodyLen) {
					if bs, ok := bot.BestSafeDir(dirInfo); ok && bot.IsSafeDir(bs, dirInfo, bodyLen) {
						plan.Dir = bs
						reason = "safe"
					}
				}
			}

			if !plan.Ok {
				if job, ok := supportJobs[b.ID]; ok {
					game.FillBG(&srcBG, sources)
					plan = bot.BestGroundAction(&state, body, facing, job.Cell, dirInfo, enemies, &srcBG, &otherOcc, &eDanger)
					if plan.Ok {
						reason = "support"
					}
				}
			}

			if !plan.Ok {
				game.FillBG(&srcBG, available)
				plan = bot.BestAction(&state, body, facing, available, dirInfo, enemies, enemyDists, &srcBG, &otherOcc, &eDanger)
				if plan.Ok {
					reason = "bmove"
				}
			}

			if !plan.Ok {
				plan.Dir = facing
				reason = "face"
			}

			nextHead := game.Add(head, game.DirDelta[plan.Dir])
			if grid.IsWall(nextHead) || otherOcc.Has(nextHead) {
				bestDir := game.DirNone
				bestFlood := -1
				for dir, di := range dirInfo {
					if !di.Alive {
						continue
					}
					t := game.Add(head, game.DirDelta[dir])
					if otherOcc.Has(t) {
						continue
					}
					if di.Flood > bestFlood {
						bestFlood = di.Flood
						bestDir = dir
					}
				}
				if bestDir != game.DirNone {
					plan.Dir = bestDir
					reason = "escape"
				}
			}

			if !isInstantEat {
				if di, ok := dirInfo[plan.Dir]; ok && di.Alive {
					if di.Flood < bodyLen+2 {
						if bs, ok := bot.BestSafeDir(dirInfo); ok && dirInfo[bs].Flood >= bodyLen*3 {
							plan.Dir = bs
							reason = "safe"
						}
					}
				}
			}

			plans = append(plans, bot.BotPlan{
				ID: b.ID, Body: append([]game.Point(nil), body...), Facing: facing,
				Dir: plan.Dir, Target: plan.Target, Reason: reason, Ok: plan.Ok,
			})
			if !plan.Ok {
				plans[len(plans)-1].Target = game.Add(head, game.DirDelta[plan.Dir])
			}
			plannedHeads.Set(game.Add(head, game.DirDelta[plan.Dir]))
		}

		bot.RefinePlans(&state, &rsc, mine, enemies, sources, plans, turnDeadline)

		var actions []string
		var marks []game.Point
		for _, p := range plans {
			if p.Ok {
				marks = append(marks, p.Target)
			}
			r := ""
			if debug {
				r = p.Reason
			}
			actions = append(actions, bot.ActionString(p.ID, p.Dir, r))
		}

		for i, m := range marks {
			if i >= 4 {
				break
			}
			actions = append(actions, fmt.Sprintf("MARK %d %d", m.X, m.Y))
		}

		out := "WAIT"
		if len(actions) > 0 {
			out = strings.Join(actions, ";")
		}
		if debug {
			fmt.Fprintln(os.Stderr, out)
		}
		fmt.Println(out)
		turn++
	}
}
