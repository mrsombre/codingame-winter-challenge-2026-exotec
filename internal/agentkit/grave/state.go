package grave

import "codingame/internal/agentkit/game"

// StateAppleDist computes BFS distance from every cell to nearest apple.
// Uses scratch buffers — returned DField valid until next call.
func StateAppleDist(s *game.State) DFieldOld {
	g := s.Grid
	w, h := g.Width, g.Height
	vals := s.DistVals
	for i := range vals {
		vals[i] = game.Unreachable
	}

	q := s.DistQ[:0]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := game.Point{X: x, Y: y}
			if g.IsWall(p) || !s.Apples.Has(p) {
				continue
			}
			vals[y*w+x] = 0
			q = append(q, p)
		}
	}

	for i := 0; i < len(q); i++ {
		p := q[i]
		bd := vals[p.Y*w+p.X]
		for dir := game.DirUp; dir <= game.DirLeft; dir++ {
			next := game.Add(p, game.DirDelta[dir])
			if g.IsWall(next) {
				continue
			}
			ni := next.Y*w + next.X
			nd := bd + 1
			if nd >= vals[ni] {
				continue
			}
			vals[ni] = nd
			q = append(q, next)
		}
	}
	s.DistQ = q[:0]

	return DFieldOld{Width: w, Height: h, Vals: vals}
}

// StateFlood does bounded BFS from start through open, unoccupied cells.
// Uses scratch buffers — zero alloc per call.
func StateFlood(s *game.State, start game.Point, occupied *game.BitGrid, maxN int) int {
	g := s.Grid
	if maxN <= 0 || !g.InB(start) || g.IsWall(start) {
		return 0
	}

	s.FloodVis.Reset()
	s.FloodVis.Set(start)
	q := s.FloodQ[:0]
	q = append(q, start)
	count := 0

	for i := 0; i < len(q) && count < maxN; i++ {
		p := q[i]
		count++
		for dir := game.DirUp; dir <= game.DirLeft; dir++ {
			next := game.Add(p, game.DirDelta[dir])
			if !g.InB(next) || g.IsWall(next) || s.FloodVis.Has(next) {
				continue
			}
			if occupied != nil && occupied.Has(next) {
				continue
			}
			s.FloodVis.Set(next)
			q = append(q, next)
		}
	}
	s.FloodQ = q[:0]

	return count
}

// RebuildSup rebuilds the apple support map in s.
func RebuildSup(s *game.State) {
	if s == nil || s.Grid == nil {
		return
	}
	s.AppleSup = make(map[game.Point][]game.TAppr, len(s.AppleSup))

	w, h := s.Grid.Width, s.Grid.Height
	targets := make([]game.Point, 0, 64)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := game.Point{X: x, Y: y}
			if s.Apples.Has(p) {
				targets = append(targets, p)
			}
		}
	}

	sym := HasMirror(s)
	for _, target := range targets {
		if _, done := s.AppleSup[target]; done {
			continue
		}
		s.AppleSup[target] = game.CloseSup(s, target)
		if !sym {
			continue
		}
		mt := MirrorPt(w, target)
		if mt == target || !s.Apples.Has(mt) {
			continue
		}
		s.AppleSup[mt] = MirrorAppr(w, s.AppleSup[target])
	}
}

func HasMirror(s *game.State) bool {
	if s == nil || s.Grid == nil {
		return false
	}
	w, h := s.Grid.Width, s.Grid.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := game.Point{X: x, Y: y}
			if s.Apples.Has(p) != s.Apples.Has(MirrorPt(w, p)) {
				return false
			}
		}
	}
	return true
}

func MirrorPt(width int, p game.Point) game.Point {
	return game.Point{X: width - 1 - p.X, Y: p.Y}
}

func MirrorAppr(width int, appr []game.TAppr) []game.TAppr {
	m := make([]game.TAppr, len(appr))
	for i, a := range appr {
		m[i] = game.TAppr{
			Cell: MirrorPt(width, a.Cell),
			MinL: a.MinL,
		}
	}
	return m
}
