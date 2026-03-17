package main

const MaxChainEaten = 5 // max apples tracked along a BFS path

// PathResult holds the output of a BFS search for a single target.
type PathResult struct {
	Dist     int // BFS steps to reach target (-1 if unreachable)
	FirstDir int // direction of the first move (DU/DR/DD/DL)
	Apples   int // apples eaten along the BFS path to this cell
}

type bfsNode struct {
	cell     int16
	ag       int8 // above-ground counter (0 = grounded)
	firstDir int8 // direction of the very first step (-1 at start)
	dist     int16
	eaten    int8 // apples consumed along this path
}

// Surface represents a horizontal platform segment where snakes can walk.
type Surface struct {
	ID    int
	Y     int // y of the walkable cells
	Left  int // leftmost x
	Right int // rightmost x
}

// SurfLink represents a directed connection between two surfaces.
type SurfLink struct {
	To      int // target surface index
	Cost    int // number of turns to traverse
	MinBody int // minimum body length needed (0 = no constraint)
}

// Plan holds precomputed data and reusable BFS buffers.
type Plan struct {
	g *Game

	// Static precomputation (set once by Precompute)
	LandY []int // y-coordinate where a free-falling cell lands

	// Surface graph
	Surfs    []Surface
	SurfAt   []int        // cell → surface index (-1 if not on surface)
	SurfAdj  [][]SurfLink // adjacency list
	fallSeen []bool       // scratch for addFallLink dedup (indexed by surface ID)

	// All-pairs surface distances (Floyd-Warshall, per body length)
	surfN   int     // len(Surfs), cached
	surfAPD [][]int // [bodyLen] flat N×N matrix; [from*surfN+to]

	// Per-turn apple bitmap (rebuilt by RebuildAppleMap)
	appleMap     []bool
	appleMapPrev []int // previous apple positions for fast clear
	appleMapN    int   // count of previous entries

	// BFS reuse buffers
	visitGen     []uint16 // flat [(cell*MaxAG+ag)*(MaxChainEaten+1)+eaten]
	curGen       uint16
	queue        []bfsNode
	firstMoveBuf []int // scratch for single-snake move simulation
}

// Precompute fills LandY, surfaces, and the surface graph.
// Called once after Init (within the 1s first-turn budget).
func (p *Plan) Precompute() {
	n := p.g.W * p.g.H
	p.LandY = make([]int, n)
	p.SurfAt = make([]int, n)
	p.appleMap = make([]bool, n)
	p.appleMapPrev = make([]int, MaxAp)
	p.visitGen = make([]uint16, p.g.NCells*MaxAG*(MaxChainEaten+1))
	p.firstMoveBuf = make([]int, MaxSeg)
	p.computeLandY()
	p.detectSurfaces()
	p.buildSurfaceGraph()
	p.precomputeAllPairsSurfDist()
	if p.queue == nil {
		p.queue = make([]bfsNode, 0, n*4)
	}
}

// RebuildAppleMap refreshes the apple bitmap from current game state.
// Must be called before BFS each turn.
func (p *Plan) RebuildAppleMap() {
	p.rebuildAppleMapFrom(p.g.Ap[:p.g.ANum])
}

func (p *Plan) rebuildAppleMapFrom(apples []int) {
	n := len(p.appleMap)
	for i := 0; i < p.appleMapN; i++ {
		p.appleMap[p.appleMapPrev[i]] = false
	}
	p.appleMapN = 0
	for _, ap := range apples {
		if ap >= 0 && ap < n {
			p.appleMap[ap] = true
			p.appleMapPrev[p.appleMapN] = ap
			p.appleMapN++
		}
	}
}

// --- LandY (O(W×H) via bottom-up propagation) ---

func (p *Plan) computeLandY() {
	g := p.g
	w, h := g.W, g.H
	for x := 0; x < w; x++ {
		landY := h - 1 // default: fall to grid bottom
		for y := h - 1; y >= 0; y-- {
			idx := y*w + x
			if !g.Cell[idx] {
				// Wall: cells above this land at y-1 (just above this wall).
				// The wall cell itself gets y (irrelevant, never queried for free cells).
				p.LandY[idx] = y
				landY = y - 1
			} else {
				p.LandY[idx] = landY
			}
		}
	}
}

