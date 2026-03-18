package main

import "sort"

// Sim holds simulation state and provides engine-compatible
// movement, collision, and gravity resolution.
type Sim struct {
	G            *Game
	appleMap     []bool
	appleMapPrev []int
	appleMapN    int
	moveBuf      []int // scratch for simulateMove

	// obstacle map for SurfBFS (other-snake bodies)
	obstacleMap     []bool
	obstacleMapPrev []int
	obstacleMapN    int
}

func NewSim(g *Game) *Sim {
	n := g.NCells
	return &Sim{
		G:               g,
		appleMap:        make([]bool, n),
		appleMapPrev:    make([]int, MaxAp),
		moveBuf:         make([]int, MaxSeg),
		obstacleMap:     make([]bool, n),
		obstacleMapPrev: make([]int, MaxASn*MaxSeg),
	}
}

// --- Apple map ---

// RebuildAppleMap refreshes the apple bitmap from current game state.
func (s *Sim) RebuildAppleMap() {
	s.rebuildAppleMapFrom(s.G.Ap[:s.G.ANum])
}

func (s *Sim) rebuildAppleMapFrom(apples []int) {
	n := len(s.appleMap)
	for i := 0; i < s.appleMapN; i++ {
		s.appleMap[s.appleMapPrev[i]] = false
	}
	s.appleMapN = 0
	for _, ap := range apples {
		if ap >= 0 && ap < n {
			s.appleMap[ap] = true
			s.appleMapPrev[s.appleMapN] = ap
			s.appleMapN++
		}
	}
}

// isApple returns true if cell c contains an apple. Returns false for OOB.
func (s *Sim) isApple(c int) bool {
	if !s.G.IsInGrid(c) {
		return false
	}
	return s.appleMap[c]
}

// --- Ground checks ---

// isGroundedAt returns true if cell c has solid ground directly below it
// (wall, apple, or grid bottom). Returns false for OOB cells.
func (s *Sim) isGroundedAt(c int) bool {
	g := s.G
	if !g.IsInGrid(c) {
		return false
	}
	_, y := g.XY(c)
	if y+1 >= g.H {
		return true
	}
	below := c + g.Stride
	return !g.Cell[below] || s.appleMap[below]
}

// --- Movement simulation ---

// neckOf returns the cell index of body[1] (neck), or -1 if not available.
func neckOf(body []int) int {
	if len(body) > 1 && body[1] >= 0 {
		return body[1]
	}
	return -1
}

func bodyContains(body []int, cell int) bool {
	for _, part := range body {
		if part == cell {
			return true
		}
	}
	return false
}

// simulateMove applies engine-compatible movement for a single snake against
// static walls, apples, and its own body. It resolves the move and beheading
// but does not apply falling.
func (s *Sim) simulateMove(body []int, d int) ([]int, bool) {
	g := s.G
	if len(body) == 0 {
		return nil, false
	}

	hx, hy := g.XY(body[0])
	nx := hx + Dl[d][0]
	ny := hy + Dl[d][1]
	newHead := g.Idx(nx, ny)

	eating := s.isApple(newHead)
	newLen := len(body)
	if eating && newLen < MaxSeg {
		newLen++
	}

	newBody := s.moveBuf[:newLen]
	newBody[0] = newHead
	if eating {
		copy(newBody[1:], body)
	} else if len(body) > 1 {
		copy(newBody[1:], body[:len(body)-1])
	}

	hitWall := g.IsInGrid(newHead) && !g.Cell[newHead]
	hitBody := false
	if newHead >= 0 {
		for i := 1; i < newLen; i++ {
			if newBody[i] == newHead {
				hitBody = true
				break
			}
		}
	}

	if hitWall || hitBody {
		if newLen <= 3 {
			return nil, false
		}
		return newBody[1:newLen], true
	}

	return newBody, true
}

func (s *Sim) stepCell(cell int, d int) int {
	if cell < 0 {
		return -1
	}
	x, y := s.G.XY(cell)
	return s.G.Idx(x+Dl[d][0], y+Dl[d][1])
}

// --- Multi-snake resolution ---

func (s *Sim) hasTileOrAppleUnder(cell int, eaten []bool) bool {
	if cell < 0 {
		return false
	}
	below := s.stepCell(cell, DD)
	if s.G.IsInGrid(below) {
		if !s.G.Cell[below] {
			return true
		}
		return s.appleMap[below] && !eaten[below]
	}
	return false
}

