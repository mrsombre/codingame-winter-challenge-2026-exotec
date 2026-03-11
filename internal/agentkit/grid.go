package agentkit

// Point is a generic integer grid coordinate for agent helpers.
type Point struct {
	X int
	Y int
}

// CardinalDirs matches the arena's four legal movement deltas.
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

// Grid is a finite playfield snapshot used by reusable search helpers.
type Grid struct {
	Width  int
	Height int
	Walls  map[Point]bool
}

func NewGrid(width, height int, walls map[Point]bool) Grid {
	return Grid{
		Width:  width,
		Height: height,
		Walls:  walls,
	}
}

func (g Grid) InBounds(p Point) bool {
	return p.X >= 0 && p.X < g.Width && p.Y >= 0 && p.Y < g.Height
}

func (g Grid) IsWall(p Point) bool {
	if !g.InBounds(p) {
		return true
	}
	return g.Walls[p]
}

// FloodFillWithDist performs BFS over non-wall, non-blocked cells.
func (g Grid) FloodFillWithDist(start Point, blocked map[Point]bool) (int, map[Point]int) {
	dists := make(map[Point]int, 128)
	if g.IsWall(start) || blocked[start] {
		return 0, dists
	}

	queue := make([]Point, 1, 128)
	queue[0] = start
	dists[start] = 0
	count := 0

	for i := 0; i < len(queue); i++ {
		p := queue[i]
		count++
		d := dists[p]

		for _, dir := range CardinalDirs {
			next := Add(p, dir)
			if _, seen := dists[next]; seen {
				continue
			}
			if g.IsWall(next) || blocked[next] {
				continue
			}
			dists[next] = d + 1
			queue = append(queue, next)
		}
	}

	return count, dists
}

func (g Grid) BFSDist(start Point, blocked map[Point]bool) map[Point]int {
	_, dists := g.FloodFillWithDist(start, blocked)
	return dists
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