// --- Surface detection (uses wall-only grounding for static surfaces) ---

func (p *Plan) detectSurfaces() {
	g := p.g
	for i := range p.SurfAt {
		p.SurfAt[i] = -1
	}
	p.Surfs = p.Surfs[:0]

	for y := 0; y < g.H; y++ {
		inSurf := false
		var cur Surface
		for x := 0; x < g.W; x++ {
			idx := y*g.W + x
			// Static grounding: wall-only (no apples — surfaces are static)
			grounded := g.Cell[idx] && (y+1 >= g.H || !g.Cell[idx+g.W])
			if grounded {
				if !inSurf {
					cur = Surface{ID: len(p.Surfs), Y: y, Left: x, Right: x}
					inSurf = true
				} else {
					cur.Right = x
				}
			} else if inSurf {
				p.addSurface(cur)
				inSurf = false
			}
		}
		if inSurf {
			p.addSurface(cur)
		}
	}
}

func (p *Plan) addSurface(s Surface) {
	g := p.g
	p.Surfs = append(p.Surfs, s)
	for x := s.Left; x <= s.Right; x++ {
		p.SurfAt[g.Idx(x, s.Y)] = s.ID
	}
}

// --- Surface graph ---

func (p *Plan) buildSurfaceGraph() {
	g := p.g
	n := len(p.Surfs)
	p.SurfAdj = make([][]SurfLink, n)
	p.fallSeen = make([]bool, n)

	for i := 0; i < n; i++ {
		s := &p.Surfs[i]
		if s.Left > 0 {
			p.addFallLink(i, s.Left, s.Y, -1)
		}
		if s.Right < g.W-1 {
			p.addFallLink(i, s.Right, s.Y, +1)
		}
		p.addClimbLinks(i, s.Left, s.Y, -1)
		p.addClimbLinks(i, s.Right, s.Y, +1)
	}
}

func (p *Plan) addFallLink(fromSurf, edgeX, surfY, dx int) {
	g := p.g

	offX := edgeX + dx
	if offX < 0 || offX >= g.W || !g.Cell[g.Idx(offX, surfY)] {
		return
	}

	// Track which target surfaces we've already linked to avoid duplicates.
	var toClean [MaxAG]int
	nClean := 0
	minLandY := g.H

	for bodyLen := 1; bodyLen <= MaxAG; bodyLen++ {
		cx := edgeX + dx*bodyLen
		if cx < 0 || cx >= g.W {
			break
		}
		cell := g.Idx(cx, surfY)
		if !g.Cell[cell] {
			break
		}
		if ly := p.LandY[cell]; ly < minLandY {
			minLandY = ly
		}
		if minLandY <= surfY {
			continue
		}

		headX := cx
		landCell := g.Idx(headX, minLandY)
		to := p.SurfAt[landCell]
		if to < 0 || to == fromSurf || p.fallSeen[to] {
			continue
		}
		p.fallSeen[to] = true
		toClean[nClean] = to
		nClean++
		p.SurfAdj[fromSurf] = append(p.SurfAdj[fromSurf], SurfLink{
			To: to, Cost: bodyLen, MinBody: bodyLen,
		})
	}
	for i := 0; i < nClean; i++ {
		p.fallSeen[toClean[i]] = false
	}
}

func (p *Plan) addClimbLinks(fromSurf int, edgeX, surfY, dx int) {
	g := p.g
	sideX := edgeX + dx
	if sideX < 0 || sideX >= g.W {
		return
	}

	for h := 1; h < g.H; h++ {
		climbY := surfY - h
		if climbY < 0 {
			break
		}
		if !g.Cell[g.Idx(edgeX, climbY)] {
			break
		}
		sideCell := g.Idx(sideX, climbY)
		if !g.Cell[sideCell] {
			continue
		}
		if to := p.SurfAt[sideCell]; to >= 0 && to != fromSurf {
			p.SurfAdj[fromSurf] = append(p.SurfAdj[fromSurf], SurfLink{
				To: to, Cost: h + 1, MinBody: h + 1,
			})
		}
	}
}

