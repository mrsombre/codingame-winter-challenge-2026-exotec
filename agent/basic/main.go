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

var scanner *bufio.Scanner

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

const (
	DirUp    = "UP"
	DirDown  = "DOWN"
	DirLeft  = "LEFT"
	DirRight = "RIGHT"
)

type Point struct{ x, y int }

type Bot struct {
	id   int
	body []Point
}

type SimState struct {
	body   []Point
	facing string
}

type SearchResult struct {
	action string
	target Point
	steps  int
	score  int
	ok     bool
}

type World struct {
	sources  map[Point]bool
	occupied map[Point]bool
	danger   map[Point]bool
}

var dirDelta = map[string]Point{
	DirUp:    {0, -1},
	DirDown:  {0, 1},
	DirLeft:  {-1, 0},
	DirRight: {1, 0},
}

var allDirs = []string{DirUp, DirRight, DirDown, DirLeft}

var (
	gridW, gridH int
	wallGrid     [][]bool
	cellDirs     map[Point][]string
)

func isWall(p Point) bool {
	if p.x < 0 || p.x >= gridW || p.y < 0 || p.y >= gridH {
		return true
	}
	return wallGrid[p.y][p.x]
}

func precompute(w, h int, walls map[Point]bool) {
	gridW, gridH = w, h
	wallGrid = make([][]bool, h)
	for y := 0; y < h; y++ {
		wallGrid[y] = make([]bool, w)
		for x := 0; x < w; x++ {
			wallGrid[y][x] = walls[Point{x, y}]
		}
	}
	cellDirs = make(map[Point][]string)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if wallGrid[y][x] {
				continue
			}
			p := Point{x, y}
			var dirs []string
			for _, dir := range allDirs {
				np := add(p, dirDelta[dir])
				if !isWall(np) {
					dirs = append(dirs, dir)
				}
			}
			cellDirs[p] = dirs
		}
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func dist(a, b Point) int {
	return abs(a.x-b.x) + abs(a.y-b.y)
}

func parseBody(s string) []Point {
	parts := strings.Split(s, ":")
	pts := make([]Point, len(parts))
	for i, p := range parts {
		fmt.Sscanf(p, "%d,%d", &pts[i].x, &pts[i].y)
	}
	return pts
}

func add(a, b Point) Point {
	return Point{a.x + b.x, a.y + b.y}
}

func opposite(dir string) string {
	switch dir {
	case DirUp:
		return DirDown
	case DirDown:
		return DirUp
	case DirLeft:
		return DirRight
	case DirRight:
		return DirLeft
	default:
		return ""
	}
}

func facingFromBody(body []Point) string {
	if len(body) < 2 {
		return DirUp
	}
	dx := body[0].x - body[1].x
	dy := body[0].y - body[1].y
	switch {
	case dx == 1 && dy == 0:
		return DirRight
	case dx == -1 && dy == 0:
		return DirLeft
	case dx == 0 && dy == 1:
		return DirDown
	case dx == 0 && dy == -1:
		return DirUp
	default:
		return DirUp
	}
}

func cloneBody(body []Point) []Point {
	out := make([]Point, len(body))
	copy(out, body)
	return out
}

func makeSet(points []Point) map[Point]bool {
	out := make(map[Point]bool, len(points))
	for _, p := range points {
		out[p] = true
	}
	return out
}

func stateKey(state SimState) string {
	var b strings.Builder
	b.WriteString(state.facing)
	for _, p := range state.body {
		fmt.Fprintf(&b, "|%d,%d", p.x, p.y)
	}
	return b.String()
}

func filterSources(sources []Point, claimed map[Point]bool) []Point {
	if len(claimed) == 0 {
		return sources
	}
	out := make([]Point, 0, len(sources))
	for _, s := range sources {
		if !claimed[s] {
			out = append(out, s)
		}
	}
	return out
}

func sourceScore(head, target Point) int {
	d := dist(head, target)
	if target.y < head.y {
		d += head.y - target.y
	}
	if isWall(Point{target.x, target.y + 1}) {
		d--
	}
	return d
}

