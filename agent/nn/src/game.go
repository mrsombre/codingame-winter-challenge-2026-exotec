package main

import (
	"bufio"
	"strconv"
	"strings"
)

const (
	DU = 0
	DR = 1
	DD = 2
	DL = 3
)

const (
	MaxW     = 45
	MaxH     = 30
	MaxCells = MaxW * MaxH
	MaxASn   = 8
	MaxPSn   = 4
	MaxSeg   = 256
	MaxAp    = 128
	MaxTurns = 200
)

var Dl = [4][2]int{
	{0, -1},
	{1, 0},
	{0, 1},
	{-1, 0},
}

var Do = [4]int{DD, DL, DU, DR}

var Dn = [4]string{"UP", "RIGHT", "DOWN", "LEFT"}

type Snake struct {
	ID    int
	Owner int
	Body  []int
	Len   int
	Dir   int
	Alive bool
}

type Game struct {
	ID int

	W, H int
	Cell []bool
	Rows []string

	MyIDs [MaxPSn]int
	MyN   int
	OpIDs [MaxPSn]int
	OpN   int

	Sn   [MaxASn]Snake
	SNum int
	Ap   []int
	ANum int

	TurnNum int
}

func Init(s *bufio.Scanner) *Game {
	g := &Game{}
	if !scanInt(s, &g.ID) {
		return g
	}
	scanInt(s, &g.W)
	scanInt(s, &g.H)

	g.Cell = make([]bool, g.W*g.H)
	g.Rows = make([]string, g.H)
	for y := 0; y < g.H; y++ {
		s.Scan()
		row := s.Text()
		g.Rows[y] = row
		for x := 0; x < g.W; x++ {
			g.Cell[g.Idx(x, y)] = row[x] != '#'
		}
	}

	var perPlayer int
	scanInt(s, &perPlayer)
	g.MyN = perPlayer
	for i := 0; i < perPlayer; i++ {
		scanInt(s, &g.MyIDs[i])
	}
	g.OpN = perPlayer
	for i := 0; i < perPlayer; i++ {
		scanInt(s, &g.OpIDs[i])
	}
	g.Ap = make([]int, 0, MaxAp)
	return g
}

func scanInt(s *bufio.Scanner, out *int) bool {
	if !s.Scan() {
		return false
	}
	v, err := strconv.Atoi(strings.TrimSpace(s.Text()))
	if err != nil {
		return false
	}
	*out = v
	return true
}

func (g *Game) Turn(s *bufio.Scanner) bool {
	var appleCount int
	if !scanInt(s, &appleCount) {
		return false
	}
	g.TurnNum++
	g.ANum = appleCount
	g.Ap = g.Ap[:0]
	for i := 0; i < appleCount; i++ {
		if !s.Scan() {
			return false
		}
		var x, y int
		_, _ = strconv.Atoi("0")
		parseXYLine(s.Text(), &x, &y)
		g.Ap = append(g.Ap, g.Idx(x, y))
	}

	for i := 0; i < g.SNum; i++ {
		g.Sn[i] = Snake{Body: g.Sn[i].Body[:0]}
	}
	var snakeCount int
	if !scanInt(s, &snakeCount) {
		return false
	}
	g.SNum = snakeCount
	for i := 0; i < snakeCount; i++ {
		if !s.Scan() {
			return false
		}
		line := s.Text()
		space := strings.IndexByte(line, ' ')
		id, _ := strconv.Atoi(line[:space])
		body := line[space+1:]
		sn := &g.Sn[i]
		sn.ID = id
		if g.IsMy(id) {
			sn.Owner = 0
		} else {
			sn.Owner = 1
		}
		g.parseBody(sn, body)
	}
	return true
}

func parseXYLine(line string, x, y *int) {
	parts := strings.Fields(line)
	if len(parts) != 2 {
		return
	}
	*x, _ = strconv.Atoi(parts[0])
	*y, _ = strconv.Atoi(parts[1])
}

func (g *Game) parseBody(sn *Snake, line string) {
	parts := strings.Split(line, ":")
	if cap(sn.Body) < len(parts) {
		sn.Body = make([]int, 0, len(parts))
	} else {
		sn.Body = sn.Body[:0]
	}
	for _, part := range parts {
		comma := strings.IndexByte(part, ',')
		x, _ := strconv.Atoi(part[:comma])
		y, _ := strconv.Atoi(part[comma+1:])
		sn.Body = append(sn.Body, g.Idx(x, y))
	}
	sn.Len = len(sn.Body)
	sn.Alive = sn.Len > 0
	sn.Dir = DU
	if sn.Len > 1 {
		sn.Dir = g.DirFromTo(sn.Body[1], sn.Body[0])
	}
}

func (g *Game) Idx(x, y int) int {
	if x < 0 || x >= g.W || y < 0 || y >= g.H {
		return -1
	}
	return y*g.W + x
}

func (g *Game) XY(idx int) (int, int) {
	return idx % g.W, idx / g.W
}

func (g *Game) InBounds(x, y int) bool {
	return x >= 0 && x < g.W && y >= 0 && y < g.H
}

func (g *Game) IsInGrid(cell int) bool {
	return cell >= 0 && cell < len(g.Cell)
}

func (g *Game) IsMy(id int) bool {
	for i := 0; i < g.MyN; i++ {
		if g.MyIDs[i] == id {
			return true
		}
	}
	return false
}

func (g *Game) DirFromTo(a, b int) int {
	ax, ay := g.XY(a)
	bx, by := g.XY(b)
	switch {
	case bx == ax && by == ay-1:
		return DU
	case bx == ax+1 && by == ay:
		return DR
	case bx == ax && by == ay+1:
		return DD
	case bx == ax-1 && by == ay:
		return DL
	default:
		return DU
	}
}