// precomputeAllPairsSurfDist runs Floyd-Warshall for each body length 1..MaxAG.
// After this, SurfDist becomes an O(1) table lookup.
func (p *Plan) precomputeAllPairsSurfDist() {
	n := len(p.Surfs)
	p.surfN = n
	if n == 0 {
		return
	}
	p.surfAPD = make([][]int, MaxAG+1)
	for bl := 1; bl <= MaxAG; bl++ {
		dist := make([]int, n*n)
		for i := range dist {
			dist[i] = MaxCells
		}
		for i := 0; i < n; i++ {
			dist[i*n+i] = 0
		}
		for u := 0; u < n; u++ {
			for _, link := range p.SurfAdj[u] {
				if link.MinBody > bl {
					continue
				}
				idx := u*n + link.To
				if link.Cost < dist[idx] {
					dist[idx] = link.Cost
				}
			}
		}
		for k := 0; k < n; k++ {
			for i := 0; i < n; i++ {
				ik := dist[i*n+k]
				if ik >= MaxCells {
					continue
				}
				for j := 0; j < n; j++ {
					if d := ik + dist[k*n+j]; d < dist[i*n+j] {
						dist[i*n+j] = d
					}
				}
			}
		}
		p.surfAPD[bl] = dist
	}
}

// --- Surface graph pathfinding ---

// SurfDist returns precomputed shortest path distance between two surfaces,
// considering only links passable with the given body length. O(1) lookup.
func (p *Plan) SurfDist(from, to, bodyLen int) int {
	n := p.surfN
	if from < 0 || to < 0 || from >= n || to >= n {
		return -1
	}
	if from == to {
		return 0
	}
	bl := bodyLen
	if bl > MaxAG {
		bl = MaxAG
	}
	if bl < 1 {
		bl = 1
	}
	d := p.surfAPD[bl][from*n+to]
	if d >= MaxCells {
		return -1
	}
	return d
}

// EstimateDist estimates the total turns from a snake to a target cell
// using body simulation for the first step + surface graph for the rest.
func (p *Plan) EstimateDist(body []int, target int) (int, int) {
	g := p.g
	head := body[0]
	if head < 0 {
		return -1, -1
	}
	bodyLen := len(body)
	targetSurf := p.SurfAt[target]
	neck := neckOf(body)

	bestDist := MaxCells
	bestDir := -1

	for d := 0; d < 4; d++ {
		nc := g.Nb[head][d]
		if nc == -1 || nc == neck {
			continue
		}
		landCell, _ := p.simulateFirstMove(body, d)
		if landCell < 0 || landCell >= g.OobBase {
			continue
		}
		landSurf := p.SurfAt[landCell]
		if landSurf < 0 || targetSurf < 0 {
			continue
		}

		var dist int
		if landSurf == targetSurf {
			dist = 1 + g.Manhattan(landCell, target)
		} else {
			ls := &p.Surfs[landSurf]
			ts := &p.Surfs[targetSurf]
			lx, _ := g.XY(landCell)
			tx, _ := g.XY(target)
			walkTo := lx - ls.Left
			if r := ls.Right - lx; r < walkTo {
				walkTo = r
			}
			graphDist := p.SurfDist(landSurf, targetSurf, bodyLen)
			if graphDist < 0 {
				continue
			}
			walkFrom := tx - ts.Left
			if r := ts.Right - tx; r < walkFrom {
				walkFrom = r
			}
			dist = 1 + walkTo + graphDist + walkFrom
		}

		if dist < bestDist {
			bestDist = dist
			bestDir = d
		}
	}

	if bestDist >= MaxCells {
		return -1, -1
	}
	return bestDist, bestDir
}

// --- Ground checks (O(1) via apple bitmap) ---

// isGroundedAt returns true if cell c has solid ground directly below it
// (wall, apple, or grid bottom). Returns false for OOB cells.
func (p *Plan) isGroundedAt(c int) bool {
	g := p.g
	if c >= g.OobBase {
		return false
	}
	_, y := g.XY(c)
	if y+1 >= g.H {
		return true
	}
	below := c + g.W
	return !g.Cell[below] || p.appleMap[below]
}

