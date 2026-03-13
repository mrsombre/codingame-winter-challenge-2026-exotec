package agentkit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func refineState() (State, []MyBotInfo, []EnemyInfo) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)

	mine := []MyBotInfo{
		{ID: 0, Body: []Point{
			{X: 14, Y: 14}, {X: 14, Y: 13}, {X: 13, Y: 13},
			{X: 12, Y: 13}, {X: 12, Y: 14},
		}},
		{ID: 1, Body: []Point{
			{X: 20, Y: 15}, {X: 20, Y: 14}, {X: 20, Y: 13},
		}},
	}

	enemies := []EnemyInfo{
		{Head: Point{X: 5, Y: 14}, Facing: DirRight, BodyLen: 4,
			Body: []Point{{X: 5, Y: 14}, {X: 4, Y: 14}, {X: 3, Y: 14}, {X: 3, Y: 15}}},
		{Head: Point{X: 25, Y: 14}, Facing: DirLeft, BodyLen: 3,
			Body: []Point{{X: 25, Y: 14}, {X: 26, Y: 14}, {X: 27, Y: 14}}},
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
	s := NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	mine := []MyBotInfo{{ID: 0, Body: []Point{{X: 2, Y: 4}, {X: 2, Y: 3}}}}
	dirs := []Direction{DirRight}
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
	s := NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	// 3 parts → collision into wall → death (body ≤ 3)
	mine := []MyBotInfo{{ID: 0, Body: []Point{{X: 1, Y: 0}, {X: 2, Y: 0}, {X: 3, Y: 0}}}}
	dirs := []Direction{DirLeft}
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
	s := NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	mine := []MyBotInfo{{ID: 0, Body: []Point{{X: 2, Y: 4}, {X: 2, Y: 3}}}}
	sources := []Point{{X: 3, Y: 4}}
	dirs := []Direction{DirRight}
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
	s := NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	mine := []MyBotInfo{{ID: 0, Body: []Point{{X: 1, Y: 4}, {X: 0, Y: 4}}}}
	enemies := []EnemyInfo{{
		Head: Point{X: 3, Y: 4}, Facing: DirLeft, BodyLen: 2,
		Body: []Point{{X: 3, Y: 4}, {X: 4, Y: 4}},
	}}
	ourDirs := []Direction{DirRight}
	eDirs := []Direction{DirLeft}
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
	s := NewState(g)
	sc := NewRefScratch(g.Width, g.Height)

	// apple at (2,3) supports body at (2,2). Move right → (3,2)
	// (3,2) has nothing below → falls
	mine := []MyBotInfo{{ID: 0, Body: []Point{{X: 2, Y: 2}, {X: 2, Y: 1}}}}
	sources := []Point{{X: 2, Y: 3}}
	dirs := []Direction{DirRight}
	o := SimOneTurn(&s, &sc, mine, nil, dirs, nil, sources)

	// Should NOT die: falls but lands eventually (or bottom)
	// In 5-high grid, (3,2) body → falls, hits bottom at y=4
	assert.Zero(t, o.Deaths[0], "should survive fall to bottom")
}

// ---------------------------------------------------------------------------
// Correctness: RefinePlans
// ---------------------------------------------------------------------------

func TestRefinePlans_Basic(t *testing.T) {
	state, mine, enemies := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)

	plans := []RefPlan{
		{ID: mine[0].ID, Body: mine[0].Body, Facing: DirRight, Dir: DirRight},
		{ID: mine[1].ID, Body: mine[1].Body, Facing: DirUp, Dir: DirUp},
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	RefinePlans(&state, &sc, mine, enemies, seed1001Apples, plans, deadline)

	for _, p := range plans {
		require.NotEqual(t, DirNone, p.Dir, "plan should have a direction")
	}
}

func TestRefinePlans_NoEnemies(t *testing.T) {
	state, mine, _ := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)

	plans := []RefPlan{
		{ID: mine[0].ID, Body: mine[0].Body, Facing: DirRight, Dir: DirRight},
	}
	deadline := time.Now().Add(100 * time.Millisecond)
	// Should not panic, returns immediately
	RefinePlans(&state, &sc, mine, nil, seed1001Apples, plans, deadline)
	assert.Equal(t, DirRight, plans[0].Dir)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkSimOneTurn_2v2(b *testing.B) {
	state, mine, enemies := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)
	ourDirs := []Direction{DirRight, DirUp}
	eDirs := []Direction{DirRight, DirLeft}
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
		{ID: 0, Body: []Point{{X: 14, Y: 14}, {X: 14, Y: 13}, {X: 13, Y: 13}, {X: 12, Y: 13}, {X: 12, Y: 14}}},
		{ID: 1, Body: []Point{{X: 20, Y: 15}, {X: 20, Y: 14}, {X: 20, Y: 13}}},
		{ID: 2, Body: []Point{{X: 10, Y: 15}, {X: 10, Y: 14}}},
		{ID: 3, Body: []Point{{X: 16, Y: 15}, {X: 16, Y: 14}, {X: 16, Y: 13}, {X: 16, Y: 12}}},
	}
	enemies := []EnemyInfo{
		{Head: Point{X: 5, Y: 14}, Facing: DirRight, BodyLen: 4, Body: []Point{{X: 5, Y: 14}, {X: 4, Y: 14}, {X: 3, Y: 14}, {X: 3, Y: 15}}},
		{Head: Point{X: 25, Y: 14}, Facing: DirLeft, BodyLen: 3, Body: []Point{{X: 25, Y: 14}, {X: 26, Y: 14}, {X: 27, Y: 14}}},
		{Head: Point{X: 8, Y: 15}, Facing: DirRight, BodyLen: 2, Body: []Point{{X: 8, Y: 15}, {X: 7, Y: 15}}},
		{Head: Point{X: 22, Y: 15}, Facing: DirLeft, BodyLen: 2, Body: []Point{{X: 22, Y: 15}, {X: 23, Y: 15}}},
	}
	ourDirs := []Direction{DirRight, DirUp, DirRight, DirUp}
	eDirs := []Direction{DirRight, DirLeft, DirRight, DirLeft}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SimOneTurn(&state, &sc, mine, enemies, ourDirs, eDirs, seed1001Apples)
	}
}

func BenchmarkWorstCasePlanRisk_2enemies(b *testing.B) {
	state, mine, enemies := refineState()
	sc := NewRefScratch(state.Grid.Width, state.Grid.Height)
	ourDirs := []Direction{DirRight, DirUp}
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
		plans := []RefPlan{
			{ID: mine[0].ID, Body: mine[0].Body, Facing: DirRight, Dir: DirRight},
			{ID: mine[1].ID, Body: mine[1].Body, Facing: DirUp, Dir: DirUp},
		}
		deadline := time.Now().Add(100 * time.Millisecond)
		RefinePlans(&state, &sc, mine, enemies, seed1001Apples, plans, deadline)
	}
}