func safeLegalDirs(pos Point, facing string) []string {
	dirs := cellDirs[pos]
	if len(dirs) == 0 {
		return nil
	}
	back := opposite(facing)
	out := make([]string, 0, len(dirs))
	for _, d := range dirs {
		if d != back {
			out = append(out, d)
		}
	}
	return out
}

func legalDirs(facing string) []string {
	out := make([]string, 0, len(allDirs))
	back := opposite(facing)
	for _, dir := range allDirs {
		if dir == back {
			continue
		}
		out = append(out, dir)
	}
	return out
}

func containsSource(sourceSet map[Point]bool, p Point) bool {
	return sourceSet[p]
}

func hasSupport(body []Point, bodySet map[Point]bool, world World, eaten *Point) bool {
	for _, part := range body {
		below := Point{part.x, part.y + 1}
		if bodySet[below] {
			continue
		}
		if isWall(below) || world.occupied[below] {
			return true
		}
		if world.sources[below] && (eaten == nil || below != *eaten) {
			return true
		}
	}
	return false
}

func simulateMove(body []Point, facing, dir string, world World) (SimState, bool, bool, Point) {
	if dir == "" {
		dir = facing
	}
	if dir == "" {
		dir = DirUp
	}

	delta := dirDelta[dir]
	newHead := add(body[0], delta)
	willEat := containsSource(world.sources, newHead)

	nextBody := make([]Point, 0, len(body)+1)
	nextBody = append(nextBody, newHead)
	if willEat {
		nextBody = append(nextBody, body...)
	} else {
		nextBody = append(nextBody, body[:len(body)-1]...)
	}

	collision := isWall(newHead) || world.occupied[newHead]
	if !collision {
		for _, part := range nextBody[1:] {
			if part == newHead {
				collision = true
				break
			}
		}
	}

	if collision {
		if len(nextBody) <= 3 {
			return SimState{}, false, willEat, newHead
		}
		nextBody = cloneBody(nextBody[1:])
	}

	var eaten *Point
	if willEat {
		eaten = &newHead
	}

	for {
		bodySet := makeSet(nextBody)
		if hasSupport(nextBody, bodySet, world, eaten) {
			break
		}
		for i := range nextBody {
			nextBody[i].y++
		}
		allOut := true
		for _, part := range nextBody {
			if part.y < gridH+1 {
				allOut = false
				break
			}
		}
		if allOut {
			return SimState{}, false, willEat, newHead
		}
	}

	return SimState{
		body:   nextBody,
		facing: facingFromBody(nextBody),
	}, true, willEat, newHead
}

// floodFillWithDist performs BFS returning reachable count and per-cell distances.
func floodFillWithDist(start Point, blocked map[Point]bool) (int, map[Point]int) {
	dists := make(map[Point]int, 128)
	if isWall(start) || blocked[start] {
		return 0, dists
	}
	dists[start] = 0
	queue := make([]Point, 0, 128)
	queue = append(queue, start)
	count := 0
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		count++
		d := dists[p]
		for _, dir := range allDirs {
			np := add(p, dirDelta[dir])
			if _, visited := dists[np]; visited {
				continue
			}
			if isWall(np) || blocked[np] {
				continue
			}
			dists[np] = d + 1
			queue = append(queue, np)
		}
	}
	return count, dists
}

// gridBFSDist returns shortest grid distance from start to each reachable cell.
func gridBFSDist(start Point, blocked map[Point]bool) map[Point]int {
	_, dists := floodFillWithDist(start, blocked)
	return dists
}

// DirInfo holds precomputed data for a single movement direction.
type DirInfo struct {
	flood int
	dists map[Point]int // BFS distances from new head
	state SimState
	alive bool
	ate   bool
	eaten Point
}