// isApple returns true if cell c contains an apple. Returns false for OOB.
func (p *Plan) isApple(c int) bool {
	if c >= p.g.OobBase {
		return false
	}
	return p.appleMap[c]
}

// --- Fall computation ---

// computeFallWithBody estimates the fall landing when the body extends
// bodyLen cells in the opposite direction of move d. Returns (landCell, ag).
// Uses min(LandY) across body columns, checks apples in fall path.
func (p *Plan) computeFallWithBody(nc int, d int, bodyLen int) (int, int) {
	g := p.g
	if nc >= g.OobBase {
		return -1, 0 // OOB head falls out of map
	}
	nx, ny := g.XY(nc)
	bdx, bdy := -Dl[d][0], -Dl[d][1]

	// Find min LandY across all body columns, accounting for apples
	minLandY := p.LandY[nc]
	p.clipFallByApples(nx, ny, &minLandY)

	for i := 1; i < bodyLen; i++ {
		bx := nx + bdx*i
		by := ny + bdy*i
		if bx < 0 || bx >= g.W || by < 0 || by >= g.H {
			break
		}
		cell := g.Idx(bx, by)
		if !g.Cell[cell] {
			break
		}
		ly := p.LandY[cell]
		p.clipFallByApples(bx, by, &ly)
		if ly < minLandY {
			minLandY = ly
		}
	}

	if minLandY <= ny {
		return nc, 0
	}

	headLand := g.Idx(nx, minLandY)

	// Compute ag: scan body segments from head, find first grounded after landing
	drop := minLandY - ny
	for i := 0; i < bodyLen; i++ {
		bx := nx + bdx*i
		by := ny + bdy*i
		if bx < 0 || bx >= g.W || by < 0 || by >= g.H {
			break
		}
		// Rigid body: all segments drop by the same amount
		landBy := by + drop
		if landBy >= g.H {
			continue
		}
		if p.isGroundedAt(g.Idx(bx, landBy)) {
			return headLand, i
		}
	}
	return headLand, 0
}

// clipFallByApples reduces *landY if any apple in column cx between cy+1
// and *landY would intercept the fall (apple acts as ground).
func (p *Plan) clipFallByApples(cx, cy int, landY *int) {
	g := p.g
	for ay := cy + 1; ay <= *landY; ay++ {
		if p.appleMap[g.Idx(cx, ay)] {
			if ay-1 < *landY {
				*landY = ay - 1
			}
			return
		}
	}
}

// --- BFS ---

// neckOf returns the cell index of body[1] (neck), or -1 if not available.
func neckOf(body []int) int {
	if len(body) > 1 && body[1] >= 0 {
		return body[1]
	}
	return -1
}

// initialAG computes the starting above-ground counter for a snake body.
// Scans HEAD-to-TAIL: ag = index of the first grounded segment.
func (p *Plan) initialAG(body []int) int {
	for i := 0; i < len(body); i++ {
		if body[i] >= 0 && p.isGroundedAt(body[i]) {
			return i
		}
	}
	return len(body)
}

