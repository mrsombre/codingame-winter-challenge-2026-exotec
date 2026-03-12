package agentkit

// BotState is a lightweight snapshot of a single bot.
type BotState struct {
	ID    int
	Owner int
	Alive bool
	Body  Body
}

// State is the primary type agents use. It holds the immutable grid,
// precomputed terrain, and per-turn mutable data.
type State struct {
	// Immutable — set once at init, never modified.
	Grid    *ArenaGrid
	Terrain *SupportTerrain

	// Per-turn — reset and repopulated each turn.
	Apples        BitGrid
	Bots          []BotState
	AppleSupports map[Point][]TargetApproach

	// Scratch — reused across calls within a turn.
	moveBuf [4]Direction
}

func NewState(grid *ArenaGrid) State {
	s := State{Grid: grid}
	if grid != nil {
		s.Terrain = NewSupportTerrain(grid)
		s.Apples = NewBitGrid(grid.Width, grid.Height)
		s.AppleSupports = map[Point][]TargetApproach{}
	}
	return s
}

// --- Immutable grid delegates -----------------------------------------------

func (s *State) Width() int  { return s.Grid.Width }
func (s *State) Height() int { return s.Grid.Height }

func (s *State) InBounds(p Point) bool  { return s.Grid.InBounds(p) }
func (s *State) IsWall(p Point) bool    { return s.Grid.IsWall(p) }
func (s *State) WallBelow(p Point) bool { return s.Grid.WallBelow(p) }
func (s *State) CellDirs(pos Point) []Direction { return s.Grid.CellDirs(pos) }
func (s *State) CellIdx(p Point) int            { return s.Grid.CellIdx(p) }

// --- Per-turn helpers -------------------------------------------------------

// ValidMoves returns non-wall directions excluding the back of facing.
// Uses a scratch buffer — returned slice is valid until the next call.
func (s *State) ValidMoves(pos Point, facing Direction) []Direction {
	dirs := s.Grid.CellDirs(pos)
	if facing == DirNone {
		return dirs
	}
	back := Opposite(facing)
	n := 0
	for _, d := range dirs {
		if d != back {
			s.moveBuf[n] = d
			n++
		}
	}
	return s.moveBuf[:n]
}

// AppleDistanceField computes BFS distance from every cell to the nearest apple.
func (s *State) AppleDistanceField() DistanceField {
	return s.Grid.AppleDistanceField(&s.Apples)
}

// FloodCount does a bounded BFS from start through open, unoccupied cells.
func (s *State) FloodCount(start Point, occupied *BitGrid, maxCount int) int {
	return s.Grid.FloodCount(start, occupied, maxCount)
}

// --- Apple support rebuild --------------------------------------------------

func (s *State) RebuildAppleSupports() {
	if s == nil || s.Grid == nil {
		return
	}
	if s.AppleSupports == nil {
		s.AppleSupports = map[Point][]TargetApproach{}
	} else {
		for target := range s.AppleSupports {
			delete(s.AppleSupports, target)
		}
	}

	w, h := s.Grid.Width, s.Grid.Height
	targets := make([]Point, 0, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := Point{X: x, Y: y}
			if s.Apples.Has(p) {
				targets = append(targets, p)
			}
		}
	}

	symmetric := s.hasMirrorSymmetricApples()
	for _, target := range targets {
		if _, done := s.AppleSupports[target]; done {
			continue
		}

		s.AppleSupports[target] = ClosestSupports(s, target)
		if !symmetric {
			continue
		}

		mirroredTarget := mirrorPoint(w, target)
		if mirroredTarget == target || !s.Apples.Has(mirroredTarget) {
			continue
		}
		s.AppleSupports[mirroredTarget] = mirrorApproaches(w, s.AppleSupports[target])
	}
}

func (s *State) hasMirrorSymmetricApples() bool {
	if s == nil || s.Grid == nil {
		return false
	}
	w, h := s.Grid.Width, s.Grid.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			p := Point{X: x, Y: y}
			if s.Apples.Has(p) != s.Apples.Has(mirrorPoint(w, p)) {
				return false
			}
		}
	}
	return true
}

func mirrorPoint(width int, p Point) Point {
	return Point{X: width - 1 - p.X, Y: p.Y}
}

func mirrorApproaches(width int, approaches []TargetApproach) []TargetApproach {
	mirrored := make([]TargetApproach, len(approaches))
	for i, approach := range approaches {
		mirrored[i] = TargetApproach{
			SupportCell: mirrorPoint(width, approach.SupportCell),
			MinLen:      approach.MinLen,
		}
	}
	return mirrored
}