// computeDirInfo precomputes simulation + flood fill + BFS for each safe legal direction.
func computeDirInfo(bot Bot, world World) map[string]*DirInfo {
	head := bot.body[0]
	facing := facingFromBody(bot.body)
	info := make(map[string]*DirInfo)

	for _, dir := range safeLegalDirs(head, facing) {
		next, alive, ate, eatenAt := simulateMove(bot.body, facing, dir, world)
		di := &DirInfo{
			alive: alive,
			ate:   ate,
			eaten: eatenAt,
		}
		if alive {
			di.state = next
			blocked := make(map[Point]bool, len(world.occupied)+len(next.body))
			for p := range world.occupied {
				blocked[p] = true
			}
			for _, p := range next.body[1:] {
				blocked[p] = true
			}
			di.flood, di.dists = floodFillWithDist(next.body[0], blocked)
		}
		info[dir] = di
	}
	return info
}

// isSafeDir checks if a direction has enough reachable space.
func isSafeDir(dir string, dirInfo map[string]*DirInfo, bodyLen int) bool {
	di, ok := dirInfo[dir]
	if !ok || !di.alive {
		return false
	}
	threshold := bodyLen * 2
	if threshold < 4 {
		threshold = 4
	}
	return di.flood >= threshold
}

// bestSafeDir returns the direction with highest flood fill.
func bestSafeDir(dirInfo map[string]*DirInfo) string {
	bestDir := ""
	bestFlood := -1
	for dir, di := range dirInfo {
		if di.alive && di.flood > bestFlood {
			bestFlood = di.flood
			bestDir = dir
		}
	}
	return bestDir
}

// filterCompetitiveSources removes sources where enemies are much closer by BFS distance.
func filterCompetitiveSources(sources []Point, myDists map[Point]int, enemyDists map[Point]int) []Point {
	out := make([]Point, 0, len(sources))
	for _, s := range sources {
		md, mok := myDists[s]
		ed, eok := enemyDists[s]
		if mok && eok && ed < md-3 {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return sources
	}
	return out
}

// computeEnemyMinDists returns minimum enemy grid distance to each source.
func computeEnemyMinDists(enemies []Bot, allOccupied map[Point]bool, sources []Point) map[Point]int {
	result := make(map[Point]int)
	for _, enemy := range enemies {
		eBlocked := make(map[Point]bool, len(allOccupied))
		for p := range allOccupied {
			eBlocked[p] = true
		}
		for _, p := range enemy.body {
			delete(eBlocked, p)
		}
		eDists := gridBFSDist(enemy.body[0], eBlocked)
		for _, s := range sources {
			if d, ok := eDists[s]; ok {
				if existing, exists := result[s]; !exists || d < existing {
					result[s] = d
				}
			}
		}
	}
	return result
}

func findInstantEat(bot Bot, world World, sources []Point) SearchResult {
	head := bot.body[0]
	facing := facingFromBody(bot.body)
	sourceSet := make(map[Point]bool, len(sources))
	for _, s := range sources {
		sourceSet[s] = true
	}
	world.sources = sourceSet

	var best SearchResult
	for _, dir := range safeLegalDirs(head, facing) {
		target := add(head, dirDelta[dir])
		if !sourceSet[target] {
			continue
		}
		_, alive, _, _ := simulateMove(bot.body, facing, dir, world)
		if !alive {
			continue
		}
		score := sourceScore(head, target)
		if !best.ok || score < best.score {
			best = SearchResult{action: dir, target: target, steps: 1, score: score, ok: true}
		}
	}
	return best
}

func findPathAction(bot Bot, world World, sources []Point, maxDepth int, dirInfo map[string]*DirInfo, enemyDists map[Point]int, deadline time.Time) SearchResult {
	if len(sources) == 0 {
		return SearchResult{}
	}

	sourceSet := make(map[Point]bool, len(sources))
	for _, s := range sources {
		sourceSet[s] = true
	}
	world.sources = sourceSet

	start := SimState{
		body:   cloneBody(bot.body),
		facing: facingFromBody(bot.body),
	}

	type queueItem struct {
		state SimState
		first string
		depth int
	}

	queue := []queueItem{{state: start}}
	seen := map[string]bool{stateKey(start): true}
	best := SearchResult{}
	iterations := 0

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxDepth {
			continue
		}

		// Check deadline every 256 iterations to avoid syscall overhead
		iterations++
		if iterations&255 == 0 && time.Now().After(deadline) {
			break
		}

		for _, dir := range safeLegalDirs(item.state.body[0], item.state.facing) {
			next, alive, ate, eatenAt := simulateMove(item.state.body, item.state.facing, dir, world)
			if !alive {
				continue
			}

			first := item.first
			if first == "" {
				first = dir
			}

			if ate && sourceSet[eatenAt] {
				rawSteps := item.depth + 1
				// Unified score: steps * 1000 + tiebreakers
				score := rawSteps * 1000
				score += sourceScore(bot.body[0], eatenAt)
				// Penalize first directions with dangerously low flood fill
				if di, ok := dirInfo[first]; ok && di.alive {
					if di.flood < len(bot.body)*2 {
						score += 3000 // equivalent to 3 extra steps
					} else if di.flood < len(bot.body)*3 {
						score += 1000
					}
				}
				// Strongly prefer sources where we beat the enemy
				if ed, ok := enemyDists[eatenAt]; ok {
					if rawSteps <= ed {
						score -= 300
					} else if rawSteps <= ed+2 {
						score += 500 // enemy is slightly closer, mild penalty
					} else {
						score += 2000 // enemy is much closer, avoid
					}
				}
				candidate := SearchResult{
					action: first,
					target: eatenAt,
					steps:  rawSteps,
					score:  score,
					ok:     true,
				}
				if !best.ok || candidate.score < best.score {
					best = candidate
				}
				continue
			}

			if best.ok && item.depth+1 >= best.steps {
				continue
			}

			key := stateKey(next)
			if seen[key] {
				continue
			}
			seen[key] = true
			queue = append(queue, queueItem{
				state: next,
				first: first,
				depth: item.depth + 1,
			})
		}
	}

	return best
}

