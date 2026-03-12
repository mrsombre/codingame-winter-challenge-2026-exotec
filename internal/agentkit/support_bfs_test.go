package agentkit

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"codingame/internal/engine/grid"
)

// stateFromSeed builds an agentkit State from engine seed + leagueLevel.
// Returns state, apples list, and spawn islands (each island = sorted body coords).
func stateFromSeed(seed int64, leagueLevel int) (State, []Point, [][]Point) {
	rng := grid.NewSHA1PRNG(seed)
	gm := grid.NewGridMaker(rng, leagueLevel)
	g := gm.Make()

	walls := make(map[Point]bool)
	for y := 0; y < g.Height; y++ {
		for x := 0; x < g.Width; x++ {
			if g.GetXY(x, y).Type == grid.TileWall {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}

	ag := NewAG(g.Width, g.Height, walls)
	state := NewState(ag)

	apples := make([]Point, len(g.Apples))
	for i, c := range g.Apples {
		p := Point{X: c.X, Y: c.Y}
		apples[i] = p
		state.Apples.Set(p)
	}

	islands := g.DetectSpawnIslands()
	spawns := make([][]Point, len(islands))
	for i, island := range islands {
		coords := make([]grid.Coord, 0, len(island))
		for c := range island {
			coords = append(coords, c)
		}
		sort.Slice(coords, func(a, b int) bool {
			return coords[a].Less(coords[b])
		})
		body := make([]Point, len(coords))
		for j, c := range coords {
			body[j] = Point{X: c.X, Y: c.Y}
		}
		spawns[i] = body
	}

	return state, apples, spawns
}

func TestSupPathBFS_Seed2248_AllApples(t *testing.T) {
	state, apples, spawns := stateFromSeed(2248502322264711400, 1)

	if len(spawns) == 0 {
		t.Fatal("no spawn islands")
	}
	body := spawns[0] // bot 0 body
	head := body[0]
	initRun := state.Terr.BodyInitRun(body)

	t.Logf("Map %dx%d, apples=%d, bot0 body=%+v initRun=%d",
		state.Width(), state.Height(), len(apples), body, initRun)

	for _, apple := range apples {
		result := state.Terr.SupPathBFS(head, initRun, apple, &state.Apples)
		if result == nil {
			t.Logf("Apple (%d,%d): UNREACHABLE", apple.X, apple.Y)
			continue
		}

		var wpStrs []string
		for _, wp := range result.Waypoints {
			wpStrs = append(wpStrs, fmt.Sprintf("(%d,%d)", wp.X, wp.Y))
		}
		wpPath := strings.Join(wpStrs, "→")
		if wpPath == "" {
			wpPath = "(none)"
		}

		t.Logf("Apple (%d,%d): minLen=%d dist=%d approach=(%d,%d) supports: %s",
			apple.X, apple.Y,
			result.MinLen, result.Dist,
			result.Approach.X, result.Approach.Y,
			wpPath,
		)
	}
}

func TestSupPathBFS_Unreachable(t *testing.T) {
	layout := []string{
		"..#..",
		"..#..",
		"#####",
	}
	terrain := sTerrainFromLayout(layout)

	result := terrain.SupPathBFS(Point{X: 0, Y: 0}, 1, Point{X: 4, Y: 0}, nil)
	if result != nil {
		t.Fatalf("expected nil for unreachable target, got %+v", result)
	}
}

func TestSupPathBFS_AdjacentTarget(t *testing.T) {
	layout := []string{
		".....",
		"#####",
	}
	terrain := sTerrainFromLayout(layout)

	result := terrain.SupPathBFS(Point{X: 2, Y: 0}, 1, Point{X: 3, Y: 0}, nil)
	if result == nil {
		t.Fatal("expected path, got nil")
	}
	if result.MinLen != 1 {
		t.Fatalf("MinLen = %d, want 1", result.MinLen)
	}
	if result.Dist != 0 {
		t.Fatalf("Dist = %d, want 0 (start is already adjacent)", result.Dist)
	}
}

func TestBodyInitRun(t *testing.T) {
	state, _, spawns := stateFromSeed(2248502322264711400, 1)

	tests := []struct {
		name string
		body []Point
		want int
	}{
		{
			name: "spawn island 0",
			body: spawns[0],
			want: -1, // will be validated dynamically
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := state.Terr.BodyInitRun(tt.body)
			t.Logf("body=%+v initRun=%d", tt.body, got)
			if got < 1 {
				t.Fatalf("BodyInitRun = %d, expected positive", got)
			}
		})
	}
}
