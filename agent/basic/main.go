package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
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
	danger   map[Point]bool // cells enemy heads can reach next turn
}

var dirDelta = map[string]Point{
	DirUp:    {0, -1},
	DirDown:  {0, 1},
	DirLeft:  {-1, 0},
	DirRight: {1, 0},
}

var allDirs = []string{DirUp, DirRight, DirDown, DirLeft}

// Precomputed grid data (built once during init, O(1) lookups)
var (
	gridW, gridH int
	wallGrid     [][]bool
	cellDirs     map[Point][]string // per free cell: directions that don't lead into walls or OOB
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
	// Mild penalty for sources above head (need wall support to climb, gravity fights us)
	if target.y < head.y {
		d += head.y - target.y
	}
	// Small bonus for sources on solid ground (easy to walk to)
	if isWall(Point{target.x, target.y + 1}) {
		d--
	}
	return d
}

// findInstantEat checks if a source is exactly 1 step away in a safe direction.
// Returns immediately without BFS if found.
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

// safeLegalDirs returns directions that don't go backward and don't hit walls.
// Uses precomputed cellDirs for O(1) wall filtering.
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

func findPathAction(bot Bot, world World, sources []Point, maxDepth int) SearchResult {
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

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		if item.depth >= maxDepth {
			continue
		}

		// Use safeLegalDirs: skip wall collisions (losing head is almost always bad)
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
				score := sourceScore(bot.body[0], eatenAt)
				candidate := SearchResult{
					action: first,
					target: eatenAt,
					steps:  item.depth + 1,
					score:  score,
					ok:     true,
				}
				if !best.ok || candidate.steps < best.steps || (candidate.steps == best.steps && candidate.score < best.score) {
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

func findGreedyAction(bot Bot, world World, sources []Point) SearchResult {
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

	for _, dir := range legalDirs(facing) {
		next, alive, ate, eatenAt := simulateMove(bot.body, facing, dir, world)
		if !alive {
			continue
		}

		bestTarget := sources[0]
		bestDist := sourceScore(next.body[0], bestTarget)
		for _, s := range sources[1:] {
			if score := sourceScore(next.body[0], s); score < bestDist {
				bestTarget = s
				bestDist = score
			}
		}

		score := bestDist
		if ate && sourceSet[eatenAt] {
			score = -1000
			bestTarget = eatenAt
		}

		// Penalize moves that cause head collision (scaled by body size)
		expectedLen := len(bot.body)
		if ate {
			expectedLen++
		}
		if len(next.body) < expectedLen {
			if len(bot.body) <= 5 {
				score += 1000 // losing head when small is devastating
			} else {
				score += 300
			}
		}

		// Penalize moving into enemy head danger zone (scaled by body size)
		if world.danger[next.body[0]] {
			if len(bot.body) <= 5 {
				score += 100 // risky when small
			} else {
				score += 20
			}
		}

		// Penalize moves that bounce back to the same position (no progress)
		if next.body[0] == bot.body[0] {
			score += 200
		}

		// Small bonus for positions near walls (better support, stay grounded)
		for _, wd := range allDirs {
			np := add(next.body[0], dirDelta[wd])
			if isWall(np) {
				score--
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

		// Compute enemy danger zones: cells any enemy head can reach next turn
		enemyDanger := make(map[Point]bool)
		for _, enemy := range enemies {
			head := enemy.body[0]
			facing := facingFromBody(enemy.body)
			for _, dir := range legalDirs(facing) {
				enemyDanger[add(head, dirDelta[dir])] = true
			}
		}

		sort.Slice(mine, func(i, j int) bool {
			return mine[i].id < mine[j].id
		})

		var actions []string
		claimed := make(map[Point]bool)

		for _, bot := range mine {
			otherOccupied := make(map[Point]bool, len(allOccupied))
			for p := range allOccupied {
				otherOccupied[p] = true
			}
			for _, p := range bot.body {
				delete(otherOccupied, p)
			}

			world := World{
				occupied: otherOccupied,
				danger:   enemyDanger,
			}

			available := filterSources(sources, claimed)
			if len(available) == 0 {
				available = sources
			}

			plan := findInstantEat(bot, world, available)
			if !plan.ok {
				maxDepth := 8
				if len(bot.body) <= 5 {
					maxDepth = 12 // search more carefully when small
				}
				plan = findPathAction(bot, world, available, maxDepth)
			}
			if !plan.ok {
				plan = findGreedyAction(bot, world, available)
			}
			if !plan.ok || plan.action == "" {
				plan.action = facingFromBody(bot.body)
			}

			// Collision avoidance: if planned move would collide, try safe alternatives
			head := bot.body[0]
			nextHead := add(head, dirDelta[plan.action])
			if isWall(nextHead) || otherOccupied[nextHead] {
				facing := facingFromBody(bot.body)
				for _, dir := range safeLegalDirs(head, facing) {
					target := add(head, dirDelta[dir])
					if otherOccupied[target] {
						continue
					}
					_, alive, _, _ := simulateMove(bot.body, facing, dir, world)
					if alive {
						plan.action = dir
						break
					}
				}
			}

			if len(available) > 0 {
				claimed[plan.target] = true
			}

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
