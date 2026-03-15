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

var DirDelta = [4][2]int{
	{0, -1}, // DU
	{1, 0},  // DR
	{0, 1},  // DD
	{-1, 0}, // DL
}

var DirName = [4]string{"UP", "RIGHT", "DOWN", "LEFT"}

type Grid struct {
	W, H int
	Wall [(MaxCells + 63) / 64]uint64
	Nb   [MaxCells][4]int // precomputed neighbor index; -1 = blocked
}

// GFI reads w, h, and row strings from scanner and builds a Grid.
func GFI(s *bufio.Scanner) *Grid {
	var w, h int
	s.Scan()
	fmt.Sscan(s.Text(), &w)
	log(w)
	s.Scan()
	fmt.Sscan(s.Text(), &h)
	log(h)
	rows := make([]string, h)
	for i := 0; i < h; i++ {
		s.Scan()
		rows[i] = s.Text()
		log(rows[i])
	}
	return NewGrid(w, h, rows)
}

// NewGrid builds an immutable grid from '.#' row strings.
func NewGrid(w, h int, rows []string) *Grid {
	g := &Grid{W: w, H: h}
	// mark all neighbors invalid
	for i := 0; i < w*h; i++ {
		g.Nb[i] = [4]int{-1, -1, -1, -1}
	}
	// set walls
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if rows[y][x] == '#' {
				g.setWall(y*w + x)
			}
		}
	}
	// precompute neighbors
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			idx := y*w + x
			if g.IsWall(idx) {
				continue
			}
			for d := 0; d < 4; d++ {
				nx := x + DirDelta[d][0]
				ny := y + DirDelta[d][1]
				if nx < 0 || nx >= w || ny < 0 || ny >= h {
					continue
				}
				ni := ny*w + nx
				if g.IsWall(ni) {
					continue
				}
				g.Nb[idx][d] = ni
			}
		}
	}
	return g
}

func (g *Grid) setWall(idx int) {
	g.Wall[idx/64] |= 1 << uint(idx%64)
}

// IsWall returns true if cell at idx is a wall.
func (g *Grid) IsWall(idx int) bool {
	return g.Wall[idx/64]&(1<<uint(idx%64)) != 0
}

// Idx converts x,y coordinates to flat index.
func (g *Grid) Idx(x, y int) int {
	return y*g.W + x
}

// XY converts flat index to x,y coordinates.
func (g *Grid) XY(idx int) (int, int) {
	return idx % g.W, idx / g.W
}

// Nbs returns pointer to precomputed [DU,DR,DD,DL] neighbor indices; -1 = blocked.
func (g *Grid) Nbs(idx int) *[4]int {
	return &g.Nb[idx]
}