func (s *Sim) isGroundedForResolve(body []int, snakes []Snake, grounded []bool, eaten []bool) bool {
	for _, cell := range body {
		if s.hasTileOrAppleUnder(cell, eaten) {
			return true
		}
		below := s.stepCell(cell, DD)
		if below < 0 {
			continue
		}
		for i := range snakes {
			if !grounded[i] || !snakes[i].Alive {
				continue
			}
			if bodyContains(snakes[i].Body, below) {
				return true
			}
		}
	}
	return false
}

func (s *Sim) allOutOfBounds(body []int) bool {
	for _, cell := range body {
		if cell < 0 {
			continue
		}
		_, y := s.G.XY(cell)
		if y < s.G.H+1 {
			return false
		}
	}
	return true
}

// resolveMove resolves engine-compatible post-move state for all snakes and
// returns the cells of apples eaten by moved heads this turn.
func (s *Sim) resolveMove(snakes []Snake) []int {
	if len(snakes) == 0 {
		return nil
	}

	s.RebuildAppleMap()
	eaten := make([]bool, s.G.NCells)
	eatenList := make([]int, 0, len(snakes))
	for i := range snakes {
		if !snakes[i].Alive || len(snakes[i].Body) == 0 {
			continue
		}
		head := snakes[i].Body[0]
		if s.G.IsInGrid(head) && s.appleMap[head] {
			if !eaten[head] {
				eaten[head] = true
				eatenList = append(eatenList, head)
			}
		}
	}

	toBehead := make([]bool, len(snakes))
	for i := range snakes {
		sn := &snakes[i]
		if !sn.Alive || len(sn.Body) == 0 {
			continue
		}

		head := sn.Body[0]
		isInWall := s.G.IsInGrid(head) && !s.G.Cell[head]
		isInBody := false

		for j := range snakes {
			other := &snakes[j]
			if !other.Alive || len(other.Body) == 0 || !bodyContains(other.Body, head) {
				continue
			}
			if i != j {
				isInBody = true
				break
			}
			if bodyContains(other.Body[1:], head) {
				isInBody = true
				break
			}
		}

		if isInWall || isInBody {
			toBehead[i] = true
		}
	}

	for i := range snakes {
		if !toBehead[i] {
			continue
		}
		if len(snakes[i].Body) <= 3 {
			snakes[i].Alive = false
			snakes[i].Body = nil
			snakes[i].Len = 0
			continue
		}
		snakes[i].Body = snakes[i].Body[1:]
		snakes[i].Len = len(snakes[i].Body)
	}

	airborne := make([]bool, len(snakes))
	grounded := make([]bool, len(snakes))
	for i := range snakes {
		if snakes[i].Alive && len(snakes[i].Body) > 0 {
			airborne[i] = true
		}
	}

	for {
		somethingFell := false
		for {
			somethingGotGrounded := false
			for i := range snakes {
				if !airborne[i] {
					continue
				}
				if s.isGroundedForResolve(snakes[i].Body, snakes, grounded, eaten) {
					grounded[i] = true
					airborne[i] = false
					somethingGotGrounded = true
				}
			}
			if !somethingGotGrounded {
				break
			}
		}

		for i := range snakes {
			if !airborne[i] {
				continue
			}
			somethingFell = true
			for bi, cell := range snakes[i].Body {
				snakes[i].Body[bi] = s.stepCell(cell, DD)
			}
			if s.allOutOfBounds(snakes[i].Body) {
				snakes[i].Alive = false
				snakes[i].Body = nil
				snakes[i].Len = 0
				airborne[i] = false
			}
		}

		if !somethingFell {
			break
		}
	}

	for i := range snakes {
		if snakes[i].Alive {
			snakes[i].Len = len(snakes[i].Body)
		}
	}
	return eatenList
}

// --- Gravity ---

// applyGravity drops body segments until grounded. Returns false if fell off map.
func (s *Sim) applyGravity(body []int) bool {
	g := s.G
	for iter := 0; iter < g.H; iter++ {
		for _, c := range body {
			if c >= 0 && s.isGroundedAt(c) {
				return true
			}
		}
		allOOB := true
		for i, c := range body {
			if c < 0 {
				continue
			}
			cx, cy := g.XY(c)
			nc := g.Idx(cx, cy+1)
			if nc < 0 {
				return false
			}
			body[i] = nc
			if g.IsInGrid(nc) {
				allOOB = false
			}
		}
		if allOOB {
			return false
		}
	}
	return false
}

// --- SimBFS ---

const (
	simMaxEatDepth     = 2 // apple eating grows body only within this many steps
	simAppleMaxDepth   = 7 // max depth for SimBFSApples
	simAppleMaxTargets = 8 // max apple targets for SimBFSApples
)

