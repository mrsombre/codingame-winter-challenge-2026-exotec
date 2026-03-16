package main

// PathResult holds the output of a BFS search for a single target.
type PathResult struct {
	Dist     int // BFS steps to reach target (-1 if unreachable)
	FirstDir int // direction of the first move (DU/DR/DD/DL)
}

type bfsNode struct {
	cell     int16
	ag       int8 // above-ground counter (0 = grounded)
	firstDir int8 // direction of the very first step (-1 at start)
	dist     int16
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
	visitGen     []uint16 // flat [cell*MaxAG + ag], generation counter
	curGen       uint16
	queue        []bfsNode
	firstMoveBuf []int // scratch for simulateFirstMove
}

// Precompute fills LandY, surfaces, and the surface graph.
// Called once after Init (within the 1s first-turn budget).
func (p *Plan) Precompute() {
	n := p.g.W * p.g.H
	p.LandY = make([]int, n)
	p.SurfAt = make([]int, n)
	p.appleMap = make([]bool, n)
	p.appleMapPrev = make([]int, MaxAp)
	p.visitGen = make([]uint16, p.g.NCells*MaxAG)
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
	g := p.g
	n := len(p.appleMap)
	// Clear only previously set entries — O(ANum) not O(W×H)
	for i := 0; i < p.appleMapN; i++ {
		p.appleMap[p.appleMapPrev[i]] = false
	}
	// Set current apples
	p.appleMapN = 0
	for j := 0; j < g.ANum; j++ {
		ap := g.Ap[j]
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

// simulateFirstMove computes the actual head position after moving in
// direction d with full body simulation (accurate multi-column fall).
func (p *Plan) simulateFirstMove(body []int, d int) (int, int) {
	g := p.g
	nc := g.Nb[body[0]][d]
	if nc == -1 {
		return -1, 0
	}
	bodyLen := len(body)

	// If stepping onto an apple, body grows (tail preserved).
	eating := nc >= 0 && nc < g.OobBase && p.isApple(nc)
	newLen := bodyLen
	if eating && bodyLen < MaxSeg {
		newLen = bodyLen + 1
	}

	// Use scratch buffer instead of allocating
	newBody := p.firstMoveBuf[:newLen]
	newBody[0] = nc
	for i := 1; i < newLen; i++ {
		newBody[i] = body[i-1]
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
			return nc, p.initialAG(newBody)
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

	return nc, p.initialAG(newBody)
}

// BFSFindAll runs a single BFS from the snake's head, exploring all reachable
// cells. The FIRST step uses exact body simulation. Subsequent steps use the
// (cell, ag) approximation. Returns results indexed by cell.
func (p *Plan) BFSFindAll(body []int) []PathResult {
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

	neck := neckOf(body)
	results[head] = PathResult{Dist: 0, FirstDir: -1}
	startAG := p.initialAG(body)
	if startAG >= maxAG {
		startAG = maxAG - 1
	}
	p.visitGen[head*MaxAG+startAG] = p.curGen

	// Seed BFS with accurate first moves (full body simulation)
	p.queue = p.queue[:0]
	for d := 0; d < 4; d++ {
		nc := g.Nb[head][d]
		if nc == -1 || nc == neck {
			continue
		}
		finalCell, nag := p.simulateFirstMove(body, d)
		if finalCell < 0 || finalCell >= g.NCells {
			continue
		}
		if nag >= maxAG {
			nag = maxAG - 1
		}
		if p.visitGen[finalCell*MaxAG+nag] == p.curGen {
			continue
		}
		p.visitGen[finalCell*MaxAG+nag] = p.curGen
		if finalCell < n && results[finalCell].Dist == -1 {
			results[finalCell] = PathResult{Dist: 1, FirstDir: d}
		}
		p.queue = append(p.queue, bfsNode{
			cell: int16(finalCell), ag: int8(nag),
			firstDir: int8(d), dist: 1,
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

			var nag int
			finalCell := nc

			if p.isGroundedAt(nc) {
				nag = 0
			} else {
				nag = cag + 1
			}

			if nag >= maxAG {
				// Apple eating: body grows +1, may prevent fall
				if p.isApple(nc) && nag < maxAG+1 {
					nag = cag
				} else {
					var fallAG int
					finalCell, fallAG = p.computeFallWithBody(nc, d, bodyLen)
					nag = fallAG
					if finalCell < 0 || finalCell >= g.NCells {
						continue
					}
				}
			}

			if finalCell < 0 || finalCell >= g.NCells {
				continue
			}
			if p.visitGen[finalCell*MaxAG+nag] == p.curGen {
				continue
			}
			p.visitGen[finalCell*MaxAG+nag] = p.curGen

			fd := int(cur.firstDir)
			newDist := cur.dist + 1

			if finalCell < n && results[finalCell].Dist == -1 {
				results[finalCell] = PathResult{Dist: int(newDist), FirstDir: fd}
			}
			p.queue = append(p.queue, bfsNode{
				cell: int16(finalCell), ag: int8(nag),
				firstDir: int8(fd), dist: newDist,
			})
		}
	}

	return results
}
