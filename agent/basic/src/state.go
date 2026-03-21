package main

const MaxBody = 80
const MaxBirds = 8

type fBody struct {
	parts [MaxBody + 1]Point
	len   int
}

func (b *fBody) set(pts []Point) {
	b.len = len(pts)
	copy(b.parts[:b.len], pts)
}

func (b *fBody) slice() []Point { return b.parts[:b.len] }

func (b *fBody) facing() Direction {
	if b.len < 2 {
		return DirUp
	}
	return FacingPts(b.parts[0], b.parts[1])
}

func (b *fBody) contains(p Point) bool {
	for i := 0; i < b.len; i++ {
		if b.parts[i] == p {
			return true
		}
	}
	return false
}

type State struct {
	Grid    *AGrid
	Terr    *STerrain
	Apples  BitGrid
	MvBuf   [4]Direction
	OobDirs [4]Direction // all 4 cardinal directions for OOB positions
	DistQ   []Point
	// Distance array pool to avoid per-BFS allocations.
	distPool [][]int
}

func NewState(grid *AGrid) State {
	s := State{
		Grid:    grid,
		OobDirs: [4]Direction{DirUp, DirRight, DirDown, DirLeft},
	}
	if grid != nil {
		n := grid.Width * grid.Height
		s.Terr = NewSTerrain(grid)
		s.Apples = NewBG(grid.Width, grid.Height)
		s.DistQ = make([]Point, 0, n)
		// Pre-allocate pool of distance arrays
		s.distPool = make([][]int, 0, 16)
	}
	return s
}

// GetDist returns a distance array from the pool, or allocates a new one.
// The caller must call PutDist when done (unless the array is stored long-term).
func (s *State) GetDist() []int {
	n := s.Grid.Width * s.Grid.Height
	if len(s.distPool) > 0 {
		d := s.distPool[len(s.distPool)-1]
		s.distPool = s.distPool[:len(s.distPool)-1]
		for i := range d {
			d[i] = Unreachable
		}
		return d
	}
	d := make([]int, n)
	for i := range d {
		d[i] = Unreachable
	}
	return d
}

// PutDist returns a distance array to the pool for reuse.
func (s *State) PutDist(d []int) {
	s.distPool = append(s.distPool, d)
}
func (s *State) VMoves(pos Point, facing Direction) []Direction {
	dirs := s.Grid.CDirs(pos)
	if dirs == nil && !s.Grid.InB(pos) {
		// Head is out of bounds — no precomputed dirs, allow all 4.
		dirs = s.OobDirs[:]
	}
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

type DirInfo struct {
	flood int
	dists []int
	body  []Point
	alive bool
}

type SearchResult struct {
	dir    Direction
	target Point
	steps  int
	score  int
	ok     bool
}

type botPlan struct {
	id     int
	body   []Point
	facing Direction
	dir    Direction
	target Point
	reason string
	ok     bool
}

type botEntry struct {
	id   int
	body []Point
}

type enemyInfo struct {
	head    Point
	facing  Direction
	bodyLen int
	body    []Point
}

type supportJob struct {
	climberID int
	apple     Point
	cell      Point
	score     int
}
