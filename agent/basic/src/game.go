package main

import (
	"bufio"
	"fmt"
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
	MaxAp  = 64  // max power sources (apples)
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

type Game struct {
	ID int

	W, H int
	Cell []bool   // false = wall, true = free
	Nb      [][4]int // precomputed neighbor index; -1 = blocked
	OobBase int      // start of OOB cell indices (= W*H)
	NCells  int      // total cells including OOB border (= W*H + 2*W + 2*H)

	MyIDs [MaxPSn]int // my snake IDs
	MyN   int
	OpIDs [MaxPSn]int // enemy snake IDs
	OpN   int

	// Turn data (overwritten each turn)
	Sn   [MaxASn]Snake
	SNum int
	Ap   []int // flat cell indices
	ANum int
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

	w, h := g.W, g.H
	n := w * h
	g.OobBase = n
	g.NCells = n + 2*w + 2*h
	g.Cell = make([]bool, n)
	g.Nb = make([][4]int, g.NCells)
	for i := 0; i < g.NCells; i++ {
		g.Nb[i] = [4]int{-1, -1, -1, -1}
	}

	// read rows and set walls
	for y := 0; y < h; y++ {
		s.Scan()
		row := s.Text()
		log(row)
		for x := 0; x < w; x++ {
			if row[x] != '#' {
				g.Cell[y*w+x] = true
			}
		}
	}

	// precompute neighbors for all cells (grid + OOB border)
	for cell := 0; cell < g.NCells; cell++ {
		cx, cy := g.CellXY(cell)
		for d := 0; d < 4; d++ {
			nx := cx + Dl[d][0]
			ny := cy + Dl[d][1]
			ni := g.CellIdx(nx, ny)
			if ni < 0 {
				continue
			}
			if ni < g.OobBase && !g.Cell[ni] {
				continue
			}
			g.Nb[cell][d] = ni
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

	return g
}

// Idx converts x,y coordinates to flat index.
func (g *Game) Idx(x, y int) int {
	return y*g.W + x
}

// XY converts flat index to x,y coordinates.
func (g *Game) XY(idx int) (int, int) {
	return idx % g.W, idx / g.W
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

// Read reads per-turn data from scanner: apples and snakes.
func (g *Game) Read(s *bufio.Scanner) {
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

// OobIdx returns the cell index for an out-of-bounds border position.
// Only positions exactly 1 cell outside the grid are supported.
// Returns -1 for positions 2+ cells out.
func (g *Game) OobIdx(x, y int) int {
	if y == -1 && x >= 0 && x < g.W {
		return g.OobBase + x
	}
	if y == g.H && x >= 0 && x < g.W {
		return g.OobBase + g.W + x
	}
	if x == -1 && y >= 0 && y < g.H {
		return g.OobBase + 2*g.W + y
	}
	if x == g.W && y >= 0 && y < g.H {
		return g.OobBase + 2*g.W + g.H + y
	}
	return -1
}

// CellXY returns x, y for any cell (grid or OOB border).
func (g *Game) CellXY(cell int) (int, int) {
	if cell < g.OobBase {
		return cell % g.W, cell / g.W
	}
	off := cell - g.OobBase
	if off < g.W {
		return off, -1
	}
	off -= g.W
	if off < g.W {
		return off, g.H
	}
	off -= g.W
	if off < g.H {
		return -1, off
	}
	off -= g.H
	return g.W, off
}

// CellIdx returns the cell index for (x, y), grid or OOB border.
// Returns -1 if out of supported range (2+ cells outside).
func (g *Game) CellIdx(x, y int) int {
	if x >= 0 && x < g.W && y >= 0 && y < g.H {
		return g.Idx(x, y)
	}
	return g.OobIdx(x, y)
}

// ParseBody parses "x,y:x,y:x,y" into flat cell indices.
// OOB positions 1 cell outside get OOB cell indices; further out get -1.
func (g *Game) ParseBody(s string) []int {
	dst := make([]int, 0, 8)
	i := 0
	for i < len(s) {
		neg := false
		if i < len(s) && s[i] == '-' {
			neg = true
			i++
		}
		x := 0
		for i < len(s) && s[i] != ',' {
			x = x*10 + int(s[i]-'0')
			i++
		}
		if neg {
			x = -x
		}
		i++ // skip ','
		neg = false
		if i < len(s) && s[i] == '-' {
			neg = true
			i++
		}
		y := 0
		for i < len(s) && s[i] != ':' {
			y = y*10 + int(s[i]-'0')
			i++
		}
		if neg {
			y = -y
		}
		i++ // skip ':'
		dst = append(dst, g.CellIdx(x, y))
	}
	return dst
}
