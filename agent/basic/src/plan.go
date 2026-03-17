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
	Cost    int // Manhattan distance between edge cells
	MinBody int // = Cost (body length to bridge, anchor excluded)
	From    int // source edge cell index
	ToCell  int // target edge cell index
}

// Plan holds precomputed data and reusable BFS buffers.
type Plan struct {
	*Sim

	// Static precomputation (set once by Precompute)
	LandY []int // y-coordinate where a free-falling cell lands

	// Surface graph
	Surfs    []Surface
	SurfAt   []int        // cell → surface index (-1 if not on surface)
	SurfAdj [][]SurfLink // adjacency list

	// All-pairs surface distances (Floyd-Warshall, per body length)
	surfN   int     // len(Surfs), cached
	surfAPD [][]int // [bodyLen] flat N×N matrix; [from*surfN+to]

	// BFS reuse buffers
	visitGen []uint16 // flat [(cell*MaxAG+ag)*(MaxChainEaten+1)+eaten]
	curGen   uint16
	queue    []bfsNode
}

// Precompute fills LandY, surfaces, and the surface graph.
// Called once after Init (within the 1s first-turn budget).
func (p *Plan) Precompute() {
	n := p.G.NCells
	p.LandY = make([]int, n)
	p.SurfAt = make([]int, n)
	p.visitGen = make([]uint16, n*MaxAG*(MaxChainEaten+1))
	p.computeLandY()
	p.detectSurfaces()
	p.buildSurfaceGraph()
	p.precomputeAllPairsSurfDist()
	if p.queue == nil {
		p.queue = make([]bfsNode, 0, n*4)
	}
}

// --- LandY (O(W×H) via bottom-up propagation) ---

