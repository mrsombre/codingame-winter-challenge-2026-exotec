package agentkit

const UnreachableDistance = 9999

type cellMoves [5][]Direction

// ArenaGrid stores static per-map caches that are worth building once.
type ArenaGrid struct {
	Width      int
	Height     int
	walls      []bool
	wallBelow  []bool
	validMoves []cellMoves
}

// DistanceField stores shortest-path distances over an arena grid.
type DistanceField struct {
	Width  int
	Height int
	Values []int
}

func NewArenaGrid(width, height int, walls map[Point]bool) *ArenaGrid {
	grid := &ArenaGrid{
		Width:      width,
		Height:     height,
		walls:      make([]bool, width*height),
		wallBelow:  make([]bool, width*height),
		validMoves: make([]cellMoves, width*height),
	}

	for p := range walls {
		if grid.InBounds(p) {
			grid.walls[grid.cellIdx(p)] = true
		}
	}

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p := Point{X: x, Y: y}
			idx := grid.cellIdx(p)

			grid.wallBelow[idx] = y == height-1 || grid.IsWall(Point{X: x, Y: y + 1})
			if grid.walls[idx] {
				continue
			}

			for facing := DirUp; facing <= DirNone; facing++ {
				var dirs []Direction
				back := DirNone
				if facing != DirNone {
					back = Opposite(facing)
				}
				for dir := DirUp; dir <= DirLeft; dir++ {
					if dir == back {
						continue
					}
					next := Add(p, DirectionDeltas[dir])
					if !grid.IsWall(next) {
						dirs = append(dirs, dir)
					}
				}
				grid.validMoves[idx][facing] = dirs
			}
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
	return g.walls[g.cellIdx(p)]
}

func (g *ArenaGrid) WallBelow(p Point) bool {
	if !g.InBounds(p) {
		return false
	}
	return g.wallBelow[g.cellIdx(p)]
}

func (g *ArenaGrid) ValidMoves(pos Point, facing Direction) []Direction {
	if !g.InBounds(pos) || g.IsWall(pos) {
		return nil
	}
	return g.validMoves[g.cellIdx(pos)][normalizeFacing(facing)]
}

func (g *ArenaGrid) AppleDistanceField(apples *BitGrid) DistanceField {
	field := DistanceField{
		Width:  g.Width,
		Height: g.Height,
		Values: make([]int, len(g.walls)),
	}
	for i := range field.Values {
		field.Values[i] = UnreachableDistance
	}
	if apples == nil {
		return field
	}

	queue := make([]Point, 0, g.Width*g.Height)
	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			p := Point{X: x, Y: y}
			if g.IsWall(p) || !apples.Has(p) {
				continue
			}
			idx := g.cellIdx(p)
			field.Values[idx] = 0
			queue = append(queue, p)
		}
	}

	for i := 0; i < len(queue); i++ {
		p := queue[i]
		baseDist := field.Values[g.cellIdx(p)]
		for dir := DirUp; dir <= DirLeft; dir++ {
			next := Add(p, DirectionDeltas[dir])
			if g.IsWall(next) {
				continue
			}
			nextIdx := g.cellIdx(next)
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

func (g *ArenaGrid) cellIdx(p Point) int {
	return p.Y*g.Width + p.X
}

func normalizeFacing(facing Direction) Direction {
	if facing < DirUp || facing > DirNone {
		return DirNone
	}
	return facing
}

func (d DistanceField) At(p Point) int {
	if p.X < 0 || p.X >= d.Width || p.Y < 0 || p.Y >= d.Height {
		return UnreachableDistance
	}
	return d.Values[p.Y*d.Width+p.X]
}
