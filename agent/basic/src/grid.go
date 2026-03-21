package main

type Direction int

const (
	DirUp Direction = iota
	DirRight
	DirDown
	DirLeft
	DirNone
)

var DirName = [4]string{"UP", "RIGHT", "DOWN", "LEFT"}
var DirDelta = [5]Point{{0, -1}, {1, 0}, {0, 1}, {-1, 0}, {0, 0}}
var oppDir = [5]Direction{DirDown, DirLeft, DirUp, DirRight, DirNone}

func Opp(dir Direction) Direction { return oppDir[dir] }

var facingTbl = [9]Direction{DirNone, DirUp, DirNone, DirLeft, DirNone, DirRight, DirNone, DirDown, DirNone}

func FacingPts(head, neck Point) Direction {
	dx := head.X - neck.X
	dy := head.Y - neck.Y
	if dx < -1 || dx > 1 || dy < -1 || dy > 1 {
		return DirNone
	}
	return facingTbl[(dy+1)*3+(dx+1)]
}

const Unreachable = 9999

type Point struct{ X, Y int }

func Add(a, b Point) Point { return Point{a.X + b.X, a.Y + b.Y} }
func MDist(a, b Point) int { return abs(a.X-b.X) + abs(a.Y-b.Y) }
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type BitGrid struct {
	Width, Height int
	Bits          []uint64
}

func NewBG(width, height int) BitGrid {
	return BitGrid{width, height, make([]uint64, (width*height+63)/64)}
}
func (g *BitGrid) Reset() {
	for i := range g.Bits {
		g.Bits[i] = 0
	}
}
func (g *BitGrid) Has(p Point) bool {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return false
	}
	idx := p.Y*g.Width + p.X
	return g.Bits[idx/64]&(uint64(1)<<uint(idx%64)) != 0
}
func (g *BitGrid) Set(p Point) {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return
	}
	idx := p.Y*g.Width + p.X
	g.Bits[idx/64] |= uint64(1) << uint(idx%64)
}
func (g *BitGrid) Clear(p Point) {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return
	}
	idx := p.Y*g.Width + p.X
	g.Bits[idx/64] &^= uint64(1) << uint(idx%64)
}

type AGrid struct {
	Width, Height int
	Walls, WallBl BitGrid
	CellDirs      [][]Direction
}

func NewAG(width, height int, walls map[Point]bool) *AGrid {
	g := &AGrid{
		Width: width, Height: height,
		Walls:    NewBG(width, height),
		WallBl:   NewBG(width, height),
		CellDirs: make([][]Direction, width*height),
	}
	for p := range walls {
		if g.InB(p) {
			g.Walls.Set(p)
		}
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p := Point{x, y}
			if y == height-1 || g.IsWall(Point{x, y + 1}) {
				g.WallBl.Set(p)
			}
			if g.IsWall(p) {
				continue
			}
			var dirs []Direction
			for dir := DirUp; dir <= DirLeft; dir++ {
				if !g.IsWall(Add(p, DirDelta[dir])) {
					dirs = append(dirs, dir)
				}
			}
			g.CellDirs[y*width+x] = dirs
		}
	}
	return g
}
func (g *AGrid) InB(p Point) bool {
	return p.X >= 0 && p.X < g.Width && p.Y >= 0 && p.Y < g.Height
}
func (g *AGrid) IsWall(p Point) bool {
	if !g.InB(p) {
		return true
	}
	return g.Walls.Has(p)
}
func (g *AGrid) WBelow(p Point) bool { return g.WallBl.Has(p) }
func (g *AGrid) CDirs(pos Point) []Direction {
	if !g.InB(pos) || g.IsWall(pos) {
		return nil
	}
	return g.CellDirs[pos.Y*g.Width+pos.X]
}
func (g *AGrid) CIdx(p Point) int { return p.Y*g.Width + p.X }

func validDirs(facing Direction) ([4]Direction, int) {
	if facing == DirNone {
		return [4]Direction{DirUp, DirRight, DirDown, DirLeft}, 4
	}
	back := Opp(facing)
	var out [4]Direction
	n := 0
	for d := DirUp; d <= DirLeft; d++ {
		if d != back {
			out[n] = d
			n++
		}
	}
	return out, n
}

func adjCells(g *AGrid, target Point) ([4]Point, int) {
	var adj [4]Point
	n := 0
	for dir := DirUp; dir <= DirLeft; dir++ {
		p := Add(target, DirDelta[dir])
		if !g.IsWall(p) {
			adj[n] = p
			n++
		}
	}
	return adj, n
}

func fillBG(bg *BitGrid, pts []Point) {
	bg.Reset()
	for _, p := range pts {
		bg.Set(p)
	}
}

func occExcept(base *BitGrid, body []Point) BitGrid {
	bg := NewBG(base.Width, base.Height)
	copy(bg.Bits, base.Bits)
	for _, p := range body {
		bg.Clear(p)
	}
	return bg
}