// SimTarget describes a physically reachable apple found by SimBFSApples.
type SimTarget struct {
	Apple    int // apple cell index
	Dist     int // steps to reach
	FirstDir int // direction of first move
	Eaten    int // total apples eaten on path (including this one)
}

type simNode struct {
	body     []int
	dist     int
	firstDir int
	eaten    int
}

func bodyHash(body []int) uint64 {
	h := uint64(14695981039346656037)
	h ^= uint64(len(body))
	h *= 1099511628211
	for _, c := range body {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}

// SimBFSApples searches for reachable apples using body simulation with
// obstacle awareness (other snakes blocked). Depth capped at simAppleMaxDepth.
func (s *Sim) SimBFSApples(sn *Snake) []SimTarget {
	g := s.G
	if sn == nil || !sn.Alive || sn.Len == 0 {
		return nil
	}
	head := sn.Body[0]
	if !g.IsInGrid(head) {
		return nil
	}

	s.buildObstacleMap(sn.ID)

	startBody := append([]int(nil), sn.Body...)
	visited := make(map[uint64]bool)
	visited[bodyHash(startBody)] = true
	queue := []simNode{{
		body:     startBody,
		firstDir: -1,
	}}
	var targets []SimTarget

	for qi := 0; qi < len(queue) && len(targets) < simAppleMaxTargets; qi++ {
		cur := queue[qi]
		if cur.dist >= simAppleMaxDepth {
			continue
		}
		curHead := cur.body[0]
		if !g.IsInGrid(curHead) {
			continue
		}
		neck := neckOf(cur.body)

		for dir := 0; dir < 4; dir++ {
			nc := g.Nbm[curHead][dir]
			if nc == -1 || nc == neck {
				continue
			}

			newBody, alive := s.simulateMove(cur.body, dir)
			if !alive {
				continue
			}

			// Reject beheading moves
			if len(newBody) < len(cur.body) {
				continue
			}

			bodycp := append([]int(nil), newBody...)

			eating := s.isApple(nc)
			expectedLen := len(cur.body)
			if eating {
				expectedLen++
			}
			if len(bodycp) < expectedLen {
				continue
			}

			if eating && cur.dist >= simMaxEatDepth {
				bodycp = bodycp[:len(bodycp)-1]
			}

			// Remove eaten apple from map so gravity doesn't use it as ground
			if eating {
				s.appleMap[nc] = false
			}
			gravOk := s.applyGravity(bodycp)
			if eating {
				s.appleMap[nc] = true // restore for other BFS branches
			}
			if !gravOk {
				continue
			}

			// Check obstacles on settled body
			blocked := false
			for _, c := range bodycp {
				if c >= 0 && c < g.NCells && s.obstacleMap[c] {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}

			fd := cur.firstDir
			if fd == -1 {
				fd = dir
			}
			newDist := cur.dist + 1
			newEaten := cur.eaten

			h := bodyHash(bodycp)
			if visited[h] {
				continue
			}
			visited[h] = true

			if eating {
				newEaten++
				targets = append(targets, SimTarget{
					Apple:    nc,
					Dist:     newDist,
					FirstDir: fd,
					Eaten:    newEaten,
				})
			}

			queue = append(queue, simNode{
				body:     bodycp,
				dist:     newDist,
				firstDir: fd,
				eaten:    newEaten,
			})
		}
	}
	return targets
}

// --- SurfBFS ---

const surfMaxDepth = 8 // max commands (moves issued)

// SurfReach describes a physically reachable surface found by SurfBFS.
type SurfReach struct {
	SurfID   int   // g.Surfs index
	Dist     int   // number of commands to reach surface
	FirstDir int   // direction of first move
	Landing  int   // head cell when landing on surface
	Dirs     []int // full sequence of directions taken (len == Dist)
	Heads    []int // head cell after each step (len == Dist)
}

type surfNode struct {
	body     []int
	dist     int
	firstDir int // -1 at start
	prevDir  int // direction taken to reach this node
	dirs     []int
	heads    []int
}

// isTailMovable returns true if a snake's tail will vacate next turn
// (i.e. it won't eat an apple this turn).
func (s *Sim) isTailMovable(sn *Snake) bool {
	head := sn.Body[0]
	if head < 0 || head >= s.G.NCells {
		return true
	}
	for d := 0; d < 4; d++ {
		nc := s.G.Nbm[head][d]
		if nc >= 0 && s.isApple(nc) {
			return false
		}
	}
	return true
}

// buildObstacleMap marks other-snake body cells as obstacles, skipping movable tails.
func (s *Sim) buildObstacleMap(excludeID int) {
	for i := 0; i < s.obstacleMapN; i++ {
		s.obstacleMap[s.obstacleMapPrev[i]] = false
	}
	s.obstacleMapN = 0
	g := s.G
	for i := 0; i < g.SNum; i++ {
		other := &g.Sn[i]
		if other.ID == excludeID || !other.Alive {
			continue
		}
		tailIdx := other.Len - 1
		movable := s.isTailMovable(other)
		for bi, c := range other.Body {
			if c >= 0 && c < g.NCells {
				if bi == tailIdx && movable {
					continue
				}
				s.obstacleMap[c] = true
				s.obstacleMapPrev[s.obstacleMapN] = c
				s.obstacleMapN++
			}
		}
	}
}

const surfTailKeep = 8 // segments to keep after support point

// SurfBFS finds shortest body-simulation paths from sn's current position
// to reachable surfaces, treating other snakes as static obstacles.
// Long snakes are shrunk: body[0..Sp+surfTailKeep] is simulated,
// remaining segments become static walls.
// Returns one SurfReach per reachable surface, sorted by Dist ascending.
func (s *Sim) SurfBFS(sn *Snake) []SurfReach {
	g := s.G
	if sn == nil || !sn.Alive || sn.Len == 0 {
		return nil
	}
	head := sn.Body[0]
	if !g.IsInGrid(head) {
		return nil
	}

	s.buildObstacleMap(sn.ID)

	// shrink long snakes: keep head..Sp + surfTailKeep segments after Sp
	// excess segments become obstacles
	cutoff := sn.Len
	if sn.Sp >= 0 {
		cutoff = sn.Sp + 1 + surfTailKeep
	} else {
		cutoff = surfTailKeep
	}
	if cutoff > sn.Len {
		cutoff = sn.Len
	}
	for i := cutoff; i < sn.Len; i++ {
		c := sn.Body[i]
		if c >= 0 && c < g.NCells {
			s.obstacleMap[c] = true
			s.obstacleMapPrev[s.obstacleMapN] = c
			s.obstacleMapN++
		}
	}

	startBody := append([]int(nil), sn.Body[:cutoff]...)
	visited := make(map[uint64]bool)
	visited[bodyHash(startBody)] = true

	queue := []surfNode{{
		body:     startBody,
		dist:     0,
		firstDir: -1,
		prevDir:  sn.Dir,
	}}

	bestSurf := make(map[int]SurfReach)

	for qi := 0; qi < len(queue); qi++ {
		cur := queue[qi]
		if cur.dist >= surfMaxDepth {
			continue
		}
		curHead := cur.body[0]
		if !g.IsInGrid(curHead) {
			continue
		}

		for dir := 0; dir < 4; dir++ {
			// reject backward
			if dir == Do[cur.prevDir] {
				continue
			}

			newBody, alive := s.simulateMove(cur.body, dir)
			if !alive {
				continue
			}

			// Reject beheading moves — never plan to lose segments
			if len(newBody) < len(cur.body) {
				continue
			}

			bodycp := append([]int(nil), newBody...)

			if !s.applyGravity(bodycp) {
				continue
			}

			// obstacle check on settled body
			blocked := false
			for _, c := range bodycp {
				if c >= 0 && c < g.NCells && s.obstacleMap[c] {
					blocked = true
					break
				}
			}
			if blocked {
				continue
			}

			h := bodyHash(bodycp)
			if visited[h] {
				continue
			}
			visited[h] = true

			fd := cur.firstDir
			if fd == -1 {
				fd = dir
			}
			newDist := cur.dist + 1
			newDirs := append(append([]int(nil), cur.dirs...), dir)
			newHead := bodycp[0]
			newHeads := append(append([]int(nil), cur.heads...), newHead)

			// check if head landed on a surface
			if g.IsInGrid(newHead) {
				sid := g.SurfAt[newHead]
				if sid >= 0 && g.Surfs[sid].Type != SurfNone {
					if _, ok := bestSurf[sid]; !ok {
						bestSurf[sid] = SurfReach{
							SurfID:   sid,
							Dist:     newDist,
							FirstDir: fd,
							Landing:  newHead,
							Dirs:     newDirs,
							Heads:    newHeads,
						}
					}
					continue // don't enqueue surface hits
				}
			}

			queue = append(queue, surfNode{
				body:     bodycp,
				dist:     newDist,
				firstDir: fd,
				prevDir:  dir,
				dirs:     newDirs,
				heads:    newHeads,
			})
		}
	}

	results := make([]SurfReach, 0, len(bestSurf))
	for _, sr := range bestSurf {
		results = append(results, sr)
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Dist < results[j].Dist
	})
	return results
}
