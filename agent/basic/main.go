package main

import (
	"bufio"
	"container/heap"
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
	upMinLen     [][]int
	supportCount int
	supportCompCount int
	supportNodes [MaxSupportNodes]SupportNode
	supportCompID [MaxSupportNodes]int
	supportNodeID [MaxGridH][MaxGridW]int
	anchorTargetMinLenCache = map[anchorTargetKey]int{}
	committedRoutes = map[int]RoutePlan{}
)

const (
	MaxGridW = 45
	MaxGridH = 30
	MaxSupportNodes = MaxGridW * MaxGridH
	noSupportNode = -1
	infDistance = 1 << 28
	maxRouteDepth = 40
	maxRouteExpansions = 5000
	maxAppleSequence = 2
	collisionLookahead = 8
)

type SupportNode struct {
	pos       Point
	neighbors [8]int
	deg       int
}

type RoutePlan struct {
	action   string
	target   Point
	eatTurn  int
	steps    int
	path     []string
	endState SimState
	ok       bool
}

type PlannedSequence struct {
	firstAction string
	firstTarget Point
	firstEatTurn int
	plans []RoutePlan
	ok bool
}

type distCacheKey struct {
	target Point
	bodyLen int
}

type anchorTargetKey struct {
	compID int
	target Point
}

type searchItem struct {
	state SimState
	first string
	path  []string
	depth int
	score int
	index int
}

type MoveEval struct {
	dir      string
	state    SimState
	alive    bool
	headLoss bool
	horizon  int
}

type searchHeap []*searchItem

func (h searchHeap) Len() int { return len(h) }
func (h searchHeap) Less(i, j int) bool {
	if h[i].score != h[j].score {
		return h[i].score < h[j].score
	}
	return h[i].depth < h[j].depth
}
func (h searchHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}
func (h *searchHeap) Push(x any) {
	item := x.(*searchItem)
	item.index = len(*h)
	*h = append(*h, item)
}
func (h *searchHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	item.index = -1
	*h = old[:n-1]
	return item
}

func isWall(p Point) bool {
	if p.x < 0 || p.x >= gridW || p.y < 0 || p.y >= gridH {
		return true
	}
	return wallGrid[p.y][p.x]
}

func precompute(w, h int, walls map[Point]bool) {
	gridW, gridH = w, h
	wallGrid = make([][]bool, h)
	upMinLen = make([][]int, h)
	anchorTargetMinLenCache = map[anchorTargetKey]int{}
	for y := 0; y < h; y++ {
		wallGrid[y] = make([]bool, w)
		upMinLen[y] = make([]int, w)
		for x := 0; x < w; x++ {
			wallGrid[y][x] = walls[Point{x, y}]
		}
	}
	precomputeUpMinLen()
	precomputeSupportNodes()
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

func precomputeSupportNodes() {
	supportCount = 0
	supportCompCount = 0
	for y := 0; y < MaxGridH; y++ {
		for x := 0; x < MaxGridW; x++ {
			supportNodeID[y][x] = noSupportNode
		}
	}

	for y := 0; y < gridH; y++ {
		for x := 0; x < gridW; x++ {
			p := Point{x, y}
			if isWall(p) || !hasStaticSupport(p) {
				continue
			}
			id := supportCount
			supportCount++
			supportNodes[id] = SupportNode{pos: p}
			supportNodeID[y][x] = id
		}
	}

	for id := 0; id < supportCount; id++ {
		p := supportNodes[id].pos
		node := SupportNode{pos: p}
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				np := Point{p.x + dx, p.y + dy}
				nid := supportNodeAt(np)
				if nid == noSupportNode {
					continue
				}
				node.neighbors[node.deg] = nid
				node.deg++
			}
		}
		supportNodes[id] = node
	}

	for id := 0; id < supportCount; id++ {
		supportCompID[id] = noSupportNode
	}
	queue := make([]int, 0, supportCount)
	for id := 0; id < supportCount; id++ {
		if supportCompID[id] != noSupportNode {
			continue
		}
		compID := supportCompCount
		supportCompCount++
		supportCompID[id] = compID
		queue = queue[:0]
		queue = append(queue, id)
		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			node := supportNodes[cur]
			for j := 0; j < node.deg; j++ {
				next := node.neighbors[j]
				if supportCompID[next] != noSupportNode {
					continue
				}
				supportCompID[next] = compID
				queue = append(queue, next)
			}
		}
	}
}

