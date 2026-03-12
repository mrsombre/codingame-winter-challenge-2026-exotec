package agentkit

const UnreachableDistance = 9999

// Point is an integer grid coordinate.
type Point struct {
	X int
	Y int
}

var CardinalDirs = []Point{
	{X: 0, Y: -1},
	{X: 1, Y: 0},
	{X: 0, Y: 1},
	{X: -1, Y: 0},
}

func Add(a, b Point) Point {
	return Point{X: a.X + b.X, Y: a.Y + b.Y}
}

func ManhattanDistance(a, b Point) int {
	return abs(a.X-b.X) + abs(a.Y-b.Y)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// --- BitGrid ----------------------------------------------------------------

// BitGrid is a compact occupancy grid backed by a uint64 bitset.
type BitGrid struct {
	Width  int
	Height int
	bits   []uint64
}

func NewBitGrid(width, height int) BitGrid {
	cells := width * height
	return BitGrid{
		Width:  width,
		Height: height,
		bits:   make([]uint64, (cells+63)/64),
	}
}

func (g *BitGrid) Reset() {
	for i := range g.bits {
		g.bits[i] = 0
	}
}

func (g *BitGrid) Has(p Point) bool {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return false
	}
	idx := p.Y*g.Width + p.X
	return g.bits[idx/64]&(uint64(1)<<uint(idx%64)) != 0
}

func (g *BitGrid) Set(p Point) {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return
	}
	idx := p.Y*g.Width + p.X
	g.bits[idx/64] |= uint64(1) << uint(idx%64)
}

func (g *BitGrid) Clear(p Point) {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return
	}
	idx := p.Y*g.Width + p.X
	g.bits[idx/64] &^= uint64(1) << uint(idx%64)
}

// --- ArenaGrid --------------------------------------------------------------

// ArenaGrid is an immutable map structure built once from init input.
// Stores walls, wall-below flags, and precomputed cell directions.
type ArenaGrid struct {
	Width     int
	Height    int
	walls     BitGrid
	wallBelow BitGrid
	cellDirs  [][]Direction
}

func NewArenaGrid(width, height int, walls map[Point]bool) *ArenaGrid {
	grid := &ArenaGrid{
		Width:     width,
		Height:    height,
		walls:     NewBitGrid(width, height),
		wallBelow: NewBitGrid(width, height),
		cellDirs:  make([][]Direction, width*height),
	}

	for p := range walls {
		if grid.InBounds(p) {
			grid.walls.Set(p)
		}
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p := Point{X: x, Y: y}

			if y == height-1 || grid.IsWall(Point{X: x, Y: y + 1}) {
				grid.wallBelow.Set(p)
			}
			if grid.IsWall(p) {
				continue
			}

			var dirs []Direction
			for dir := DirUp; dir <= DirLeft; dir++ {
				next := Add(p, DirectionDeltas[dir])
				if !grid.IsWall(next) {
					dirs = append(dirs, dir)
				}
			}
			grid.cellDirs[y*width+x] = dirs
		}
	}

	return grid
}

func (g *ArenaGrid) InBounds(p Point) bool {
	return p.X >= 0 && p.X < g.Width && p.Y >= 0 && p.Y < g.Height
}

func (g *ArenaGrid) IsWall(p Point) bool {
	if !g.InBounds(p) {
		return true
	}
	return g.walls.Has(p)
}

func (g *ArenaGrid) WallBelow(p Point) bool {
	return g.wallBelow.Has(p)
}

// CellDirs returns all non-wall neighbor directions for a cell.
func (g *ArenaGrid) CellDirs(pos Point) []Direction {
	if !g.InBounds(pos) || g.IsWall(pos) {
		return nil
	}
	return g.cellDirs[pos.Y*g.Width+pos.X]
}

// CellIdx returns the flat index for a point in the grid.
func (g *ArenaGrid) CellIdx(p Point) int {
	return p.Y*g.Width + p.X
}

// --- DistanceField ----------------------------------------------------------

// DistanceField stores shortest-path distances over an arena grid.
type DistanceField struct {
	Width  int
	Height int
	Values []int
}

func (d DistanceField) At(p Point) int {
	if p.X < 0 || p.X >= d.Width || p.Y < 0 || p.Y >= d.Height {
		return UnreachableDistance
	}
	return d.Values[p.Y*d.Width+p.X]
}

// --- ArenaGrid BFS helpers --------------------------------------------------

// AppleDistanceField computes BFS distance from every cell to the nearest apple.
func (g *ArenaGrid) AppleDistanceField(apples *BitGrid) DistanceField {
	n := g.Width * g.Height
	field := DistanceField{
		Width:  g.Width,
		Height: g.Height,
		Values: make([]int, n),
	}
	for i := range field.Values {
		field.Values[i] = UnreachableDistance
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
			field.Values[y*g.Width+x] = 0
			queue = append(queue, p)
		}
	}

	for i := 0; i < len(queue); i++ {
		p := queue[i]
		baseDist := field.Values[p.Y*g.Width+p.X]
		for dir := DirUp; dir <= DirLeft; dir++ {
			next := Add(p, DirectionDeltas[dir])
			if g.IsWall(next) {
				continue
			}
			nextIdx := next.Y*g.Width + next.X
			nextDist := baseDist + 1
			if nextDist >= field.Values[nextIdx] {
				continue
			}
			field.Values[nextIdx] = nextDist
			queue = append(queue, next)
		}
	}

	return field
}

// FloodCount does a bounded BFS from start through open, unoccupied cells.
func (g *ArenaGrid) FloodCount(start Point, occupied *BitGrid, maxCount int) int {
	if maxCount <= 0 || !g.InBounds(start) || g.IsWall(start) {
		return 0
	}

	visited := NewBitGrid(g.Width, g.Height)
	visited.Set(start)
	queue := make([]Point, 1, maxCount)
	queue[0] = start
	count := 0

	for i := 0; i < len(queue) && count < maxCount; i++ {
		p := queue[i]
		count++

		for dir := DirUp; dir <= DirLeft; dir++ {
			next := Add(p, DirectionDeltas[dir])
			if !g.InBounds(next) || g.IsWall(next) || visited.Has(next) {
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
