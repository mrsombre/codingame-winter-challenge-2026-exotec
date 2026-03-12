package agentkit

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

// --- DField -----------------------------------------------------------------

type DField struct {
	Width  int
	Height int
	Vals   []int
}

func (d DField) At(p Point) int {
	if p.X < 0 || p.X >= d.Width || p.Y < 0 || p.Y >= d.Height {
		return Unreachable
	}
	return d.Vals[p.Y*d.Width+p.X]
}

// --- AGrid BFS helpers ------------------------------------------------------

func (g *AGrid) AppleDist(apples *BitGrid) DField {
	n := g.Width * g.Height
	field := DField{
		Width:  g.Width,
		Height: g.Height,
		Vals:   make([]int, n),
	}
	for i := range field.Vals {
		field.Vals[i] = Unreachable
	}
	if apples == nil {
		return field
	}

	queue := make([]Point, 0, n)
	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			p := Point{X: x, Y: y}
			if g.IsWall(p) || !apples.Has(p) {
				continue
			}
			field.Vals[y*g.Width+x] = 0
			queue = append(queue, p)
		}
	}

	for i := 0; i < len(queue); i++ {
		p := queue[i]
		bd := field.Vals[p.Y*g.Width+p.X]
		for dir := DirUp; dir <= DirLeft; dir++ {
			next := Add(p, DirDelta[dir])
			if g.IsWall(next) {
				continue
			}
			ni := next.Y*g.Width + next.X
			nd := bd + 1
			if nd >= field.Vals[ni] {
				continue
			}
			field.Vals[ni] = nd
			queue = append(queue, next)
		}
	}

	return field
}

func (g *AGrid) Flood(start Point, occupied *BitGrid, maxN int) int {
	if maxN <= 0 || !g.InB(start) || g.IsWall(start) {
		return 0
	}

	visited := NewBG(g.Width, g.Height)
	visited.Set(start)
	queue := make([]Point, 1, maxN)
	queue[0] = start
	count := 0

	for i := 0; i < len(queue) && count < maxN; i++ {
		p := queue[i]
		count++
		for dir := DirUp; dir <= DirLeft; dir++ {
			next := Add(p, DirDelta[dir])
			if !g.InB(next) || g.IsWall(next) || visited.Has(next) {
				continue
			}
			if occupied != nil && occupied.Has(next) {
				continue
			}
			visited.Set(next)
			queue = append(queue, next)
		}
	}

	return count
}
