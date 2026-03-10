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
	width    int
	height   int
	walls    map[Point]bool
	sources  map[Point]bool
	occupied map[Point]bool
}

var dirDelta = map[string]Point{
	DirUp:    {0, -1},
	DirDown:  {0, 1},
	DirLeft:  {-1, 0},
	DirRight: {1, 0},
}

var allDirs = []string{DirUp, DirRight, DirDown, DirLeft}

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
	upPenalty := 0
	if target.y < head.y {
		upPenalty = (head.y - target.y) * 3
	}
	return dist(head, target) + upPenalty
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
		if world.walls[below] || world.occupied[below] {
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

	collision := world.walls[newHead] || world.occupied[newHead]
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
			if part.y < world.height+1 {
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

		for _, dir := range legalDirs(item.state.facing) {
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
		bestScore := sourceScore(next.body[0], bestTarget)
		for _, s := range sources[1:] {
			if score := sourceScore(next.body[0], s); score < bestScore {
				bestTarget = s
				bestScore = score
			}
		}
		if ate && sourceSet[eatenAt] {
			bestScore = -1000
			bestTarget = eatenAt
		}

		candidate := SearchResult{
			action: dir,
			target: bestTarget,
			score:  bestScore,
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
				width:    width,
				height:   height,
				walls:    walls,
				occupied: otherOccupied,
			}

			available := filterSources(sources, claimed)
			if len(available) == 0 {
				available = sources
			}

			plan := findPathAction(bot, world, available, 8)
			if !plan.ok {
				plan = findGreedyAction(bot, world, available)
			}
			if !plan.ok || plan.action == "" {
				plan.action = facingFromBody(bot.body)
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
