package main

import (
	"bufio"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

const (
	DU = 0 // up    dy=-1
	DR = 1 // right dx=+1
	DD = 2 // down  dy=+1
	DL = 3 // left  dx=-1
)

const (
	MaxW     = 45
	MaxH     = 30
	MaxCells = MaxW * MaxH
)

var Dl = [4][2]int{
	{0, -1}, // DU
	{1, 0},  // DR
	{0, 1},  // DD
	{-1, 0}, // DL
}

var Dn = [4]string{"UP", "RIGHT", "DOWN", "LEFT"}

const (
	MaxASn = 8   // total snakes both players
	MaxPSn = 4   // max snakes per player
	MaxSeg = 256 // max body parts per snake
	MaxAp  = 128 // max power sources (apples)
	MaxAG  = 33  // max above-ground counter for BFS (capped for memory)
)

// Snake holds one snake's current body as flat cell indices, head-first.
type Snake struct {
	ID    int
	Owner int // 0 = mine, 1 = enemy
	Body  []int
	Len   int
	Alive bool
}

// SurfType classifies how a surface is grounded.
const (
	SurfSolid = iota // grounded on walls
	SurfApple        // grounded on apples (temporary)
	SurfNone         // apple gone, surface invalid
)

// Surface represents a horizontal platform segment where snakes can walk.
type Surface struct {
	ID    int
	Y     int // y of the walkable cells
	Left  int // leftmost x
	Right int // rightmost x
	Len   int // Right - Left + 1
	Type  int // SurfSolid, SurfApple, SurfNone
	Links []SurfLink
}

// SurfLink represents a BFS-based directed connection between two surfaces.
type SurfLink struct {
	To      int   // target surface ID
	Landing int   // landing cell index (first cell reached on target surface)
	Len     int   // BFS step count
	Path    []int // full path: [source edge cell, ..., landing cell]
}

type Game struct {
	ID int

	W, H   int
	Stride int      // W + 2 (expanded grid row width)
	Cell   []bool   // false = wall, true = free (sized NCells; border cells = true)
	Nb     [][4]int // precomputed neighbor index; -1 = out of bounds (sized NCells)
	Nbm    [][4]int // valid moves: not wall, in bounds; -1 = blocked (sized NCells)
	NCells int      // (W+2) * (H+2)
	InGrid []bool   // true for game-grid cells, false for border cells

	// Static map precomputation
	Surfs  []Surface
	SurfAt []int // cell -> surface index (-1 if not on surface)

	MyIDs [MaxPSn]int // my snake IDs
	MyN   int
	OpIDs [MaxPSn]int // enemy snake IDs
	OpN   int

	// Turn data (overwritten each turn)
	Sn   [MaxASn]Snake
	SNum int
	Ap   []int // flat cell indices
	ANum int

	Oob int
}

// Init reads w, h, row strings from scanner, builds walls and neighbors in one pass.
func Init(s *bufio.Scanner) *Game {
	g := &Game{}
	s.Scan()
	fmt.Sscan(s.Text(), &g.ID)
	log(g.ID)
	s.Scan()
	fmt.Sscan(s.Text(), &g.W)
	log(g.W)
	s.Scan()
	fmt.Sscan(s.Text(), &g.H)
	log(g.H)

	g.Oob = 2
	g.Stride = g.W + g.Oob*2
	g.NCells = g.Stride * (g.H + g.Oob*2)
	g.Cell = make([]bool, g.NCells)
	g.InGrid = make([]bool, g.NCells)
	g.Nb = make([][4]int, g.NCells)
	g.Nbm = make([][4]int, g.NCells)

	// Border cells default to free (snakes can be OOB); game cells set below.
	for i := range g.Cell {
		g.Cell[i] = true
	}
	// read rows and set walls + InGrid
	for y := 0; y < g.H; y++ {
		s.Scan()
		row := s.Text()
		log(row)
		for x := 0; x < g.W; x++ {
			idx := g.Idx(x, y)
			g.InGrid[idx] = true
			if row[x] == '#' {
				g.Cell[idx] = false
			}
		}
	}

	// precompute neighbors for all cells (grid + border)
	// Nb[cell][d]  = neighbor index, -1 only if out of expanded bounds
	// Nbm[cell][d] = valid move neighbor (not wall, in bounds), -1 if blocked
	for cell := 0; cell < g.NCells; cell++ {
		cx, cy := g.XY(cell)
		for d := 0; d < 4; d++ {
			ni := g.Idx(cx+Dl[d][0], cy+Dl[d][1])
			g.Nb[cell][d] = ni
			if ni >= 0 && g.Cell[ni] {
				g.Nbm[cell][d] = ni
			} else {
				g.Nbm[cell][d] = -1
			}
		}
	}
	g.precomputeStaticMap()

	var sp int
	s.Scan()
	fmt.Sscan(s.Text(), &sp)
	log(sp)
	g.MyN = sp
	for i := 0; i < sp; i++ {
		s.Scan()
		fmt.Sscan(s.Text(), &g.MyIDs[i])
		log(g.MyIDs[i])
	}
	g.OpN = sp
	for i := 0; i < sp; i++ {
		s.Scan()
		fmt.Sscan(s.Text(), &g.OpIDs[i])
		log(g.OpIDs[i])
	}

	// pre-allocate turn data
	g.Ap = make([]int, 0, MaxAp)

	return g
}

// ParseBody parses "x,y:x,y:x,y" into flat cell indices.
// OOB segments get -1.
func (g *Game) ParseBody(s string) []int {
	dst := make([]int, 0, 8)
	for _, seg := range strings.Split(s, ":") {
		comma := strings.IndexByte(seg, ',')
		x, _ := strconv.Atoi(seg[:comma])
		y, _ := strconv.Atoi(seg[comma+1:])
		dst = append(dst, g.Idx(x, y))
	}
	return dst
}

// Turn reads per-turn data from scanner: apples and snakes.
func (g *Game) Turn(s *bufio.Scanner) {
	// apples
	s.Scan()
	fmt.Sscan(s.Text(), &g.ANum)
	log(g.ANum)
	g.Ap = g.Ap[:0]
	for i := 0; i < g.ANum; i++ {
		var x, y int
		s.Scan()
		fmt.Sscan(s.Text(), &x, &y)
		log(x, y)
		g.Ap = append(g.Ap, g.Idx(x, y))
	}

	// snakes
	for i := 0; i < g.SNum; i++ {
		g.Sn[i] = Snake{}
	}
	s.Scan()
	fmt.Sscan(s.Text(), &g.SNum)
	log(g.SNum)
	for i := 0; i < g.SNum; i++ {
		var id int
		var body string
		s.Scan()
		fmt.Sscan(s.Text(), &id, &body)
		log(id, body)
		sn := &g.Sn[i]
		sn.ID = id
		sn.Alive = true
		if g.IsMy(id) {
			sn.Owner = 0
		} else {
			sn.Owner = 1
		}
		sn.Body = g.ParseBody(body)
		sn.Len = len(sn.Body)
	}
}

// Idx converts game coordinates to expanded flat index.
func (g *Game) Idx(x, y int) int {
	if x < -g.Oob || x >= g.W+g.Oob || y < -g.Oob || y >= g.H+g.Oob {
		return -1
	}
	return (y+g.Oob)*g.Stride + (x + g.Oob)
}

// XY converts expanded flat index to game coordinates.
func (g *Game) XY(idx int) (int, int) {
	return idx%g.Stride - g.Oob, idx/g.Stride - g.Oob
}

func (g *Game) IsInGrid(cell int) bool {
	return cell >= 0 && cell < g.NCells && g.InGrid[cell]
}

// IsMy returns true if id belongs to my snakes.
func (g *Game) IsMy(id int) bool {
	for i := 0; i < g.MyN; i++ {
		if g.MyIDs[i] == id {
			return true
		}
	}
	return false
}

// Manhattan returns the Manhattan distance between two cells.
func (g *Game) Manhattan(a, b int) int {
	ax, ay := g.XY(a)
	bx, by := g.XY(b)
	dx := ax - bx
	if dx < 0 {
		dx = -dx
	}
	dy := ay - by
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

func (g *Game) precomputeStaticMap() {
	g.SurfAt = make([]int, g.NCells)
	g.detectSurfaces()
	g.buildSurfaceLinks()
}

// --- Surface detection (uses wall-only grounding for static surfaces) ---

func (g *Game) detectSurfaces() {
	for i := range g.SurfAt {
		g.SurfAt[i] = -1
	}
	g.Surfs = g.Surfs[:0]

	for y := 0; y < g.H; y++ {
		inSurf := false
		var cur Surface
		for x := 0; x < g.W; x++ {
			idx := g.Idx(x, y)
			// Static grounding: wall-only (no apples — surfaces are static)
			grounded := g.Cell[idx] && (y+1 >= g.H || !g.Cell[idx+g.Stride])
			if grounded {
				if !inSurf {
					cur = Surface{ID: len(g.Surfs), Y: y, Left: x, Right: x, Type: SurfSolid}
					inSurf = true
				} else {
					cur.Right = x
				}
			} else if inSurf {
				g.addSurface(cur)
				inSurf = false
			}
		}
		if inSurf {
			g.addSurface(cur)
		}
	}
}

func (g *Game) addSurface(s Surface) {
	s.Len = s.Right - s.Left + 1
	g.Surfs = append(g.Surfs, s)
	for x := s.Left; x <= s.Right; x++ {
		g.SurfAt[g.Idx(x, s.Y)] = s.ID
	}
}

// --- Surface links (BFS-based) ---

const maxLinkDepth = 8 // max BFS steps from surface edge

// surfBFS holds reusable buffers for surface link BFS.
type surfBFS struct {
	visited  []bool
	dist     []int
	parent   []int
	queue    []int
	surfBest map[int]SurfLink
}

func newSurfBFS(ncells int) *surfBFS {
	return &surfBFS{
		visited:  make([]bool, ncells),
		dist:     make([]int, ncells),
		parent:   make([]int, ncells),
		queue:    make([]int, 0, ncells),
		surfBest: make(map[int]SurfLink),
	}
}

// buildLinksFor runs BFS from the edges of surface si and populates its Links.
func (b *surfBFS) buildLinksFor(g *Game, si int) {
	s := &g.Surfs[si]

	edges := [2]int{g.Idx(s.Left, s.Y), -1}
	ne := 1
	if s.Right != s.Left {
		edges[1] = g.Idx(s.Right, s.Y)
		ne = 2
	}

	for k := range b.surfBest {
		delete(b.surfBest, k)
	}

	for ei := 0; ei < ne; ei++ {
		src := edges[ei]
		b.queue = b.queue[:0]

		for x := s.Left; x <= s.Right; x++ {
			b.visited[g.Idx(x, s.Y)] = true
		}
		b.visited[src] = false

		b.queue = append(b.queue, src)
		b.visited[src] = true
		b.dist[src] = 0
		b.parent[src] = -1

		edgeHits := make(map[int]int)
		var surfHits []int

		head := 0
		for head < len(b.queue) {
			cur := b.queue[head]
			head++
			if b.dist[cur] >= maxLinkDepth {
				continue
			}

			for d := 0; d < 4; d++ {
				nb := g.Nbm[cur][d]
				if nb < 0 || b.visited[nb] {
					continue
				}
				b.visited[nb] = true
				b.dist[nb] = b.dist[cur] + 1
				b.parent[nb] = cur

				tid := g.SurfAt[nb]
				if tid >= 0 && tid != si {
					surfHits = append(surfHits, nb)
					if _, already := edgeHits[tid]; !already {
						edgeHits[tid] = nb
					}
					continue
				}
				b.queue = append(b.queue, nb)
			}
		}

		for tid, landing := range edgeHits {
			d := b.dist[landing]
			if prev, ok := b.surfBest[tid]; ok && d >= prev.Len {
				continue
			}
			path := make([]int, d+1)
			p := landing
			for i := d; i >= 0; i-- {
				path[i] = p
				p = b.parent[p]
			}
			b.surfBest[tid] = SurfLink{
				To: tid, Landing: landing, Len: d, Path: path,
			}
		}

		for i := 0; i < len(b.queue); i++ {
			b.visited[b.queue[i]] = false
		}
		for _, cell := range surfHits {
			b.visited[cell] = false
		}
		for x := s.Left; x <= s.Right; x++ {
			b.visited[g.Idx(x, s.Y)] = false
		}
	}

	s.Links = make([]SurfLink, 0, len(b.surfBest))
	for _, link := range b.surfBest {
		s.Links = append(s.Links, link)
	}
	sort.Slice(s.Links, func(i, j int) bool {
		return s.Links[i].Len < s.Links[j].Len
	})
}

func (g *Game) buildSurfaceLinks() {
	b := newSurfBFS(g.NCells)
	for si := 0; si < len(g.Surfs); si++ {
		b.buildLinksFor(g, si)
	}
	g.addFallLinks()
}

// addFallLinks adds one-way "fall" links from solid surface edges.
// From each edge, step one cell off the surface horizontally, then fall
// straight down to the first surface. No depth limit.
func (g *Game) addFallLinks() {
	for si := range g.Surfs {
		s := &g.Surfs[si]
		if s.Type != SurfSolid {
			continue
		}

		// (edge cell, step-off direction)
		type edgeFall struct {
			edge int
			dx   int
		}
		var falls []edgeFall
		// Left edge: step left (dx=-1)
		falls = append(falls, edgeFall{g.Idx(s.Left, s.Y), -1})
		// Right edge: step right (dx=+1)
		if s.Right != s.Left {
			falls = append(falls, edgeFall{g.Idx(s.Right, s.Y), +1})
		} else {
			// Single cell: also step right
			falls = append(falls, edgeFall{g.Idx(s.Left, s.Y), +1})
		}

		for _, f := range falls {
			ex, ey := g.XY(f.edge)
			offX := ex + f.dx
			offCell := g.Idx(offX, ey)

			// Step-off cell must be free and not on any surface.
			if offCell < 0 || !g.Cell[offCell] {
				continue
			}
			if g.SurfAt[offCell] >= 0 {
				continue // already on a surface, BFS handles this
			}

			// Fall straight down from step-off cell.
			path := []int{f.edge, offCell}
			for y := ey + 1; y < g.H; y++ {
				cell := g.Idx(offX, y)
				if !g.Cell[cell] {
					break // hit wall
				}
				path = append(path, cell)
				tid := g.SurfAt[cell]
				if tid >= 0 && tid != si {
					s.Links = append(s.Links, SurfLink{
						To:      tid,
						Landing: cell,
						Len:     len(path) - 1,
						Path:    path,
					})
					break
				}
			}
		}
	}
}

// InitAppleSurfaces creates apple surfaces and builds their BFS links.
// Call once on first turn. Links are kept — eaten apples become SurfNone.
func (g *Game) InitAppleSurfaces() {
	for i := 0; i < g.ANum; i++ {
		ax, ay := g.XY(g.Ap[i])
		above := g.Idx(ax, ay-1)
		if ay <= 0 || !g.Cell[above] {
			continue
		}
		if sid := g.SurfAt[above]; sid >= 0 && g.Surfs[sid].Type == SurfSolid {
			continue
		}
		g.addSurface(Surface{ID: len(g.Surfs), Y: ay - 1, Left: ax, Right: ax, Type: SurfApple})
	}

	// Build BFS links for apple surfaces.
	b := newSurfBFS(g.NCells)
	for si := range g.Surfs {
		if g.Surfs[si].Type == SurfApple {
			b.buildLinksFor(g, si)
		}
	}
}

// UpdateAppleSurfaces marks eaten apple surfaces as SurfNone.
// Call after Turn() each round (after first).
func (g *Game) UpdateAppleSurfaces() {
	// Build set of current apple cells.
	appleAt := make([]bool, g.NCells)
	for i := 0; i < g.ANum; i++ {
		appleAt[g.Ap[i]] = true
	}

	// Apple surface sits at (Left, Y); the apple that grounds it is at (Left, Y+1).
	for i := range g.Surfs {
		s := &g.Surfs[i]
		if s.Type != SurfApple {
			continue
		}
		appleCell := g.Idx(s.Left, s.Y+1)
		if !appleAt[appleCell] {
			s.Type = SurfNone
		}
	}
}
