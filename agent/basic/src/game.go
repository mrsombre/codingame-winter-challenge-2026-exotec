package main

import (
	"bufio"
	"fmt"
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
	MaxW             = 45
	MaxH             = 30
	MaxCells         = MaxW * MaxH
	MaxExpandedCells = (MaxW + 4) * (MaxH + 4) // expanded grid with Oob=2
)

var Dl = [4][2]int{
	{0, -1}, // DU
	{1, 0},  // DR
	{0, 1},  // DD
	{-1, 0}, // DL
}

var Do = [4]int{DD, DL, DU, DR} // opposite direction

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
	Dir   int // head facing direction (DU/DR/DD/DL), derived from Body[0]→Body[1]
	Sp    int // body index of nearest supported segment from head; -1 if none
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
	ID     int
	Y      int // y of the walkable cells
	Left   int // leftmost x
	Right  int // rightmost x
	Len    int // Right - Left + 1
	Type   int // SurfSolid, SurfApple, SurfNone
	Links  []SurfLink
	Apples []AppleLink
}

// SurfLink represents a BFS-based directed connection between two surfaces.
type SurfLink struct {
	To      int   // target surface ID
	Landing int   // landing cell index (first cell reached on target surface)
	Len     int   // BFS step count
	Path    []int // full path: [source edge cell, ..., landing cell]
}

// AppleLink represents a shortest path from some cell on a surface to an apple.
type AppleLink struct {
	Apple int   // target apple cell
	Start int   // source cell on this surface where the path begins
	Len   int   // BFS step count from Start to Apple
	Path  []int // full path: [start cell, ..., apple cell]
}

// Constellation represents a cluster of spatially adjacent apples.
type Constellation struct {
	ID     int
	Apples []int // member apple cell indices
	Size   int
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

	// Surface graph data, populated by Plan.Init.
	Surfs     []Surface
	SurfAt    []int // cell -> surface index (-1 if not on surface)
	Clusters  []Constellation
	ClusterAt []int // apple cell -> cluster ID (-1 if not in cluster)

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

	// scratch buffer for Sp computation (allocated once)
	bodyOf []int
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
	g.bodyOf = make([]int, g.NCells)

	return g
}

// ParseBody parses "x,y:x,y:x,y" into sn, setting Body, Len, Dir, Alive.
func (g *Game) ParseBody(sn *Snake, line string) {
	seg := strings.Split(line, ":")
	if cap(sn.Body) >= len(seg) {
		sn.Body = sn.Body[:0]
	} else {
		sn.Body = make([]int, 0, len(seg))
	}
	for _, s := range seg {
		comma := strings.IndexByte(s, ',')
		x, _ := strconv.Atoi(s[:comma])
		y, _ := strconv.Atoi(s[comma+1:])
		sn.Body = append(sn.Body, g.Idx(x, y))
	}
	sn.Len = len(sn.Body)
	sn.Alive = true
	sn.Dir = g.DirFromTo(sn.Body[1], sn.Body[0])
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

	// snakes — preserve Body backing array for reuse
	for i := 0; i < g.SNum; i++ {
		g.Sn[i] = Snake{Body: g.Sn[i].Body[:0]}
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
		if g.IsMy(id) {
			sn.Owner = 0
		} else {
			sn.Owner = 1
		}
		g.ParseBody(sn, body)
	}

	// compute Sp (support point) for each snake
	// build body occupancy: cell -> snake index (-1 = unoccupied)
	bodyOf := g.bodyOf[:g.NCells]
	for i := range bodyOf {
		bodyOf[i] = -1
	}
	for i := 0; i < g.SNum; i++ {
		for _, c := range g.Sn[i].Body {
			if c >= 0 {
				bodyOf[c] = i
			}
		}
	}
	for i := 0; i < g.SNum; i++ {
		g.Sn[i].Sp = g.findSp(i, bodyOf)
	}
}

// findSp returns body index of the nearest segment from head that is supported.
// Supported = cell below (DD neighbor) is wall, apple, or another snake's body.
func (g *Game) findSp(si int, bodyOf []int) int {
	sn := &g.Sn[si]
	for i, c := range sn.Body {
		below := c + g.Stride
		if below < 0 || below >= g.NCells {
			continue
		}
		// wall
		if !g.Cell[below] {
			return i
		}
		// other snake
		if bodyOf[below] >= 0 && bodyOf[below] != si {
			return i
		}
		// apple
		for j := 0; j < g.ANum; j++ {
			if g.Ap[j] == below {
				return i
			}
		}
	}
	return -1
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

// DirFromTo returns the dominant direction from cell a to cell b.
// For adjacent cells this is exact (no division). For distant cells it picks
// the axis with greater delta; ties break to vertical (DU/DD).
// Returns -1 when a == b.
func (g *Game) DirFromTo(a, b int) int {
	delta := b - a
	if delta == 0 {
		return -1
	}
	// fast path: adjacent cells
	switch delta {
	case -g.Stride:
		return DU
	case 1:
		return DR
	case g.Stride:
		return DD
	case -1:
		return DL
	}
	// distant cells: dominant axis
	ax, ay := g.XY(a)
	bx, by := g.XY(b)
	dx, dy := bx-ax, by-ay
	adx, ady := dx, dy
	if adx < 0 {
		adx = -adx
	}
	if ady < 0 {
		ady = -ady
	}
	if ady >= adx {
		if dy < 0 {
			return DU
		}
		return DD
	}
	if dx > 0 {
		return DR
	}
	return DL
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