func hasStaticSupport(p Point) bool {
	return isWall(Point{p.x, p.y + 1})
}

func supportNodeAt(p Point) int {
	if p.x < 0 || p.x >= gridW || p.y < 0 || p.y >= gridH {
		return noSupportNode
	}
	return supportNodeID[p.y][p.x]
}

func precomputeUpMinLen() {
	for y := 0; y < gridH; y++ {
		for x := 0; x < gridW; x++ {
			if wallGrid[y][x] {
				upMinLen[y][x] = 0
				continue
			}
			upMinLen[y][x] = 9999
			for yy := y + 1; yy < gridH; yy++ {
				if wallGrid[yy][x] {
					upMinLen[y][x] = yy - y
					break
				}
			}
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

// gridBFSDistBodyAware is a conservative cheap-distance heuristic.
// It refuses to treat steep UP entries as normal BFS edges when the body is too short
// to remain vertically supported in that column on empty terrain.
func gridBFSDistBodyAware(start Point, blocked map[Point]bool, bodyLen int) map[Point]int {
	dists := make(map[Point]int, 128)
	if isWall(start) || blocked[start] {
		return dists
	}
	dists[start] = 0
	queue := make([]Point, 0, 128)
	queue = append(queue, start)
	for i := 0; i < len(queue); i++ {
		p := queue[i]
		d := dists[p]
		for _, dir := range allDirs {
			np := add(p, dirDelta[dir])
			if _, visited := dists[np]; visited {
				continue
			}
			if isWall(np) || blocked[np] {
				continue
			}
			if dir == DirUp && bodyLen < upMinLen[np.y][np.x] {
				continue
			}
			dists[np] = d + 1
			queue = append(queue, np)
		}
	}
	return dists
}

func supportEdgeMinLen(fromID, toID int) int {
	from := supportNodes[fromID].pos
	to := supportNodes[toID].pos
	dx := abs(to.x - from.x)
	dy := to.y - from.y
	switch {
	case dy >= 0:
		return 1
	case dy == -1 && dx <= 1:
		return 3
	default:
		return 9999
	}
}

func anchorSupportNode(body []Point) int {
	for _, part := range body {
		if hasStaticSupport(part) {
			if id := supportNodeAt(part); id != noSupportNode {
				return id
			}
		}
	}
	return noSupportNode
}

func anchorSupportComp(body []Point) int {
	if id := anchorSupportNode(body); id != noSupportNode {
		return supportCompID[id]
	}
	return noSupportNode
}

func minLenFromAnchorCompToTarget(anchorComp int, target Point) int {
	if anchorComp == noSupportNode || isWall(target) {
		return infDistance
	}
	key := anchorTargetKey{compID: anchorComp, target: target}
	if v, ok := anchorTargetMinLenCache[key]; ok {
		return v
	}

	type state struct {
		p   Point
		run int
	}

	maxLen := gridW + gridH
	if maxLen < 1 {
		maxLen = 1
	}

	for need := 1; need <= maxLen; need++ {
		visited := make([][][]bool, gridH)
		for y := 0; y < gridH; y++ {
			visited[y] = make([][]bool, gridW)
			for x := 0; x < gridW; x++ {
				visited[y][x] = make([]bool, need+1)
			}
		}

		queue := make([]state, 0, supportCount)
		for id := 0; id < supportCount; id++ {
			if supportCompID[id] != anchorComp {
				continue
			}
			p := supportNodes[id].pos
			visited[p.y][p.x][1] = true
			queue = append(queue, state{p: p, run: 1})
		}

		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			if cur.p == target {
				anchorTargetMinLenCache[key] = need
				return need
			}

			for _, dir := range allDirs {
				np := add(cur.p, dirDelta[dir])
				if isWall(np) {
					continue
				}
				nextRun := cur.run + 1
				if hasStaticSupport(np) {
					nextRun = 1
				}
				if nextRun > need || visited[np.y][np.x][nextRun] {
					continue
				}
				visited[np.y][np.x][nextRun] = true
				queue = append(queue, state{p: np, run: nextRun})
			}
		}
	}

	anchorTargetMinLenCache[key] = infDistance
	return infDistance
}

func appleApproachNodes(target Point) []int {
	var nodes []int
	seen := [MaxSupportNodes]bool{}
	if id := supportNodeAt(target); id != noSupportNode {
		nodes = append(nodes, id)
		seen[id] = true
	}
	for _, dir := range allDirs {
		p := add(target, dirDelta[dir])
		id := supportNodeAt(p)
		if id == noSupportNode || seen[id] {
			continue
		}
		nodes = append(nodes, id)
		seen[id] = true
	}
	if len(nodes) > 0 {
		return nodes
	}

	bestY := infDistance
	for id := 0; id < supportCount; id++ {
		p := supportNodes[id].pos
		if p.y < target.y || abs(p.x-target.x) > 1 {
			continue
		}
		if p.y < bestY {
			bestY = p.y
			nodes = nodes[:0]
			nodes = append(nodes, id)
			seen[id] = true
			continue
		}
		if p.y == bestY && !seen[id] {
			nodes = append(nodes, id)
			seen[id] = true
		}
	}
	if len(nodes) > 0 {
		return nodes
	}

	bestY = infDistance
	for id := 0; id < supportCount; id++ {
		p := supportNodes[id].pos
		if p.y < target.y || abs(p.x-target.x) > 2 {
			continue
		}
		if p.y < bestY {
			bestY = p.y
			nodes = nodes[:0]
			nodes = append(nodes, id)
			seen[id] = true
			continue
		}
		if p.y == bestY && !seen[id] {
			nodes = append(nodes, id)
			seen[id] = true
		}
	}
	return nodes
}

func supportDistances(target Point, bodyLen int, cache map[distCacheKey][]int) []int {
	key := distCacheKey{target: target, bodyLen: bodyLen}
	if d, ok := cache[key]; ok {
		return d
	}

	dists := make([]int, supportCount)
	for i := range dists {
		dists[i] = infDistance
	}

	startNodes := appleApproachNodes(target)
	queue := make([]int, 0, supportCount)
	for _, id := range startNodes {
		if dists[id] == 0 {
			continue
		}
		dists[id] = 0
		queue = append(queue, id)
	}

	for i := 0; i < len(queue); i++ {
		cur := queue[i]
		base := dists[cur]
		node := supportNodes[cur]
		for j := 0; j < node.deg; j++ {
			prev := node.neighbors[j]
			if bodyLen < supportEdgeMinLen(prev, cur) {
				continue
			}
			if base+1 >= dists[prev] {
				continue
			}
			dists[prev] = base + 1
			queue = append(queue, prev)
		}
	}

	cache[key] = dists
	return dists
}

func supportEstimate(state SimState, target Point, targetDists []int) int {
	estimate := dist(state.body[0], target)
	if target.y < state.body[0].y {
		estimate += state.body[0].y - target.y
	}

	anchor := anchorSupportNode(state.body)
	if anchor != noSupportNode && anchor < len(targetDists) {
		if d := targetDists[anchor]; d < infDistance {
			candidate := d * 2
			if candidate < estimate {
				estimate = candidate
			}
		}
	}

	return estimate
}

func planningWorld(sourceSet map[Point]bool) World {
	return World{
		sources:  sourceSet,
		occupied: map[Point]bool{},
	}
}

func planRouteToApple(start SimState, target Point, sourceSet map[Point]bool, bodyLen int, cache map[distCacheKey][]int, deadline time.Time) RoutePlan {
	anchorComp := anchorSupportComp(start.body)
	if target.y < start.body[0].y && bodyLen < minLenFromAnchorCompToTarget(anchorComp, target) {
		return RoutePlan{}
	}
	dists := supportDistances(target, bodyLen, cache)

	searchWorld := planningWorld(sourceSet)
	seen := map[string]int{stateKey(start): 0}
	pq := searchHeap{}
	heap.Init(&pq)
	heap.Push(&pq, &searchItem{
		state: start,
		depth: 0,
		score: supportEstimate(start, target, dists),
	})

	expansions := 0
	for pq.Len() > 0 {
		if time.Now().After(deadline) || expansions >= maxRouteExpansions {
			break
		}
		expansions++
		item := heap.Pop(&pq).(*searchItem)
		if item.depth >= maxRouteDepth {
			continue
		}
		if prev, ok := seen[stateKey(item.state)]; ok && prev < item.depth {
			continue
		}

		for _, dir := range safeLegalDirs(item.state.body[0], item.state.facing) {
			next, alive, ate, eatenAt := simulateMove(item.state.body, item.state.facing, dir, searchWorld)
			if !alive {
				continue
			}

			first := item.first
			if first == "" {
				first = dir
			}
			nextPath := append(append([]string(nil), item.path...), dir)
			depth := item.depth + 1

			if ate {
				if eatenAt == target {
					return RoutePlan{
						action:   first,
						target:   target,
						eatTurn:  depth,
						steps:    depth,
						path:     nextPath,
						endState: next,
						ok:       true,
					}
				}
				continue
			}

			key := stateKey(next)
			if prev, ok := seen[key]; ok && prev <= depth {
				continue
			}
			seen[key] = depth
			heap.Push(&pq, &searchItem{
				state: next,
				first: first,
				path:  nextPath,
				depth: depth,
				score: depth + supportEstimate(next, target, dists),
			})
		}
	}

	return RoutePlan{}
}

func chooseBestAppleRoute(start SimState, apples []Point, reserved map[Point]int, currentTurn int, cache map[distCacheKey][]int, deadline time.Time) RoutePlan {
	sourceSet := make(map[Point]bool, len(apples))
	for _, apple := range apples {
		sourceSet[apple] = true
	}

	best := RoutePlan{}
	bestScore := infDistance
	for _, apple := range apples {
		if _, ok := reserved[apple]; ok {
			continue
		}
		plan := planRouteToApple(start, apple, sourceSet, len(start.body), cache, deadline)
		if !plan.ok {
			continue
		}
		absEatTurn := currentTurn + plan.eatTurn
		score := absEatTurn*1000 + plan.steps*10 + dist(start.body[0], apple)
		if !best.ok || score < bestScore {
			best = plan
			best.eatTurn = absEatTurn
			bestScore = score
		}
	}
	return best
}

func buildAppleSequence(bot Bot, apples []Point, reserved map[Point]int, cache map[distCacheKey][]int, deadline time.Time) PlannedSequence {
	state := SimState{
		body:   cloneBody(bot.body),
		facing: facingFromBody(bot.body),
	}

	remaining := append([]Point(nil), apples...)
	currentTurn := 0
	sequence := PlannedSequence{}

	for len(sequence.plans) < maxAppleSequence && len(remaining) > 0 {
		plan := chooseBestAppleRoute(state, remaining, reserved, currentTurn, cache, deadline)
		if !plan.ok {
			break
		}
		if !sequence.ok {
			sequence.ok = true
			sequence.firstAction = plan.action
			sequence.firstTarget = plan.target
			sequence.firstEatTurn = plan.eatTurn
		}
		sequence.plans = append(sequence.plans, plan)

		nextRemaining := remaining[:0]
		for _, apple := range remaining {
			if apple != plan.target {
				nextRemaining = append(nextRemaining, apple)
			}
		}
		remaining = append([]Point(nil), nextRemaining...)

		state = plan.endState
		currentTurn = plan.eatTurn
	}

	return sequence
}

func sequenceFromRoute(plan RoutePlan) PlannedSequence {
	if !plan.ok || len(plan.path) == 0 {
		return PlannedSequence{}
	}
	return PlannedSequence{
		firstAction:  plan.path[0],
		firstTarget:  plan.target,
		firstEatTurn: plan.eatTurn,
		plans:        []RoutePlan{plan},
		ok:           true,
	}
}

func committedRouteAction(bot Bot, route RoutePlan, immediateWorld World) (string, RoutePlan, bool) {
	if !route.ok || len(route.path) == 0 {
		return "", RoutePlan{}, false
	}

	facing := facingFromBody(bot.body)
	for _, dir := range safeLegalDirs(bot.body[0], facing) {
		if dir != route.path[0] {
			continue
		}
		next, alive, _, _ := simulateMove(bot.body, facing, dir, immediateWorld)
		if !alive || len(next.body) < len(bot.body) {
			return "", RoutePlan{}, false
		}

		advanced := route
		advanced.path = append([]string(nil), route.path[1:]...)
		advanced.eatTurn--
		advanced.steps--
		if len(advanced.path) > 0 {
			advanced.action = advanced.path[0]
		} else {
			advanced.action = ""
		}
		return dir, advanced, true
	}

	return "", RoutePlan{}, false
}

func repeatedMoveHorizon(start SimState, dir string, world World, limit int) int {
	state := start
	horizon := 0
	for horizon < limit {
		next, alive, ate, _ := simulateMove(state.body, state.facing, dir, world)
		if !alive {
			break
		}
		expectedLen := len(state.body)
		if ate {
			expectedLen++
		}
		if len(next.body) < expectedLen {
			break
		}
		horizon++
		state = next
	}
	return horizon
}

func evaluateMoves(bot Bot, world World) []MoveEval {
	facing := facingFromBody(bot.body)
	candidates := safeLegalDirs(bot.body[0], facing)
	evals := make([]MoveEval, 0, len(candidates))
	for _, dir := range candidates {
		next, alive, ate, _ := simulateMove(bot.body, facing, dir, world)
		ev := MoveEval{dir: dir, state: next, alive: alive}
		if !alive {
			evals = append(evals, ev)
			continue
		}
		expectedLen := len(bot.body)
		if ate {
			expectedLen++
		}
		ev.headLoss = len(next.body) < expectedLen
		ev.horizon = repeatedMoveHorizon(next, dir, world, collisionLookahead)
		evals = append(evals, ev)
	}
	return evals
}

func bestCollisionAvoidingMove(evals []MoveEval) string {
	best := ""
	bestHorizon := -1
	for _, ev := range evals {
		if !ev.alive || ev.headLoss {
			continue
		}
		if ev.horizon > bestHorizon {
			bestHorizon = ev.horizon
			best = ev.dir
		}
	}
	return best
}

func jumpUpOrStay(bot Bot, evals []MoveEval) string {
	for _, ev := range evals {
		if ev.dir == DirUp && ev.alive && !ev.headLoss {
			return ev.dir
		}
	}
	for _, ev := range evals {
		if ev.dir == DirUp && ev.alive {
			return ev.dir
		}
	}
	if best := bestCollisionAvoidingMove(evals); best != "" {
		return best
	}
	facing := facingFromBody(bot.body)
	for _, ev := range evals {
		if ev.alive {
			return ev.dir
		}
	}
	return facing
}

func chooseThreeStageAction(bot Bot, preferred string, world World) string {
	evals := evaluateMoves(bot, world)
	if preferred != "" {
		for _, ev := range evals {
			if ev.dir != preferred {
				continue
			}
			if ev.alive && !ev.headLoss {
				return ev.dir
			}
			if best := bestCollisionAvoidingMove(evals); best != "" && best != preferred {
				return best
			}
			if ev.alive {
				return ev.dir
			}
			return jumpUpOrStay(bot, evals)
		}
		if best := bestCollisionAvoidingMove(evals); best != "" {
			return best
		}
	}
	return jumpUpOrStay(bot, evals)
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
				di.flood, _ = floodFillWithDist(next.body[0], blocked)
				di.dists = gridBFSDistBodyAware(next.body[0], blocked, len(next.body))
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
		eDists := gridBFSDistBodyAware(enemy.body[0], eBlocked, len(enemy.body))
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

		cache := make(map[distCacheKey][]int)
		currentSources := make(map[Point]bool, len(sources))
		for _, src := range sources {
			currentSources[src] = true
		}
		for id, route := range committedRoutes {
			if !currentSources[route.target] || len(route.path) == 0 {
				delete(committedRoutes, id)
			}
		}

		type botPlan struct {
			bot      Bot
			preview  PlannedSequence
			assigned PlannedSequence
			committed bool
		}

		plans := make([]botPlan, len(mine))
		reserved := make(map[Point]int)
		for i, bot := range mine {
			plans[i].bot = bot
			if route, ok := committedRoutes[bot.id]; ok {
				plans[i].preview = sequenceFromRoute(route)
				plans[i].assigned = plans[i].preview
				plans[i].committed = plans[i].assigned.ok
				if plans[i].assigned.ok {
					reserved[route.target] = route.eatTurn
					continue
				}
			}
			plans[i].preview = buildAppleSequence(bot, sources, reserved, cache, turnDeadline)
		}

		sort.Slice(plans, func(i, j int) bool {
			pi, pj := plans[i].preview, plans[j].preview
			if pi.ok != pj.ok {
				return pi.ok
			}
			if !pi.ok {
				return plans[i].bot.id < plans[j].bot.id
			}
			if pi.firstEatTurn != pj.firstEatTurn {
				return pi.firstEatTurn < pj.firstEatTurn
			}
			if pi.firstTarget != pj.firstTarget {
				if pi.firstTarget.x != pj.firstTarget.x {
					return pi.firstTarget.x < pj.firstTarget.x
				}
				return pi.firstTarget.y < pj.firstTarget.y
			}
			return plans[i].bot.id < plans[j].bot.id
		})

		for i := range plans {
			if plans[i].committed {
				continue
			}
			plans[i].assigned = buildAppleSequence(plans[i].bot, sources, reserved, cache, turnDeadline)
			if !plans[i].assigned.ok {
				continue
			}
			for _, step := range plans[i].assigned.plans {
				reserved[step.target] = step.eatTurn
			}
		}

		var actions []string
		plannedHeads := make(map[Point]bool)

		for _, entry := range plans {
			bot := entry.bot
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

			immediateWorld := World{
				sources:  make(map[Point]bool, len(sources)),
				occupied: otherOccupied,
			}
			for _, s := range sources {
				immediateWorld.sources[s] = true
			}

			preferred := ""
			status := "jump-up"
			target := "-"
			eatTurn := -1
			seqLen := 0
			var activeRoute RoutePlan
			activeRouteOK := false

			if route, ok := committedRoutes[bot.id]; ok && len(route.path) > 0 {
				preferred = route.path[0]
				activeRoute = route
				activeRouteOK = true
				status = "support-route"
				target = fmt.Sprintf("%d,%d", route.target.x, route.target.y)
				eatTurn = route.eatTurn
				seqLen = 1
			} else {
				delete(committedRoutes, bot.id)
				if entry.assigned.ok && len(entry.assigned.plans) > 0 && len(entry.assigned.plans[0].path) > 0 {
					activeRoute = entry.assigned.plans[0]
					activeRouteOK = true
					preferred = activeRoute.path[0]
					status = "support-route"
					target = fmt.Sprintf("%d,%d", entry.assigned.firstTarget.x, entry.assigned.firstTarget.y)
					eatTurn = entry.assigned.firstEatTurn
					seqLen = len(entry.assigned.plans)
				}
			}

			action := chooseThreeStageAction(bot, preferred, immediateWorld)

			if activeRouteOK && action == preferred {
				if len(activeRoute.path) > 1 {
					advanced := activeRoute
					advanced.path = append([]string(nil), activeRoute.path[1:]...)
					advanced.eatTurn--
					advanced.steps--
					advanced.action = advanced.path[0]
					committedRoutes[bot.id] = advanced
				} else {
					delete(committedRoutes, bot.id)
				}
			} else if activeRouteOK {
				delete(committedRoutes, bot.id)
				if status == "support-route" {
					status = "collision-avoid"
				}
			} else {
				delete(committedRoutes, bot.id)
			}

			if debug {
				fmt.Fprintf(os.Stderr, "bot %d decision=%s action=%s target=%s eatTurn=%d seq=%d\n",
					bot.id, status, action, target, eatTurn, seqLen)
			}

			plannedHeads[add(bot.body[0], dirDelta[action])] = true
			actions = append(actions, fmt.Sprintf("%d %s", bot.id, action))
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
