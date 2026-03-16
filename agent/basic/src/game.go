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
	Nb   [][4]int // precomputed neighbor index; -1 = blocked
	Edge []bool   // wall with free cell above and free left or right neighbor

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
	g.Cell = make([]bool, n)
	g.Nb = make([][4]int, n)
	for i := 0; i < n; i++ {
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

	// precompute neighbors for all cells
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := g.Idx(x, y)
			for d := 0; d < 4; d++ {
				nx := x + Dl[d][0]
				ny := y + Dl[d][1]
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					continue
				}
				ni := g.Idx(nx, ny)
				if !g.Cell[ni] {
					continue
				}
				g.Nb[idx][d] = ni
			}
		}
	}

	// precompute edges: wall cells with free above and free left or right
	g.Edge = make([]bool, n)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := g.Idx(x, y)
			if g.Cell[idx] {
				continue // free cell, not a wall
			}
			if g.Nb[idx][DU] == -1 {
				continue // no free cell above
			}
			if g.Nb[idx][DL] != -1 || g.Nb[idx][DR] != -1 {
				g.Edge[idx] = true
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

// ParseBody parses "x,y:x,y:x,y" into flat cell indices.
// Segments with out-of-bounds coordinates are stored as -1.
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
		if x < 0 || x >= g.W || y < 0 || y >= g.H {
			dst = append(dst, -1) // sentinel for out-of-bounds
		} else {
			dst = append(dst, g.Idx(x, y))
		}
	}
	return dst
}
