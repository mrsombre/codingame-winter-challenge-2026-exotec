package game

const Unreachable = 9999

type Point struct {
	X int
	Y int
}

func Add(a, b Point) Point {
	return Point{X: a.X + b.X, Y: a.Y + b.Y}
}

func MDist(a, b Point) int {
	return abs(a.X-b.X) + abs(a.Y-b.Y)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// --- BitGrid ----------------------------------------------------------------

type BitGrid struct {
	Width  int
	Height int
	Bits   []uint64
}

func NewBG(width, height int) BitGrid {
	cells := width * height
	return BitGrid{
		Width:  width,
		Height: height,
		Bits:   make([]uint64, (cells+63)/64),
	}
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

// --- AGrid ------------------------------------------------------------------

type AGrid struct {
	Width    int
	Height   int
	Walls    BitGrid
	WallBl   BitGrid
	CellDirs [][]Direction
}

func NewAG(width, height int, walls map[Point]bool) *AGrid {
	g := &AGrid{
		Width:    width,
		Height:   height,
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
			p := Point{X: x, Y: y}
			if y == height-1 || g.IsWall(Point{X: x, Y: y + 1}) {
				g.WallBl.Set(p)
			}
			if g.IsWall(p) {
				continue
			}
			var dirs []Direction
			for dir := DirUp; dir <= DirLeft; dir++ {
				next := Add(p, DirDelta[dir])
				if !g.IsWall(next) {
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

func (g *AGrid) WBelow(p Point) bool {
	return g.WallBl.Has(p)
}

func (g *AGrid) CDirs(pos Point) []Direction {
	if !g.InB(pos) || g.IsWall(pos) {
		return nil
	}
	return g.CellDirs[pos.Y*g.Width+pos.X]
}

func (g *AGrid) CIdx(p Point) int {
	return p.Y*g.Width + p.X
}

// FillBG resets bg then sets every point in pts.
func FillBG(bg *BitGrid, pts []Point) {
	bg.Reset()
	for _, p := range pts {
		bg.Set(p)
	}
}

// OccExcept returns a copy of base with all body positions cleared.
func OccExcept(base *BitGrid, body []Point) BitGrid {
	bg := NewBG(base.Width, base.Height)
	copy(bg.Bits, base.Bits)
	for _, p := range body {
		bg.Clear(p)
	}
	return bg
}
