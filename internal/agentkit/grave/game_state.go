package game

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
	SimBuf   [MaxBody + 1]Point
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

// FloodDist returns (reachable count, per-cell BFS distance) from start.
// Allocates a new distance slice each call; reuses s.DistQ as queue scratch.
func (s *State) FloodDist(start Point, blocked *BitGrid) (int, []int) {
	g := s.Grid
	n := g.Width * g.Height
	dist := make([]int, n)
	for i := range dist {
		dist[i] = Unreachable
	}
	if g.IsWall(start) || (blocked != nil && blocked.Has(start)) {
		return 0, dist
	}
	dist[start.Y*g.Width+start.X] = 0
	q := s.DistQ[:0]
	q = append(q, start)
	count := 0
	for i := 0; i < len(q); i++ {
		p := q[i]
		count++
		d := dist[p.Y*g.Width+p.X]
		for dir := DirUp; dir <= DirLeft; dir++ {
			np := Add(p, DirDelta[dir])
			if g.IsWall(np) {
				continue
			}
			ni := np.Y*g.Width + np.X
			if dist[ni] != Unreachable || (blocked != nil && blocked.Has(np)) {
				continue
			}
			dist[ni] = d + 1
			q = append(q, np)
		}
	}
	s.DistQ = q[:0]
	return count, dist
}
