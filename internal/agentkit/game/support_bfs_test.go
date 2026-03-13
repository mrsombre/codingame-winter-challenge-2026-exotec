package game

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"codingame/internal/engine/grid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stateFromSeed builds a game State from engine seed + leagueLevel.
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

	require.NotEmpty(t, spawns, "no spawn islands")
	body := spawns[0] // bot 0 body
	head := body[0]
	initRun := state.Terr.BodyInitRun(body)

	t.Logf("Map %dx%d, apples=%d, bot0 body=%+v initRun=%d",
		state.Grid.Width, state.Grid.Height, len(apples), body, initRun)

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
	assert.Nil(t, result)
}

func TestSupPathBFS_AdjacentTarget(t *testing.T) {
	layout := []string{
		".....",
		"#####",
	}
	terrain := sTerrainFromLayout(layout)

	result := terrain.SupPathBFS(Point{X: 2, Y: 0}, 1, Point{X: 3, Y: 0}, nil)
	require.NotNil(t, result, "expected path, got nil")
	assert.Equal(t, 1, result.MinLen)
	assert.Zero(t, result.Dist)
}

// TestSupReachMulti_MatchesPerTarget verifies SupReachMulti returns the same
// set of reachable targets as calling SupPathBFS per target with MinLen <= bodyLen.
func TestSupReachMulti_MatchesPerTarget(t *testing.T) {
	state, apples, spawns := stateFromSeed(2248502322264711400, 1)
	require.NotEmpty(t, spawns)

	body := spawns[0]
	head := body[0]
	initRun := state.Terr.BodyInitRun(body)
	bodyLen := len(body)

	// Old approach: per-target SupPathBFS.
	var oldReach []Point
	for _, apple := range apples {
		res := state.Terr.SupPathBFS(head, initRun, apple, &state.Apples)
		if res != nil && res.MinLen <= bodyLen {
			oldReach = append(oldReach, apple)
		}
	}

	// New approach: single multi-target BFS.
	newReach := state.Terr.SupReachMulti(head, initRun, bodyLen, apples, &state.Apples)

	// Sort both for comparison.
	sortPts := func(pts []Point) {
		sort.Slice(pts, func(i, j int) bool {
			if pts[i].Y != pts[j].Y {
				return pts[i].Y < pts[j].Y
			}
			return pts[i].X < pts[j].X
		})
	}
	sortPts(oldReach)
	sortPts(newReach)

	t.Logf("bodyLen=%d oldReach=%d newReach=%d", bodyLen, len(oldReach), len(newReach))
	assert.Equal(t, oldReach, newReach, "SupReachMulti must match per-target SupPathBFS results")
}

func TestSupReachMulti_Seed1001(t *testing.T) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	body := []Point{
		{X: 14, Y: 14}, {X: 14, Y: 13}, {X: 13, Y: 13},
		{X: 12, Y: 13}, {X: 12, Y: 14},
	}
	head := body[0]
	initRun := state.Terr.BodyInitRun(body)
	bodyLen := len(body)

	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, seed1001Apples)

	// Old.
	var oldReach []Point
	for _, apple := range seed1001Apples {
		res := state.Terr.SupPathBFS(head, initRun, apple, &srcBG)
		if res != nil && res.MinLen <= bodyLen {
			oldReach = append(oldReach, apple)
		}
	}

	// New.
	newReach := state.Terr.SupReachMulti(head, initRun, bodyLen, seed1001Apples, &srcBG)

	sortPts := func(pts []Point) {
		sort.Slice(pts, func(i, j int) bool {
			if pts[i].Y != pts[j].Y {
				return pts[i].Y < pts[j].Y
			}
			return pts[i].X < pts[j].X
		})
	}
	sortPts(oldReach)
	sortPts(newReach)

	t.Logf("bodyLen=%d oldReach=%d newReach=%d", bodyLen, len(oldReach), len(newReach))
	assert.Equal(t, oldReach, newReach)
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
			assert.Positive(t, got)
		})
	}
}