func findGreedyAction(bot Bot, world World, sources []Point, dirInfo map[string]*DirInfo, enemies []Bot, enemyDists map[Point]int) SearchResult {
	if len(sources) == 0 {
		return SearchResult{action: DirUp, ok: true}
	}

	sourceSet := make(map[Point]bool, len(sources))
	for _, s := range sources {
		sourceSet[s] = true
	}
	world.sources = sourceSet

	best := SearchResult{}
	facing := facingFromBody(bot.body)
	bodyLen := len(bot.body)

	for _, dir := range legalDirs(facing) {
		next, alive, ate, eatenAt := simulateMove(bot.body, facing, dir, world)
		if !alive {
			continue
		}

		// Use BFS distances from DirInfo if available, else fall back to manhattan
		di := dirInfo[dir]
		bestTarget := sources[0]
		bestDist := 9999
		useBFS := di != nil && di.alive && di.dists != nil

		for _, s := range sources {
			var d int
			if useBFS {
				if bd, ok := di.dists[s]; ok {
					d = bd
				} else {
					d = 9999 // unreachable via BFS
				}
			} else {
				d = sourceScore(next.body[0], s)
			}
			if d < bestDist {
				bestDist = d
				bestTarget = s
			}
		}

		score := bestDist
		if ate && sourceSet[eatenAt] {
			score = -1000
			bestTarget = eatenAt
		}

		// Head collision penalty
		expectedLen := bodyLen
		if ate {
			expectedLen++
		}
		if len(next.body) < expectedLen {
			if bodyLen <= 5 {
				score += 1000
			} else {
				score += 300
			}
		}

		// Danger zone with size awareness
		if world.danger[next.body[0]] {
			dangerPenalty := 20
			if bodyLen <= 3 {
				dangerPenalty = 500
			} else if bodyLen <= 5 {
				dangerPenalty = 100
			}
			for _, enemy := range enemies {
				ehead := enemy.body[0]
				efacing := facingFromBody(enemy.body)
				canReach := false
				for _, edir := range legalDirs(efacing) {
					if add(ehead, dirDelta[edir]) == next.body[0] {
						canReach = true
						break
					}
				}
				if canReach && len(enemy.body) <= 3 && bodyLen > 3 {
					dangerPenalty = -500
				}
			}
			score += dangerPenalty
		}

		// No-progress penalty
		if next.body[0] == bot.body[0] {
			score += 200
		}

		// Wall proximity bonus
		for _, wd := range allDirs {
			np := add(next.body[0], dirDelta[wd])
			if isWall(np) {
				score--
			}
		}

		// Flood fill safety penalty
		if di != nil && di.alive {
			if di.flood < bodyLen {
				score += 2000
			} else if di.flood < bodyLen*2 {
				score += 500
			}
		} else if di == nil {
			score += 1500
		}

		// Enemy competition: penalize targets enemy will reach first
		if ed, ok := enemyDists[bestTarget]; ok {
			myD := bestDist
			if myD < 9999 && ed < myD-3 {
				score += 50
			}
		}

		candidate := SearchResult{
			action: dir,
			target: bestTarget,
			score:  score,
			ok:     true,
		}

		if !best.ok || candidate.score < best.score {
			best = candidate
		}
	}

	if best.ok {
		return best
	}

	return SearchResult{action: facing, ok: true}
}

