package agentkit

type Bot struct {
	ID    int
	Owner int
	Alive bool
	Body  Body
}

type State struct {
	// Immutable
	Grid *AGrid
	Terr *STerrain

	// Per-turn
	Apples   BitGrid
	Bots     []Bot
	AppleSup map[Point][]TAppr

	// Scratch
	MvBuf    [4]Direction
	FloodVis BitGrid
	FloodQ   []Point
	DistVals []int
	DistQ    []Point
}

func NewState(grid *AGrid) State {
	s := State{Grid: grid}
	if grid != nil {
		n := grid.Width * grid.Height
		s.Terr = NewSTerrain(grid)
		s.Apples = NewBG(grid.Width, grid.Height)
		s.AppleSup = make(map[Point][]TAppr)
		s.FloodVis = NewBG(grid.Width, grid.Height)
		s.FloodQ = make([]Point, 0, n)
		s.DistVals = make([]int, n)
		s.DistQ = make([]Point, 0, n)
	}
	return s
}

// --- Immutable grid delegates -----------------------------------------------

func (s *State) Width() int  { return s.Grid.Width }
func (s *State) Height() int { return s.Grid.Height }

func (s *State) InB(p Point) bool            { return s.Grid.InB(p) }
func (s *State) IsWall(p Point) bool         { return s.Grid.IsWall(p) }
func (s *State) WBelow(p Point) bool         { return s.Grid.WBelow(p) }
func (s *State) CDirs(pos Point) []Direction { return s.Grid.CDirs(pos) }
func (s *State) CIdx(p Point) int            { return s.Grid.CIdx(p) }

// --- Per-turn helpers -------------------------------------------------------

// VMoves returns non-wall directions excluding back of facing.
// Uses scratch buffer — valid until next call.
func (s *State) VMoves(pos Point, facing Direction) []Direction {
	dirs := s.Grid.CDirs(pos)
	if facing == DirNone {
		return dirs
	}
	back := Opp(facing)
	n := 0
	for _, d := range dirs {
		if d != back {
			s.MvBuf[n] = d
			n++
		}
	}
	return s.MvBuf[:n]
}

// AppleDist computes BFS distance from every cell to nearest apple.
// Uses scratch buffers — returned DField valid until next call.
func (s *State) AppleDist() DField {
	g := s.Grid
	w, h := g.Width, g.Height
	vals := s.DistVals
	for i := range vals {
		vals[i] = Unreachable
	}

	q := s.DistQ[:0]
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := Point{X: x, Y: y}
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
		for dir := DirUp; dir <= DirLeft; dir++ {
			next := Add(p, DirDelta[dir])
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

	return DField{Width: w, Height: h, Vals: vals}
}

// Flood does bounded BFS from start through open, unoccupied cells.
// Uses scratch buffers — zero alloc per call.
func (s *State) Flood(start Point, occupied *BitGrid, maxN int) int {
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
		for dir := DirUp; dir <= DirLeft; dir++ {
			next := Add(p, DirDelta[dir])
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

// --- Apple support rebuild --------------------------------------------------

func (s *State) RebuildSup() {
	if s == nil || s.Grid == nil {
		return
	}
	s.AppleSup = make(map[Point][]TAppr, len(s.AppleSup))

	w, h := s.Grid.Width, s.Grid.Height
	targets := make([]Point, 0, 64)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := Point{X: x, Y: y}
			if s.Apples.Has(p) {
				targets = append(targets, p)
			}
		}
	}

	sym := s.HasMirror()
	for _, target := range targets {
		if _, done := s.AppleSup[target]; done {
			continue
		}
		s.AppleSup[target] = CloseSup(s, target)
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

func (s *State) HasMirror() bool {
	if s == nil || s.Grid == nil {
		return false
	}
	w, h := s.Grid.Width, s.Grid.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := Point{X: x, Y: y}
			if s.Apples.Has(p) != s.Apples.Has(MirrorPt(w, p)) {
				return false
			}
		}
	}
	return true
}

func MirrorPt(width int, p Point) Point {
	return Point{X: width - 1 - p.X, Y: p.Y}
}

func MirrorAppr(width int, appr []TAppr) []TAppr {
	m := make([]TAppr, len(appr))
	for i, a := range appr {
		m[i] = TAppr{
			Cell: MirrorPt(width, a.Cell),
			MinL: a.MinL,
		}
	}
	return m
}
