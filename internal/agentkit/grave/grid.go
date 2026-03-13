package grave

import "codingame/internal/agentkit/game"

type DFieldOld struct {
	Width  int
	Height int
	Vals   []int
}

func (d DFieldOld) At(p game.Point) int {
	if p.X < 0 || p.X >= d.Width || p.Y < 0 || p.Y >= d.Height {
		return game.Unreachable
	}
	return d.Vals[p.Y*d.Width+p.X]
}

func GridAppleDist(g *game.AGrid, apples *game.BitGrid) DFieldOld {
	n := g.Width * g.Height
	field := DFieldOld{
		Width:  g.Width,
		Height: g.Height,
		Vals:   make([]int, n),
	}
	for i := range field.Vals {
		field.Vals[i] = game.Unreachable
	}
	if apples == nil {
		return field
	}

	queue := make([]game.Point, 0, n)
	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			p := game.Point{X: x, Y: y}
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
		for dir := game.DirUp; dir <= game.DirLeft; dir++ {
			next := game.Add(p, game.DirDelta[dir])
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

func GridFlood(g *game.AGrid, start game.Point, occupied *game.BitGrid, maxN int) int {
	if maxN <= 0 || !g.InB(start) || g.IsWall(start) {
		return 0
	}

	visited := game.NewBG(g.Width, g.Height)
	visited.Set(start)
	queue := make([]game.Point, 1, maxN)
	queue[0] = start
	count := 0

	for i := 0; i < len(queue) && count < maxN; i++ {
		p := queue[i]
		count++
		for dir := game.DirUp; dir <= game.DirLeft; dir++ {
			next := game.Add(p, game.DirDelta[dir])
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
