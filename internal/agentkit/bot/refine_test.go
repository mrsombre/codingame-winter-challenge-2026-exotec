package bot

import (
	"testing"
	"time"

	"codingame/internal/agentkit/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

var seed1001Layout = []string{
	".#............................#.",
	".#............................#.",
	"..#..........................#..",
	"...........#........#...........",
	"...........##......##...........",
	"................................",
	".##....#................#....##.",
	".......#......#..#......#.......",
	".........##....##....##.........",
	"#......##..............##......#",
	".##.........##....##.........##.",
	"........##..#......#..##........",
	".#.....####.#......#.####.....#.",
	".#....####..#..##..#..####....#.",
	".#..######..#..##..#..######..#.",
	"#############..##..#############",
	"################################",
}

var seed1001Apples = []game.Point{
	{X: 4, Y: 1}, {X: 27, Y: 1}, {X: 1, Y: 3}, {X: 30, Y: 3},
	{X: 2, Y: 7}, {X: 29, Y: 7}, {X: 3, Y: 7}, {X: 28, Y: 7},
	{X: 8, Y: 1}, {X: 23, Y: 1}, {X: 5, Y: 5}, {X: 26, Y: 5},
	{X: 11, Y: 6}, {X: 20, Y: 6}, {X: 2, Y: 8}, {X: 29, Y: 8},
	{X: 3, Y: 12}, {X: 28, Y: 12},
}

func stateFromLayout(layout []string, apples []game.Point) game.State {
	walls := make(map[game.Point]bool)
	for y, row := range layout {
		for x, ch := range row {
			if ch == '#' {
				walls[game.Point{X: x, Y: y}] = true
			}
		}
	}
	grid := game.NewAG(len(layout[0]), len(layout), walls)
	state := game.NewState(grid)
	for _, p := range apples {
		state.Apples.Set(p)
	}
	return state
}

func refineState() (game.State, []MyBotInfo, []EnemyInfo) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)

	mine := []MyBotInfo{
		{ID: 0, Body: []game.Point{
			{X: 14, Y: 14}, {X: 14, Y: 13}, {X: 13, Y: 13},
			{X: 12, Y: 13}, {X: 12, Y: 14},
		}},
		{ID: 1, Body: []game.Point{
			{X: 20, Y: 15}, {X: 20, Y: 14}, {X: 20, Y: 13},
		}},
	}

	enemies := []EnemyInfo{
		{Head: game.Point{X: 5, Y: 14}, Facing: game.DirRight, BodyLen: 4,
			Body: []game.Point{{X: 5, Y: 14}, {X: 4, Y: 14}, {X: 3, Y: 14}, {X: 3, Y: 15}}},
		{Head: game.Point{X: 25, Y: 14}, Facing: game.DirLeft, BodyLen: 3,
			Body: []game.Point{{X: 25, Y: 14}, {X: 26, Y: 14}, {X: 27, Y: 14}}},
	}

	return state, mine, enemies
}

// ---------------------------------------------------------------------------
// Correctness: SimOneTurn
// ---------------------------------------------------------------------------

func TestSimOneTurn_NoCollision(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	mine := []MyBotInfo{{ID: 0, Body: []game.Point{{X: 2, Y: 4}, {X: 2, Y: 3}}}}
	dirs := []game.Direction{game.DirRight}
	o := SimOneTurn(&s, &sc, mine, nil, dirs, nil, nil)

	assert.Zero(t, o.Deaths[0], "should not die on simple move")
	assert.Zero(t, o.Losses[0])
	assert.Zero(t, o.Trapped[0])
}