func (p *Plan) computeLandY() {
	g := p.G
	for x := 0; x < g.W; x++ {
		landY := g.H - 1 // default: fall to grid bottom
		for y := g.H - 1; y >= 0; y-- {
			idx := g.Idx(x, y)
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
	g := p.G
	for i := range p.SurfAt {
		p.SurfAt[i] = -1
	}
	p.Surfs = p.Surfs[:0]

	for y := 0; y < g.H; y++ {
		inSurf := false
		var cur Surface
		for x := 0; x < g.W; x++ {
			idx := g.Idx(x, y)
			// Static grounding: wall-only (no apples — surfaces are static)
			grounded := g.Cell[idx] && (y+1 >= g.H || !g.Cell[idx+g.Stride])
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
	g := p.G
	p.Surfs = append(p.Surfs, s)
	for x := s.Left; x <= s.Right; x++ {
		p.SurfAt[g.Idx(x, s.Y)] = s.ID
	}
}

// --- Surface graph ---

const maxLinkDist = 5 // prune connections with Manhattan > 5

// betweenRect returns true if cell g lies strictly inside the bounding
// rectangle of a→b and is strictly closer to a than b is.
func betweenRect(g *Game, a, b, c int) bool {
	ax, ay := g.XY(a)
	bx, by := g.XY(b)
	cx, cy := g.XY(c)
	minX, maxX := ax, bx
	if minX > maxX {
		minX, maxX = maxX, minX
	}
	minY, maxY := ay, by
	if minY > maxY {
		minY, maxY = maxY, minY
	}
	if cx < minX || cx > maxX || cy < minY || cy > maxY {
		return false
	}
	if cx == ax && cy == ay {
		return false
	}
	return g.Manhattan(a, c) < g.Manhattan(a, b)
}

func (p *Plan) buildSurfaceGraph() {
	n := len(p.Surfs)
	p.SurfAdj = make([][]SurfLink, n)

	// Collect all edge cells: each surface contributes left and right edges.
	g := p.G

	type edgeCell struct {
		surfID int
		cell   int
	}
	var edges []edgeCell
	for i := 0; i < n; i++ {
		s := &p.Surfs[i]
		edges = append(edges, edgeCell{i, g.Idx(s.Left, s.Y)})
		if s.Right != s.Left {
			edges = append(edges, edgeCell{i, g.Idx(s.Right, s.Y)})
		}
	}

	// For each pair of edges from different surfaces, compute Manhattan.
	// Keep only the best (shortest) edge pair per surface pair.
	type candidate struct {
		from, to       int
		fromCell, toCell int
		dist           int
	}

	best := make([]candidate, n*n)
	for i := range best {
		best[i].dist = maxLinkDist + 1
	}

	for i := 0; i < len(edges); i++ {
		for j := 0; j < len(edges); j++ {
			ei, ej := edges[i], edges[j]
			if ei.surfID == ej.surfID {
				continue
			}
			d := g.Manhattan(ei.cell, ej.cell)
			if d > maxLinkDist {
				continue
			}
			key := ei.surfID*n + ej.surfID
			if d < best[key].dist {
				best[key] = candidate{ei.surfID, ej.surfID, ei.cell, ej.cell, d}
			}
		}
	}

	// Filter: remove links where an intermediate surface edge sits between source and target.
	for i := range best {
		c := &best[i]
		if c.dist > maxLinkDist {
			continue
		}
		for k := 0; k < len(edges); k++ {
			ek := edges[k]
			if ek.surfID == c.from || ek.surfID == c.to {
				continue
			}
			if betweenRect(g, c.fromCell, c.toCell, ek.cell) {
				c.dist = maxLinkDist + 1
				break
			}
		}
	}

	// Build adjacency list from surviving candidates.
	for i := range best {
		c := best[i]
		if c.dist > maxLinkDist {
			continue
		}
		p.SurfAdj[c.from] = append(p.SurfAdj[c.from], SurfLink{
			To: c.to, Cost: c.dist, MinBody: c.dist,
			From: c.fromCell, ToCell: c.toCell,
		})
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
	g := p.G
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
		nc := g.Nbm[head][d]
		if nc == -1 || nc == neck {
			continue
		}
		landCell, _ := p.simulateFirstMove(body, d)
		if landCell < 0 || !g.IsInGrid(landCell) {
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

// --- Fall computation ---

// computeFallWithBody estimates the fall landing when the body extends
// bodyLen cells in the opposite direction of move d. Returns (landCell, ag).
func (p *Plan) computeFallWithBody(nc int, d int, bodyLen int) (int, int) {
	g := p.G
	if !g.IsInGrid(nc) {
		return -1, 0
	}
	nx, ny := g.XY(nc)
	bdx, bdy := -Dl[d][0], -Dl[d][1]

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

	drop := minLandY - ny
	for i := 0; i < bodyLen; i++ {
		bx := nx + bdx*i
		by := ny + bdy*i
		if bx < 0 || bx >= g.W || by < 0 || by >= g.H {
			break
		}
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

func (p *Plan) clipFallByApples(cx, cy int, landY *int) {
	g := p.G
	for ay := cy + 1; ay <= *landY; ay++ {
		if p.appleMap[g.Idx(cx, ay)] {
			if ay-1 < *landY {
				*landY = ay - 1
			}
			return
		}
	}
}

// --- BFS helpers ---

// initialAG computes the starting above-ground counter for a snake body.
func (p *Plan) initialAG(body []int) int {
	for i := 0; i < len(body); i++ {
		if body[i] >= 0 && p.isGroundedAt(body[i]) {
			return i
		}
	}
	return len(body)
}

// simulateFirstMove computes the actual head position after moving in
// direction d with full body simulation (movement + falling).
func (p *Plan) simulateFirstMove(body []int, d int) (int, int) {
	g := p.G
	newBody, alive := p.simulateMove(body, d)
	if !alive || len(newBody) == 0 {
		return -1, 0
	}

	grounded := false
	for _, c := range newBody {
		if c >= 0 && p.isGroundedAt(c) {
			grounded = true
			break
		}
	}

	if !grounded {
		minDrop := g.H
		for _, c := range newBody {
			if !g.IsInGrid(c) {
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
		for i, c := range newBody {
			if c < 0 {
				continue
			}
			cx, cy := g.XY(c)
			newBody[i] = g.Idx(cx, cy+minDrop)
		}
		head := newBody[0]
		if head < 0 || head >= g.NCells {
			return -1, 0
		}
		return head, p.initialAG(newBody)
	}

	return newBody[0], p.initialAG(newBody)
}
