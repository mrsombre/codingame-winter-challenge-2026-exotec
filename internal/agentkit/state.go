package agentkit

// BotState is a lightweight public snapshot for planner helpers.
type BotState struct {
	ID    int
	Owner int
	Alive bool
	Body  Body
}

// State stores the current board snapshot plus static per-map caches.
//
// All fields are public by design so agents can use it as a plain data bag.
type State struct {
	Grid          *ArenaGrid
	Terrain       *SupportTerrain
	Apples        BitGrid
	Bots          []BotState
	AppleSupports map[Point][]TargetApproach
}

func NewState(grid *ArenaGrid) State {
	state := State{Grid: grid}
	if grid != nil {
		state.Terrain = NewSupportTerrain(grid)
		state.Apples = NewBitGrid(grid.Width, grid.Height)
		state.AppleSupports = map[Point][]TargetApproach{}
	}
	return state
}

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

	targets := make([]Point, 0, s.Grid.Width*s.Grid.Height)
	for y := 0; y < s.Grid.Height; y++ {
		for x := 0; x < s.Grid.Width; x++ {
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

		mirroredTarget := mirrorPoint(s.Grid.Width, target)
		if mirroredTarget == target || !s.Apples.Has(mirroredTarget) {
			continue
		}
		s.AppleSupports[mirroredTarget] = mirrorApproaches(s.Grid.Width, s.AppleSupports[target])
	}
}

func (s *State) hasMirrorSymmetricApples() bool {
	if s == nil || s.Grid == nil {
		return false
	}
	for y := 0; y < s.Grid.Height; y++ {
		for x := 0; x < s.Grid.Width; x++ {
			p := Point{X: x, Y: y}
			if s.Apples.Has(p) != s.Apples.Has(mirrorPoint(s.Grid.Width, p)) {
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