// simulateMove applies engine-compatible movement for a single snake against
// static walls, apples, and its own body. It resolves the move and beheading
// but does not apply falling.
func (p *Plan) simulateMove(body []int, d int) ([]int, bool) {
	g := p.g
	if len(body) == 0 {
		return nil, false
	}

	hx, hy := g.CellXY(body[0])
	nx := hx + Dl[d][0]
	ny := hy + Dl[d][1]
	newHead := g.CellIdx(nx, ny)

	eating := newHead >= 0 && newHead < g.OobBase && p.isApple(newHead)
	newLen := len(body)
	if eating && newLen < MaxSeg {
		newLen++
	}

	newBody := p.firstMoveBuf[:newLen]
	newBody[0] = newHead
	if eating {
		copy(newBody[1:], body)
	} else if len(body) > 1 {
		copy(newBody[1:], body[:len(body)-1])
	}

	hitWall := newHead >= 0 && newHead < g.OobBase && !g.Cell[newHead]
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

func bodyContains(body []int, cell int) bool {
	for _, part := range body {
		if part == cell {
			return true
		}
	}
	return false
}

func (p *Plan) stepCell(cell int, d int) int {
	if cell < 0 {
		return -1
	}
	x, y := p.g.CellXY(cell)
	return p.g.CellIdx(x+Dl[d][0], y+Dl[d][1])
}

func (p *Plan) hasTileOrAppleUnder(cell int, eaten []bool) bool {
	if cell < 0 {
		return false
	}
	below := p.stepCell(cell, DD)
	if below >= 0 && below < p.g.OobBase {
		if !p.g.Cell[below] {
			return true
		}
		return p.appleMap[below] && !eaten[below]
	}
	return false
}

func (p *Plan) isGroundedForResolve(body []int, snakes []Snake, grounded []bool, eaten []bool) bool {
	for _, cell := range body {
		if p.hasTileOrAppleUnder(cell, eaten) {
			return true
		}
		below := p.stepCell(cell, DD)
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

func (p *Plan) allOutOfBounds(body []int) bool {
	for _, cell := range body {
		if cell < 0 {
			continue
		}
		_, y := p.g.CellXY(cell)
		if y < p.g.H+1 {
			return false
		}
	}
	return true
}

// resolveMove resolves engine-compatible post-move state for all snakes and
// returns the cells of apples eaten by moved heads this turn.
// Input bodies must already reflect the simultaneous movement step:
// head added, tail popped unless an apple was eaten.
func (p *Plan) resolveMove(snakes []Snake) []int {
	if len(snakes) == 0 {
		return nil
	}

	p.RebuildAppleMap()
	eaten := make([]bool, p.g.OobBase)
	eatenList := make([]int, 0, len(snakes))
	for i := range snakes {
		if !snakes[i].Alive || len(snakes[i].Body) == 0 {
			continue
		}
		head := snakes[i].Body[0]
		if head >= 0 && head < p.g.OobBase && p.appleMap[head] {
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
		isInWall := head >= 0 && head < p.g.OobBase && !p.g.Cell[head]
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
				if p.isGroundedForResolve(snakes[i].Body, snakes, grounded, eaten) {
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
				snakes[i].Body[bi] = p.stepCell(cell, DD)
			}
			if p.allOutOfBounds(snakes[i].Body) {
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

// simulateFirstMove computes the actual head position after moving in
// direction d with full body simulation (movement + falling).
func (p *Plan) simulateFirstMove(body []int, d int) (int, int) {
	g := p.g
	newBody, alive := p.simulateMove(body, d)
	if !alive || len(newBody) == 0 {
		return -1, 0
	}

	// Check if any segment is grounded
	grounded := false
	for _, c := range newBody {
		if c >= 0 && p.isGroundedAt(c) {
			grounded = true
			break
		}
	}

	if !grounded {
		// Compute fall: min drop across in-grid body segments
		minDrop := g.H
		for _, c := range newBody {
			if c < 0 || c >= g.OobBase {
				continue
			}
			_, cy := g.XY(c)
			if drop := p.LandY[c] - cy; drop < minDrop {
				minDrop = drop
			}
		}
		if minDrop <= 0 {
			return newBody[0], p.initialAG(newBody)
		}
		// Apply fall to all segments (grid + OOB)
		for i, c := range newBody {
			if c < 0 {
				continue
			}
			cx, cy := g.CellXY(c)
			newBody[i] = g.CellIdx(cx, cy+minDrop)
		}
		head := newBody[0]
		if head < 0 || head >= g.NCells {
			return -1, 0
		}
		return head, p.initialAG(newBody)
	}

	return newBody[0], p.initialAG(newBody)
}

// BFSFindAll runs chain-aware BFS (tracks apple eating along path).
func (p *Plan) BFSFindAll(body []int) []PathResult {
	return p.bfsFindAll(body, true)
}

// BFSFindAllSimple runs BFS without chain tracking (faster, for enemies).
func (p *Plan) BFSFindAllSimple(body []int) []PathResult {
	return p.bfsFindAll(body, false)
}

// bfsFindAll runs a single BFS from the snake's head, exploring all reachable
// cells. The FIRST step uses exact body simulation. Subsequent steps use the
// (cell, ag, eaten) approximation — eating apples increases effective body
// length, enabling longer unsupported bridges. Returns results indexed by cell.
func (p *Plan) bfsFindAll(body []int, trackChain bool) []PathResult {
	g := p.g
	n := g.W * g.H
	bodyLen := len(body)
	if bodyLen == 0 {
		return nil
	}
	head := body[0]
	if head < 0 || head >= n {
		return nil // OOB or invalid head — can't BFS
	}

	maxAG := bodyLen
	if maxAG > MaxAG {
		maxAG = MaxAG
	}

	// Rebuild apple bitmap for this turn's state
	p.RebuildAppleMap()

	results := make([]PathResult, n)
	for i := range results {
		results[i].Dist = -1
	}

	p.curGen++
	if p.curGen == 0 {
		for i := range p.visitGen {
			p.visitGen[i] = 0
		}
		p.curGen = 1
	}

	stride := MaxChainEaten + 1 // eaten dimension size

	neck := neckOf(body)
	results[head] = PathResult{Dist: 0, FirstDir: -1}
	startAG := p.initialAG(body)
	if startAG >= maxAG {
		startAG = maxAG - 1
	}
	p.visitGen[(head*MaxAG+startAG)*stride] = p.curGen

	// Seed BFS with accurate first moves (full body simulation)
	p.queue = p.queue[:0]
	for d := 0; d < 4; d++ {
		nc := g.Nb[head][d]
		if nc == -1 || nc == neck {
			continue
		}

		// Detect apple eating on first move
		var firstEaten int8
		if trackChain && nc >= 0 && nc < n && p.isApple(nc) {
			firstEaten = 1
		}

		finalCell, nag := p.simulateFirstMove(body, d)
		if finalCell < 0 || finalCell >= g.NCells {
			continue
		}
		eMaxAG := maxAG + int(firstEaten)
		if eMaxAG > MaxAG {
			eMaxAG = MaxAG
		}
		if nag >= eMaxAG {
			nag = eMaxAG - 1
		}
		vk := (finalCell*MaxAG+nag)*stride + int(firstEaten)
		if p.visitGen[vk] == p.curGen {
			continue
		}
		p.visitGen[vk] = p.curGen
		if finalCell < n {
			r := &results[finalCell]
			if r.Dist == -1 {
				*r = PathResult{Dist: 1, FirstDir: d, Apples: int(firstEaten)}
			}
		}
		p.queue = append(p.queue, bfsNode{
			cell: int16(finalCell), ag: int8(nag),
			firstDir: int8(d), dist: 1,
			eaten: firstEaten,
		})
	}

	// Main BFS loop
	for qi := 0; qi < len(p.queue); qi++ {
		cur := p.queue[qi]
		cc := int(cur.cell)
		cag := int(cur.ag)

		for d := 0; d < 4; d++ {
			nc := g.Nb[cc][d]
			if nc == -1 {
				continue
			}

			// Track apple eating: stepping onto an apple grows body by 1.
			newEaten := cur.eaten
			if trackChain && nc >= 0 && nc < n && p.isApple(nc) && newEaten < MaxChainEaten {
				newEaten++
			}

			effectiveBodyLen := bodyLen + int(newEaten)
			eMaxAG := effectiveBodyLen
			if eMaxAG > MaxAG {
				eMaxAG = MaxAG
			}

			var nag int
			finalCell := nc

			if p.isGroundedAt(nc) {
				nag = 0
			} else {
				nag = cag + 1
			}

			if nag >= eMaxAG {
				// Apple eating: body grows +1, may prevent fall.
				// In chain mode this is handled by increased eMaxAG.
				// In simple mode, restore the one-step apple-prevents-fall logic.
				if !trackChain && p.isApple(nc) && nag < eMaxAG+1 {
					nag = cag
				} else {
					var fallAG int
					finalCell, fallAG = p.computeFallWithBody(nc, d, effectiveBodyLen)
					nag = fallAG
					if finalCell < 0 || finalCell >= g.NCells {
						continue
					}
				}
			}

			if finalCell < 0 || finalCell >= g.NCells {
				continue
			}
			vk := (finalCell*MaxAG+nag)*stride + int(newEaten)
			if p.visitGen[vk] == p.curGen {
				continue
			}
			p.visitGen[vk] = p.curGen

			fd := int(cur.firstDir)
			newDist := cur.dist + 1

			if finalCell < n {
				r := &results[finalCell]
				if r.Dist == -1 || (int(newDist) == r.Dist && int(newEaten) > r.Apples) {
					r.Dist = int(newDist)
					r.FirstDir = fd
					r.Apples = int(newEaten)
				}
			}
			p.queue = append(p.queue, bfsNode{
				cell: int16(finalCell), ag: int8(nag),
				firstDir: int8(fd), dist: newDist,
				eaten: newEaten,
			})
		}
	}

	return results
}

// --- SimBFS: body-simulation search for nearest reachable apples ---

const (
	simMaxTargets  = 5 // stop after finding this many apple targets
	simMaxEatDepth = 2 // apple eating grows body only within this many steps
)

// SimTarget describes a physically reachable apple found by SimBFS.
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

// applyGravity drops body segments until grounded. Returns false if fell off map.
func (p *Plan) applyGravity(body []int) bool {
	g := p.g
	n := g.W * g.H
	for iter := 0; iter < g.H; iter++ {
		for _, c := range body {
			if c >= 0 && c < n && p.isGroundedAt(c) {
				return true
			}
		}
		allOOB := true
		for i, c := range body {
			if c < 0 {
				continue
			}
			cx, cy := g.CellXY(c)
			nc := g.CellIdx(cx, cy+1)
			if nc < 0 {
				return false
			}
			body[i] = nc
			if nc < n {
				allOOB = false
			}
		}
		if allOOB {
			return false
		}
	}
	return false
}

// SimBFS searches for nearest reachable apples using actual body simulation.
// Uses simulateMove for physics, rejects moves causing death or segment loss.
// Apple eating grows body only within simMaxEatDepth steps.
func (p *Plan) SimBFS(body []int) []SimTarget {
	g := p.g
	n := g.W * g.H
	if len(body) == 0 || body[0] < 0 || body[0] >= n {
		return nil
	}

	p.RebuildAppleMap()

	visited := make(map[uint64]bool)
	visited[bodyHash(body)] = true
	queue := []simNode{{
		body:     append([]int(nil), body...),
		firstDir: -1,
	}}
	var targets []SimTarget

	for qi := 0; qi < len(queue) && len(targets) < simMaxTargets; qi++ {
		cur := queue[qi]
		head := cur.body[0]
		if head < 0 || head >= n {
			continue
		}
		neck := neckOf(cur.body)

		for dir := 0; dir < 4; dir++ {
			nc := g.Nb[head][dir]
			if nc == -1 || nc == neck {
				continue
			}

			// Simulate move (movement + collision + eating).
			newBody, alive := p.simulateMove(cur.body, dir)
			if !alive {
				continue
			}

			// Copy immediately (simulateMove returns firstMoveBuf slice).
			bodycp := append([]int(nil), newBody...)

			// Detect eating and segment loss.
			eating := nc >= 0 && nc < n && p.isApple(nc)
			expectedLen := len(cur.body)
			if eating {
				expectedLen++
			}
			if len(bodycp) < expectedLen {
				continue // segment loss — reject
			}

			// Beyond eat depth: undo body growth (score the apple, don't grow).
			if eating && cur.dist >= simMaxEatDepth {
				bodycp = bodycp[:len(bodycp)-1]
			}

			// Apply gravity.
			if !p.applyGravity(bodycp) {
				continue
			}

			fd := cur.firstDir
			if fd == -1 {
				fd = dir
			}
			newDist := cur.dist + 1
			newEaten := cur.eaten

			// Visited check.
			h := bodyHash(bodycp)
			if visited[h] {
				continue
			}
			visited[h] = true

			// Record apple target.
			if eating {
				newEaten++
				targets = append(targets, SimTarget{
					Apple:    nc,
					Dist:     newDist,
					FirstDir: fd,
					Eaten:    newEaten,
				})
				if len(targets) >= simMaxTargets {
					return targets
				}
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
