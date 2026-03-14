package bot

import (
	"testing"
	"time"

	"codingame/internal/agentkit/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mkTestGrid builds a grid from ASCII rows ('.' = open, '#' = wall).
func mkTestGrid(rows []string) *game.AGrid {
	h := len(rows)
	w := 0
	for _, r := range rows {
		if len(r) > w {
			w = len(r)
		}
	}
	walls := make(map[game.Point]bool)
	for y, row := range rows {
		for x, ch := range row {
			if ch == '#' {
				walls[game.Point{X: x, Y: y}] = true
			}
		}
	}
	return game.NewAG(w, h, walls)
}

// --- StateHash --------------------------------------------------------------

func TestStateHash(t *testing.T) {
	body := []game.Point{{X: 1, Y: 2}, {X: 1, Y: 3}}
	h1 := StateHash(game.DirUp, body)
	h2 := StateHash(game.DirUp, body)
	assert.Equal(t, h1, h2, "StateHash should be deterministic")
	h3 := StateHash(game.DirDown, body)
	assert.NotEqual(t, h1, h3, "different facing should produce different hash")
	h4 := StateHash(game.DirUp, []game.Point{{X: 1, Y: 3}, {X: 1, Y: 2}})
	assert.NotEqual(t, h1, h4, "different body order should produce different hash")
}

// --- FiltSrc ----------------------------------------------------------------

func TestFiltSrc(t *testing.T) {
	g := game.NewAG(5, 5, nil)
	s := game.NewState(g)
	W := g.Width

	sources := []game.Point{{X: 1, Y: 0}, {X: 3, Y: 0}}
	myDists := make([]int, 5*5)
	enemyDists := make([]int, 5*5)
	for i := range myDists {
		myDists[i] = game.Unreachable
		enemyDists[i] = game.Unreachable
	}
	// (1,0): enemy 7 steps closer → filter out.
	myDists[0*W+1] = 10
	enemyDists[0*W+1] = 2
	// (3,0): close race → keep.
	myDists[0*W+3] = 5
	enemyDists[0*W+3] = 4

	got := FiltSrc(&s, sources, myDists, enemyDists)
	require.Len(t, got, 1)
	assert.Equal(t, game.Point{X: 3, Y: 0}, got[0])
}

func TestFiltSrcFallback(t *testing.T) {
	g := game.NewAG(5, 5, nil)
	s := game.NewState(g)
	W := g.Width

	sources := []game.Point{{X: 1, Y: 0}}
	myDists := make([]int, 5*5)
	enemyDists := make([]int, 5*5)
	for i := range myDists {
		myDists[i] = game.Unreachable
		enemyDists[i] = game.Unreachable
	}
	myDists[0*W+1] = 10
	enemyDists[0*W+1] = 2

	got := FiltSrc(&s, sources, myDists, enemyDists)
	require.Len(t, got, 1)
	assert.Equal(t, sources[0], got[0])
}

// --- IsSafeDir / BestSafeDir ------------------------------------------------

func TestIsSafeDir(t *testing.T) {
	dirInfo := map[game.Direction]*DirInfo{
		game.DirUp:    {Alive: true, Flood: 20},
		game.DirRight: {Alive: true, Flood: 3},
		game.DirLeft:  {Alive: false, Flood: 0},
	}
	bodyLen := 5 // thresh = 10
	assert.True(t, IsSafeDir(game.DirUp, dirInfo, bodyLen), "DirUp (flood 20) should be safe")
	assert.False(t, IsSafeDir(game.DirRight, dirInfo, bodyLen), "DirRight (flood 3 < 10) should be unsafe")
	assert.False(t, IsSafeDir(game.DirLeft, dirInfo, bodyLen), "DirLeft (not alive) should be unsafe")
}

func TestBestSafeDir(t *testing.T) {
	dirInfo := map[game.Direction]*DirInfo{
		game.DirUp:    {Alive: true, Flood: 10},
		game.DirRight: {Alive: true, Flood: 30},
		game.DirLeft:  {Alive: false, Flood: 0},
	}
	dir, ok := BestSafeDir(dirInfo)
	require.True(t, ok)
	assert.Equal(t, game.DirRight, dir)
}

func TestBestSafeDirNone(t *testing.T) {
	dirInfo := map[game.Direction]*DirInfo{
		game.DirUp: {Alive: false},
	}
	_, ok := BestSafeDir(dirInfo)
	assert.False(t, ok, "BestSafeDir with no alive dirs should return false")
}

// --- CalcDirInfo ------------------------------------------------------------

func TestCalcDirInfo(t *testing.T) {
	// Open 5x5 grid; body grounded at bottom row.
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	body := []game.Point{{X: 2, Y: 4}, {X: 2, Y: 3}}
	occ := game.NewBG(5, 5)
	for _, p := range body[1:] {
		occ.Set(p)
	}

	info := CalcDirInfo(&s, body, game.DirUp, &occ)
	require.NotEmpty(t, info, "CalcDirInfo should return at least one direction")
	for _, di := range info {
		if di.Alive {
			assert.Positive(t, di.Flood, "alive direction should have positive flood count")
			assert.NotNil(t, di.Dists, "alive direction should have distances")
		}
	}
}

// --- InstantEat -------------------------------------------------------------

func TestInstantEat(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	body := []game.Point{{X: 2, Y: 4}, {X: 2, Y: 3}}

	srcBG := game.NewBG(5, 5)
	srcBG.Set(game.Point{X: 3, Y: 4}) // apple adjacent right
	sources := []game.Point{{X: 3, Y: 4}}
	occ := game.NewBG(5, 5)

	res := InstantEat(&s, body, game.DirUp, sources, &srcBG, &occ)
	require.True(t, res.Ok, "InstantEat should find adjacent apple")
	assert.Equal(t, game.DirRight, res.Dir)
	assert.Equal(t, 1, res.Steps)
}

func TestInstantEatNone(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	body := []game.Point{{X: 2, Y: 4}, {X: 2, Y: 3}}

	srcBG := game.NewBG(5, 5)
	srcBG.Set(game.Point{X: 0, Y: 0}) // far away
	sources := []game.Point{{X: 0, Y: 0}}
	occ := game.NewBG(5, 5)

	res := InstantEat(&s, body, game.DirUp, sources, &srcBG, &occ)
	assert.False(t, res.Ok, "InstantEat should not find non-adjacent apple")
}

// --- PathBFS ----------------------------------------------------------------

func TestPathBFS(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	body := []game.Point{{X: 0, Y: 4}, {X: 0, Y: 3}}
	facing := game.DirUp
	sources := []game.Point{{X: 4, Y: 4}}

	srcBG := game.NewBG(5, 5)
	game.FillBG(&srcBG, sources)
	occ := game.NewBG(5, 5)
	occ.Set(game.Point{X: 0, Y: 3})
	dirInfo := CalcDirInfo(&s, body, facing, &occ)
	enemyDists := make([]int, 5*5)
	for i := range enemyDists {
		enemyDists[i] = game.Unreachable
	}

	deadline := time.Now().Add(100 * time.Millisecond)
	res := PathBFS(&s, body, facing, sources, 10, dirInfo, enemyDists, &srcBG, &occ, deadline)
	require.True(t, res.Ok, "PathBFS should find the apple")
	assert.Equal(t, sources[0], res.Target)
}

// --- BestAction -------------------------------------------------------------

func TestBestActionPicksClosestSource(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	body := []game.Point{{X: 0, Y: 4}, {X: 0, Y: 3}}
	facing := game.DirUp

	// Source to the right — should prefer moving right.
	sources := []game.Point{{X: 4, Y: 4}}
	srcBG := game.NewBG(5, 5)
	game.FillBG(&srcBG, sources)
	occ := game.NewBG(5, 5)
	occ.Set(game.Point{X: 0, Y: 3})
	danger := game.NewBG(5, 5)
	dirInfo := CalcDirInfo(&s, body, facing, &occ)
	enemyDists := make([]int, 5*5)
	for i := range enemyDists {
		enemyDists[i] = game.Unreachable
	}

	res := BestAction(&s, body, facing, sources, dirInfo, nil, enemyDists, &srcBG, &occ, &danger)
	require.True(t, res.Ok, "BestAction should return Ok")
	assert.Equal(t, game.DirRight, res.Dir)
}

func TestBestActionNoSources(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	body := []game.Point{{X: 2, Y: 4}, {X: 2, Y: 3}}
	occ := game.NewBG(5, 5)
	danger := game.NewBG(5, 5)
	srcBG := game.NewBG(5, 5)
	dirInfo := CalcDirInfo(&s, body, game.DirUp, &occ)
	enemyDists := make([]int, 5*5)
	for i := range enemyDists {
		enemyDists[i] = game.Unreachable
	}

	res := BestAction(&s, body, game.DirUp, nil, dirInfo, nil, enemyDists, &srcBG, &occ, &danger)
	require.True(t, res.Ok, "BestAction with no sources should still return Ok")
	assert.Equal(t, game.DirUp, res.Dir)
}

func TestBestActionDangerPenalty(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	body := []game.Point{{X: 0, Y: 4}, {X: 0, Y: 3}}
	facing := game.DirUp

	// Source is to the right but (1,4) — the right step — is marked danger.
	sources := []game.Point{{X: 4, Y: 4}}
	srcBG := game.NewBG(5, 5)
	game.FillBG(&srcBG, sources)
	occ := game.NewBG(5, 5)
	occ.Set(game.Point{X: 0, Y: 3})
	danger := game.NewBG(5, 5)
	danger.Set(game.Point{X: 1, Y: 4}) // penalise moving right

	dirInfo := CalcDirInfo(&s, body, facing, &occ)
	enemyDists := make([]int, 5*5)
	for i := range enemyDists {
		enemyDists[i] = game.Unreachable
	}

	res := BestAction(&s, body, facing, sources, dirInfo, nil, enemyDists, &srcBG, &occ, &danger)
	require.True(t, res.Ok, "BestAction should return Ok even with danger")
	assert.NotEqual(t, game.DirNone, res.Dir, "BestAction should not return DirNone")
}

// --- CalcEnemyDist ----------------------------------------------------------

func TestCalcEnemyDist(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := game.NewState(g)
	allOcc := game.NewBG(5, 5)

	enemies := []EnemyInfo{
		{Head: game.Point{X: 0, Y: 4}, Facing: game.DirUp, BodyLen: 2, Body: []game.Point{{X: 0, Y: 4}, {X: 0, Y: 3}}},
	}
	allOcc.Set(game.Point{X: 0, Y: 4})
	allOcc.Set(game.Point{X: 0, Y: 3})

	result := CalcEnemyDist(&s, enemies, &allOcc)
	// (0,4) is enemy head → dist 0
	assert.Zero(t, result[4*5+0])
	// (1,4) is one step from enemy → dist 1
	assert.Equal(t, 1, result[4*5+1])
}
