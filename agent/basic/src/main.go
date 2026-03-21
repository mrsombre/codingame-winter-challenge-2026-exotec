package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

const debug = false

var (
	scanner *bufio.Scanner
	grid    *AGrid
	state   State
	rsc     refScratch
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

func parseBody(s string) []Point {
	parts := strings.Split(s, ":")
	pts := make([]Point, len(parts))
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

	walls := make(map[Point]bool)
	for y := 0; y < H; y++ {
		row := readline()
		for x, ch := range row {
			if ch == '#' {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}
	grid = NewAG(W, H, walls)
	state = NewState(grid)
	rsc = newRefScratch(W, H)

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
		sources := make([]Point, srcN)
		for i := range sources {
			fmt.Sscan(readline(), &sources[i].X, &sources[i].Y)
		}

		var botN int
		fmt.Sscan(readline(), &botN)

		allOcc := NewBG(W, H)
		var mine []botEntry
		var enemies []enemyInfo

		for i := 0; i < botN; i++ {
			var id int
			var body string
			fmt.Sscan(readline(), &id, &body)
			pts := parseBody(body)
			for _, p := range pts {
				allOcc.Set(p)
			}
			f := DirUp
			if len(pts) >= 2 {
				f = FacingPts(pts[0], pts[1])
			}
			if myBots[id] {
				mine = append(mine, botEntry{id: id, body: pts})
			} else {
				enemies = append(enemies, enemyInfo{head: pts[0], facing: f, bodyLen: len(pts), body: pts})
			}
		}

		budget := 40 * time.Millisecond
		if turn == 0 {
			budget = 900 * time.Millisecond
		}
		turnDeadline := time.Now().Add(budget)

		enemyWalls := predictEnemyWalls(enemies, sources, &allOcc)
		eDanger := NewBG(W, H)
		for _, e := range enemies {
			ed, edn := validDirs(e.facing)
			for _, d := range ed[:edn] {
				eDanger.Set(Add(e.head, DirDelta[d]))
			}
		}

		enemyDists := calcEnemyDist(enemies, &allOcc)

		type sortEntry struct {
			idx     int
			minDist int
		}
		sortKeys := make([]sortEntry, len(mine))
		tmpDists := make([][]int, len(mine))
		for i, bot := range mine {
			occ := occExcept(&allOcc, bot.body)
			for j := range occ.Bits {
				occ.Bits[j] |= enemyWalls.Bits[j]
			}
			f := bodyFacing(bot.body)
			_, tmpDists[i] = cmdFlood(bot.body, f, &occ)
			md := Unreachable
			for _, s := range sources {
				if d := tmpDists[i][s.Y*W+s.X]; d < md {
					md = d
				}
			}
			sortKeys[i] = sortEntry{idx: i, minDist: md}
		}
		sort.Slice(sortKeys, func(i, j int) bool {
			if sortKeys[i].minDist != sortKeys[j].minDist {
				return sortKeys[i].minDist < sortKeys[j].minDist
			}
			return mine[sortKeys[i].idx].id < mine[sortKeys[j].idx].id
		})
		sortedMine := make([]botEntry, len(mine))
		botDists := make([][]int, len(mine))
		for i, sk := range sortKeys {
			sortedMine[i] = mine[sk.idx]
			botDists[i] = tmpDists[sk.idx]
		}
		mine = sortedMine

		vsrc := make([][]Point, len(mine))
		for _, s := range sources {
			si := s.Y*W + s.X
			bestBot := -1
			bestDist := Unreachable
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
		for i := range vsrc {
			bd := botDists[i]
			sort.Slice(vsrc[i], func(a, b int) bool {
				sa := vsrc[i][a].Y*W + vsrc[i][a].X
				sb := vsrc[i][b].Y*W + vsrc[i][b].X
				ma := (enemyDists[sa] - bd[sa]) - bd[sa]/4
				mb := (enemyDists[sb] - bd[sb]) - bd[sb]/4
				return ma > mb
			})
		}
		supportJobs := planSupportJobs(mine, vsrc, sources, botDists, turnDeadline)

		plans := make([]botPlan, 0, len(mine))
		plannedHeads := NewBG(W, H)

		for botIdx, bot := range mine {
			body := bot.body
			head := body[0]
			facing := bodyFacing(body)
			bodyLen := len(body)

			otherOcc := occExcept(&allOcc, body)
			for i := range otherOcc.Bits {
				otherOcc.Bits[i] |= plannedHeads.Bits[i]
				otherOcc.Bits[i] |= enemyWalls.Bits[i]
			}

			dirInfo := calcDirInfo(body, facing, &otherOcc)

			_, myDists := cmdFlood(body, facing, &otherOcc)

			srcBG := NewBG(W, H)
			fillBG(&srcBG, sources)
			allCompetitive := filtSrc(sources, myDists, enemyDists)
			plan := instantEat(body, facing, allCompetitive, &srcBG, &otherOcc)
			isInstantEat := false
			planReason := ""

			if plan.ok {
				di := dirInfo[plan.dir]
				if di != nil && di.alive {
					isInstantEat = true
					planReason = "eat"
				} else {
					altPlan := instantEat(body, facing, sources, &srcBG, &otherOcc)
					if altPlan.ok {
						altDi := dirInfo[altPlan.dir]
						if altDi != nil && altDi.alive {
							plan = altPlan
							isInstantEat = true
							planReason = "eat"
						} else {
							plan.ok = false
						}
					} else {
						plan.ok = false
					}
				}
			}

			available := vsrc[botIdx]
			if len(available) == 0 {
				available = sources
			}
			competitive := filtSrc(available, myDists, enemyDists)
			if len(competitive) == 0 {
				competitive = available
			}

			if !plan.ok {
				fillBG(&srcBG, competitive)

				maxDepth := 16
				if bodyLen <= 5 {
					maxDepth = 20
				}
				remaining := time.Until(turnDeadline)
				if remaining < 8*time.Millisecond {
					maxDepth = 4
				} else if remaining < 15*time.Millisecond {
					maxDepth = 6
				} else if remaining < 22*time.Millisecond {
					maxDepth = 10
				}

				plan = cmdBFS(body, facing, competitive, maxDepth, dirInfo, enemyDists, &srcBG, &otherOcc, turnDeadline)
				if plan.ok {
					planReason = "bfs"
				}

				if plan.ok && !isSafeDir(plan.dir, dirInfo, bodyLen) {
					if bs, ok := bestSafeDir(dirInfo); ok && isSafeDir(bs, dirInfo, bodyLen) {
						plan.dir = bs
						planReason = "safe"
					}
				}
			}

			if !plan.ok {
				if job, ok := supportJobs[bot.id]; ok {
					fillBG(&srcBG, sources)
					plan = bestGroundAction(body, facing, job.cell, dirInfo, enemies, &srcBG, &otherOcc, &eDanger)
					if plan.ok {
						planReason = "support"
					}
				}
			}

			if !plan.ok {
				fillBG(&srcBG, available)
				plan = bestAction(body, facing, available, dirInfo, enemies, enemyDists, &srcBG, &otherOcc, &eDanger)
				if plan.ok {
					planReason = "bmove"
				}
			}

			if !plan.ok {
				plan.dir = facing
				planReason = "face"
			}

			nextHead := Add(head, DirDelta[plan.dir])
			if grid.IsWall(nextHead) || otherOcc.Has(nextHead) {
				bestDir := DirNone
				bestFlood := -1
				for dir, di := range dirInfo {
					if !di.alive {
						continue
					}
					t := Add(head, DirDelta[dir])
					if otherOcc.Has(t) {
						continue
					}
					if di.flood > bestFlood {
						bestFlood = di.flood
						bestDir = dir
					}
				}
				if bestDir != DirNone {
					plan.dir = bestDir
					planReason = "escape"
				}
			}

			if !isInstantEat {
				if di, ok := dirInfo[plan.dir]; ok && di.alive {
					if di.flood < bodyLen+2 {
						if bs, ok := bestSafeDir(dirInfo); ok && dirInfo[bs].flood >= bodyLen*3 {
							plan.dir = bs
							planReason = "safe"
						}
					}
				}
			}

			// Head-lock rejection for long snakes
			if bodyLen >= 6 && isHeadLockedWorstCase(body, facing, plan.dir, enemies, &otherOcc, &srcBG) {
				replaced := false
				bestFlood := -1
				vd, nd := validDirs(facing)
				for _, d := range vd[:nd] {
					if d == plan.dir {
						continue
					}
					di := dirInfo[d]
					if di == nil || !di.alive {
						continue
					}
					if isHeadLockedWorstCase(body, facing, d, enemies, &otherOcc, &srcBG) {
						continue
					}
					if di.flood > bestFlood {
						bestFlood = di.flood
						plan.dir = d
						planReason = "nolock"
						replaced = true
					}
				}
				_ = replaced
			}

			if plan.ok {
				plans = append(plans, botPlan{id: bot.id, body: append([]Point(nil), body...), facing: facing, dir: plan.dir, target: plan.target, reason: planReason, ok: true})
			} else {
				plans = append(plans, botPlan{id: bot.id, body: append([]Point(nil), body...), facing: facing, dir: plan.dir, target: Add(head, DirDelta[plan.dir]), reason: planReason})
			}
			plannedHeads.Set(Add(head, DirDelta[plan.dir]))
		}

		// Opponent-aware plan refinement
		if time.Until(turnDeadline) > 10*time.Millisecond {
			predOcc := NewBG(W, H)
			copy(predOcc.Bits, allOcc.Bits)
			for _, e := range enemies {
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
				ph := Add(e.head, DirDelta[bestDir])
				if !grid.IsWall(ph) {
					predOcc.Set(ph)
				}
			}

			for i := range plans {
				if time.Until(turnDeadline) < 5*time.Millisecond {
					break
				}
				body := plans[i].body
				facing := plans[i].facing
				bodyLen := len(body)
				myOcc := occExcept(&predOcc, body)
				predDI := calcDirInfo(body, facing, &myOcc)
				di := predDI[plans[i].dir]
				if di != nil && di.alive && di.flood >= bodyLen*2 {
					continue
				}
				bestDir := plans[i].dir
				bestFlood := 0
				if di != nil && di.alive {
					bestFlood = di.flood
				}
				for dir, info := range predDI {
					if info != nil && info.alive && info.flood > bestFlood {
						bestFlood = info.flood
						bestDir = dir
					}
				}
				if bestDir != plans[i].dir && bestFlood >= bodyLen*2 {
					plans[i].dir = bestDir
					plans[i].reason = "pred"
				}
			}
		}

		refinePlansWithOneTurnSafety(&rsc, mine, enemies, sources, plans, turnDeadline)

		// Final guard: never emit a move that sends head into neck (body[1]).
		for i := range plans {
			body := plans[i].body
			if len(body) < 2 {
				continue
			}
			neck := body[1]
			nextHead := Add(body[0], DirDelta[plans[i].dir])
			if nextHead == neck {
				replaced := false
				for d := DirUp; d <= DirLeft; d++ {
					if d == plans[i].dir {
						continue
					}
					alt := Add(body[0], DirDelta[d])
					if alt == neck {
						continue
					}
					if !grid.IsWall(alt) {
						plans[i].dir = d
						plans[i].reason = "fix"
						replaced = true
						break
					}
				}
				if !replaced {
					for d := DirUp; d <= DirLeft; d++ {
						alt := Add(body[0], DirDelta[d])
						if alt != neck {
							plans[i].dir = d
							plans[i].reason = "fix"
							break
						}
					}
				}
			}
		}

		var actions []string
		for _, plan := range plans {
			actions = append(actions, actionString(plan.id, plan.dir, plan.reason))
		}

		if debug {
			n := 0
			for _, plan := range plans {
				if plan.ok && plan.target.X >= 0 && plan.target.Y >= 0 && n < 4 {
					actions = append(actions, fmt.Sprintf("MARK %d %d", plan.target.X, plan.target.Y))
					n++
				}
			}
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