func main() {
	scanner = bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1000000), 1000000)

	var myId int
	fmt.Sscan(readline(), &myId)

	var width, height int
	fmt.Sscan(readline(), &width)
	fmt.Sscan(readline(), &height)

	walls := make(map[Point]bool)
	for i := 0; i < height; i++ {
		row := readline()
		for x, ch := range row {
			if ch == '#' {
				walls[Point{x, i}] = true
			}
		}
	}

	precompute(width, height, walls)

	var snakebotsPerPlayer int
	fmt.Sscan(readline(), &snakebotsPerPlayer)

	myBots := make(map[int]bool)
	for i := 0; i < snakebotsPerPlayer; i++ {
		var id int
		fmt.Sscan(readline(), &id)
		myBots[id] = true
	}
	for i := 0; i < snakebotsPerPlayer; i++ {
		readline()
	}

	for {
		turnStart := time.Now()
		turnDeadline := turnStart.Add(45 * time.Millisecond)

		var powerSourceCount int
		fmt.Sscan(readline(), &powerSourceCount)

		sources := make([]Point, powerSourceCount)
		for i := 0; i < powerSourceCount; i++ {
			fmt.Sscan(readline(), &sources[i].x, &sources[i].y)
		}

		var snakebotCount int
		fmt.Sscan(readline(), &snakebotCount)

		var mine []Bot
		var enemies []Bot
		allOccupied := make(map[Point]bool)

		for i := 0; i < snakebotCount; i++ {
			var id int
			var body string
			fmt.Sscan(readline(), &id, &body)
			pts := parseBody(body)
			for _, p := range pts {
				allOccupied[p] = true
			}
			if myBots[id] {
				mine = append(mine, Bot{id: id, body: pts})
			} else {
				enemies = append(enemies, Bot{id: id, body: pts})
			}
		}

		// Enemy danger zones
		enemyDanger := make(map[Point]bool)
		for _, enemy := range enemies {
			head := enemy.body[0]
			facing := facingFromBody(enemy.body)
			for _, dir := range legalDirs(facing) {
				enemyDanger[add(head, dirDelta[dir])] = true
			}
		}

		// Enemy minimum distances to sources
		enemyDists := computeEnemyMinDists(enemies, allOccupied, sources)

		// Sort bots: closest to any source gets first pick of targets
		sort.Slice(mine, func(i, j int) bool {
			di, dj := 9999, 9999
			for _, s := range sources {
				if d := dist(mine[i].body[0], s); d < di {
					di = d
				}
				if d := dist(mine[j].body[0], s); d < dj {
					dj = d
				}
			}
			if di != dj {
				return di < dj
			}
			return mine[i].id < mine[j].id
		})

		var actions []string
		claimed := make(map[Point]bool)
		plannedHeads := make(map[Point]bool)

		for _, bot := range mine {
			otherOccupied := make(map[Point]bool, len(allOccupied)+len(plannedHeads))
			for p := range allOccupied {
				otherOccupied[p] = true
			}
			for p := range plannedHeads {
				otherOccupied[p] = true
			}
			for _, p := range bot.body {
				delete(otherOccupied, p)
			}

			world := World{
				occupied: otherOccupied,
				danger:   enemyDanger,
			}

			// Precompute per-direction info (simulation + flood fill + BFS distances)
			dirInfo := computeDirInfo(bot, world)

			// Filter sources: unclaimed, then competitive
			available := filterSources(sources, claimed)
			if len(available) == 0 {
				available = sources
			}
			myDists := gridBFSDist(bot.body[0], otherOccupied)
			competitive := filterCompetitiveSources(available, myDists, enemyDists)

			// Decision pipeline
			plan := findInstantEat(bot, world, competitive)

			// Check if instant eat direction is safe
			if plan.ok && !isSafeDir(plan.action, dirInfo, len(bot.body)) {
				altPlan := findInstantEat(bot, world, available)
				if altPlan.ok && isSafeDir(altPlan.action, dirInfo, len(bot.body)) {
					plan = altPlan
				} else {
					plan.ok = false
				}
			}

			if !plan.ok {
				maxDepth := 8
				if len(bot.body) <= 5 {
					maxDepth = 12
				}
				// Reduce depth if running low on time
				remaining := time.Until(turnDeadline)
				if remaining < 15*time.Millisecond {
					maxDepth = 4
				} else if remaining < 25*time.Millisecond {
					maxDepth = 6
				}
				// Single BFS with all sources (competition handled via scoring)
				plan = findPathAction(bot, world, available, maxDepth, dirInfo, enemyDists, turnDeadline)

				// Safety check: if BFS direction has dangerously low flood, pick safest
				if plan.ok && !isSafeDir(plan.action, dirInfo, len(bot.body)) {
					bs := bestSafeDir(dirInfo)
					if bs != "" && isSafeDir(bs, dirInfo, len(bot.body)) {
						plan.action = bs
					}
				}
			}
			if !plan.ok {
				plan = findGreedyAction(bot, world, available, dirInfo, enemies, enemyDists)
			}
			if !plan.ok || plan.action == "" {
				plan.action = facingFromBody(bot.body)
			}

			// Final collision avoidance using dirInfo
			head := bot.body[0]
			nextHead := add(head, dirDelta[plan.action])
			if isWall(nextHead) || otherOccupied[nextHead] {
				bestDir := ""
				bestFlood := -1
				for dir, di := range dirInfo {
					if !di.alive {
						continue
					}
					target := add(head, dirDelta[dir])
					if otherOccupied[target] {
						continue
					}
					if di.flood > bestFlood {
						bestFlood = di.flood
						bestDir = dir
					}
				}
				if bestDir != "" {
					plan.action = bestDir
				}
			}

			// Safety override: only when chosen direction is dangerously low
			if di, ok := dirInfo[plan.action]; ok && di.alive {
				if di.flood < len(bot.body)+2 {
					bs := bestSafeDir(dirInfo)
					if bs != "" && dirInfo[bs].flood >= len(bot.body)*3 {
						plan.action = bs
					}
				}
			}

			if len(available) > 0 {
				claimed[plan.target] = true
			}

			// Track planned head so subsequent allied bots avoid this cell
			plannedHeads[add(bot.body[0], dirDelta[plan.action])] = true

			actions = append(actions, fmt.Sprintf("%d %s", bot.id, plan.action))
		}

		var output string
		if len(actions) == 0 {
			output = "WAIT"
		} else {
			output = strings.Join(actions, ";")
		}
		if debug {
			fmt.Fprintln(os.Stderr, output)
		}
		fmt.Println(output)
	}
}
