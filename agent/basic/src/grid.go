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
	// Padded wall lookup: (W+2)*(H+2) flat bool array.
	// Index as WallPad[(y+1)*PadW+(x+1)]; border cells are true (wall).
	// Allows branch-free wall checks in BFS without OOB guards.
	WallPad []bool
	PadW    int // Width + 2
}

func NewAG(width, height int, walls map[Point]bool) *AGrid {
	padW := width + 2
	padH := height + 2
	g := &AGrid{
		Width: width, Height: height,
		Walls:    NewBG(width, height),
		WallBl:   NewBG(width, height),
		CellDirs: make([][]Direction, width*height),
		WallPad:  make([]bool, padW*padH),
		PadW:     padW,
	}
	// Fill border as walls
	for i := range g.WallPad {
		g.WallPad[i] = true
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			g.WallPad[(y+1)*padW+(x+1)] = false
		}
	}
	for p := range walls {
		if g.InB(p) {
			g.Walls.Set(p)
			g.WallPad[(p.Y+1)*padW+(p.X+1)] = true
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

// IsWallFast checks the padded wall array — no bounds check needed.
// Valid for any coordinate including OOB within ±1 of the grid.
func (g *AGrid) IsWallFast(p Point) bool {
	return g.WallPad[(p.Y+1)*g.PadW+(p.X+1)]
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

// IsActualWall returns true only for in-bounds wall tiles, matching the
// engine's collision check (Grid.Get returns NoTile for OOB with Type=-1,
// which is NOT TileWall). Moving the head out of bounds is legal as long
// as the snake still has support.
func (g *AGrid) IsActualWall(p Point) bool {
	if !g.InB(p) {
		return false
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