func TestSimOneTurn_WallKill(t *testing.T) {
	g := mkTestGrid([]string{
		"#....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	// 3 parts → collision into wall → death (body ≤ 3)
	mine := []MyBotInfo{{ID: 0, Body: []game.Point{{X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}}}}
	dirs := []game.Direction{game.DirLeft}
	o := SimOneTurn(&s, &sc, mine, nil, dirs, nil, nil)

	assert.Equal(t, 1, o.Deaths[0], "should die hitting wall with body=3")
}

func TestSimOneTurn_EatApple(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	mine := []MyBotInfo{{ID: 0, Body: []game.Point{{X: 2, Y: 4}, {X: 2, Y: 3}}}}
	sources := []game.Point{{X: 3, Y: 4}}
	dirs := []game.Direction{game.DirRight}
	o := SimOneTurn(&s, &sc, mine, nil, dirs, nil, sources)

	assert.Zero(t, o.Deaths[0])
	assert.Zero(t, o.Losses[0])
}

func TestSimOneTurn_HeadOnCollision(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	mine := []MyBotInfo{{ID: 0, Body: []game.Point{{X: 1, Y: 4}, {X: 0, Y: 4}}}}
	enemies := []EnemyInfo{{
		Head: game.Point{X: 3, Y: 4}, Facing: game.DirLeft, BodyLen: 2,
		Body: []game.Point{{X: 3, Y: 4}, {X: 4, Y: 4}},
	}}
	ourDirs := []game.Direction{game.DirRight}
	eDirs := []game.Direction{game.DirLeft}
	o := SimOneTurn(&s, &sc, mine, enemies, ourDirs, eDirs, nil)

	// Both move into (2,4), both get beheaded
	assert.Equal(t, 1, o.Deaths[0], "we should die")
	assert.Equal(t, 1, o.Deaths[1], "enemy should die")
}

func TestSimOneTurn_Gravity(t *testing.T) {
	// Body standing on an apple; apple not eaten → stays
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	// apple at (2,3) supports body at (2,2). Move right → (3,2)
	// (3,2) has nothing below → falls
	mine := []MyBotInfo{{ID: 0, Body: []game.Point{{X: 2, Y: 2}, {X: 2, Y: 1}}}}
	sources := []game.Point{{X: 2, Y: 3}}
	dirs := []game.Direction{game.DirRight}
	o := SimOneTurn(&s, &sc, mine, nil, dirs, nil, sources)

	// Should NOT die: falls but lands eventually (or bottom)
	assert.Zero(t, o.Deaths[0], "should survive fall to bottom")
}

// ---------------------------------------------------------------------------
// Correctness: RefinePlans
// ---------------------------------------------------------------------------

func TestRefinePlans_Basic(t *testing.T) {
	state, mine, enemies := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)

	plans := []BotPlan{
		{ID: mine[0].ID, Body: mine[0].Body, Facing: game.DirRight, Dir: game.DirRight},
		{ID: mine[1].ID, Body: mine[1].Body, Facing: game.DirUp, Dir: game.DirUp},
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	RefinePlans(&state, &sc, mine, enemies, seed1001Apples, plans, deadline)

	for _, p := range plans {
		require.NotEqual(t, game.DirNone, p.Dir, "plan should have a direction")
	}
}

func TestRefinePlans_NoEnemies(t *testing.T) {
	state, mine, _ := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)

	plans := []BotPlan{
		{ID: mine[0].ID, Body: mine[0].Body, Facing: game.DirRight, Dir: game.DirRight},
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	// Should not panic, returns immediately
	RefinePlans(&state, &sc, mine, nil, seed1001Apples, plans, deadline)
	assert.Equal(t, game.DirRight, plans[0].Dir)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkSimOneTurn_2v2(b *testing.B) {
	state, mine, enemies := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)
	ourDirs := []game.Direction{game.DirRight, game.DirUp}
	eDirs := []game.Direction{game.DirRight, game.DirLeft}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SimOneTurn(&state, &sc, mine, enemies, ourDirs, eDirs, seed1001Apples)
	}
}

func BenchmarkSimOneTurn_4v4(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	g := state.Grid
	sc := NewRefScratch(g.Width, g.Height)

	mine := []MyBotInfo{
		{ID: 0, Body: []game.Point{{X: 14, Y: 14}, {X: 14, Y: 13}, {X: 13, Y: 13}, {X: 12, Y: 13}, {X: 12, Y: 14}}},
		{ID: 1, Body: []game.Point{{X: 20, Y: 15}, {X: 20, Y: 14}, {X: 20, Y: 13}}},
		{ID: 2, Body: []game.Point{{X: 10, Y: 15}, {X: 10, Y: 14}}},
		{ID: 3, Body: []game.Point{{X: 16, Y: 15}, {X: 16, Y: 14}, {X: 16, Y: 13}, {X: 16, Y: 12}}},
	}
	enemies := []EnemyInfo{
		{Head: game.Point{X: 5, Y: 14}, Facing: game.DirRight, BodyLen: 4, Body: []game.Point{{X: 5, Y: 14}, {X: 4, Y: 14}, {X: 3, Y: 14}, {X: 3, Y: 15}}},
		{Head: game.Point{X: 25, Y: 14}, Facing: game.DirLeft, BodyLen: 3, Body: []game.Point{{X: 25, Y: 14}, {X: 26, Y: 14}, {X: 27, Y: 14}}},
		{Head: game.Point{X: 8, Y: 15}, Facing: game.DirRight, BodyLen: 2, Body: []game.Point{{X: 8, Y: 15}, {X: 7, Y: 15}}},
		{Head: game.Point{X: 22, Y: 15}, Facing: game.DirLeft, BodyLen: 2, Body: []game.Point{{X: 22, Y: 15}, {X: 23, Y: 15}}},
	}
	ourDirs := []game.Direction{game.DirRight, game.DirUp, game.DirRight, game.DirUp}
	eDirs := []game.Direction{game.DirRight, game.DirLeft, game.DirRight, game.DirLeft}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SimOneTurn(&state, &sc, mine, enemies, ourDirs, eDirs, seed1001Apples)
	}
}

func BenchmarkWorstCasePlanRisk_2enemies(b *testing.B) {
	state, mine, enemies := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)
	ourDirs := []game.Direction{game.DirRight, game.DirUp}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		WorstCasePlanRisk(&state, &sc, mine, enemies, seed1001Apples, ourDirs)
	}
}

func BenchmarkRefinePlans_2v2(b *testing.B) {
	state, mine, enemies := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		plans := []BotPlan{
			{ID: mine[0].ID, Body: mine[0].Body, Facing: game.DirRight, Dir: game.DirRight},
			{ID: mine[1].ID, Body: mine[1].Body, Facing: game.DirUp, Dir: game.DirUp},
		}
		deadline := time.Now().Add(100 * time.Millisecond)
		RefinePlans(&state, &sc, mine, enemies, seed1001Apples, plans, deadline)
	}
}
