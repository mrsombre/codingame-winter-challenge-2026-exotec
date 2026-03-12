package agentkit

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mkTestGrid builds a grid from ASCII rows ('.' = open, '#' = wall).
func mkTestGrid(rows []string) *AGrid {
	h := len(rows)
	w := 0
	for _, r := range rows {
		if len(r) > w {
			w = len(r)
		}
	}
	walls := make(map[Point]bool)
	for y, row := range rows {
		for x, ch := range row {
			if ch == '#' {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}
	return NewAG(w, h, walls)
}

// --- FillBG -----------------------------------------------------------------

func TestFillBG(t *testing.T) {
	bg := NewBG(5, 5)
	bg.Set(Point{X: 0, Y: 0})
	FillBG(&bg, []Point{{X: 1, Y: 1}, {X: 2, Y: 2}})
	assert.False(t, bg.Has(Point{X: 0, Y: 0}), "FillBG should reset before filling")
	assert.True(t, bg.Has(Point{X: 1, Y: 1}) && bg.Has(Point{X: 2, Y: 2}), "FillBG did not set expected points")
}

// --- OccExcept --------------------------------------------------------------

func TestOccExcept(t *testing.T) {
	base := NewBG(5, 5)
	base.Set(Point{X: 1, Y: 1})
	base.Set(Point{X: 2, Y: 2})
	base.Set(Point{X: 3, Y: 3})

	got := OccExcept(&base, []Point{{X: 2, Y: 2}})
	assert.True(t, got.Has(Point{X: 1, Y: 1}), "OccExcept should keep non-body cells")
	assert.False(t, got.Has(Point{X: 2, Y: 2}), "OccExcept should clear body cell")
	assert.True(t, got.Has(Point{X: 3, Y: 3}), "OccExcept should keep non-body cells")
}

// --- LegalDirs --------------------------------------------------------------

func TestLegalDirs(t *testing.T) {
	dirs := LegalDirs(DirUp)
	assert.Len(t, dirs, 3)
	for _, d := range dirs {
		assert.NotEqual(t, DirDown, d, "LegalDirs(DirUp) must not include DirDown")
	}

	dirs = LegalDirs(DirLeft)
	for _, d := range dirs {
		assert.NotEqual(t, DirRight, d, "LegalDirs(DirLeft) must not include DirRight")
	}
}

// --- FloodDist --------------------------------------------------------------

func TestFloodDist(t *testing.T) {
	// Wall at (2,1); occupied at (1,0); start at (0,0).
	g := NewAG(5, 4, map[Point]bool{{X: 2, Y: 1}: true})
	s := NewState(g)
	occ := NewBG(5, 4)
	occ.Set(Point{X: 1, Y: 0})

	count, dist := s.FloodDist(Point{X: 0, Y: 0}, &occ)
	assert.NotZero(t, count, "expected non-zero flood count")
	assert.Zero(t, dist[0*5+0])
	assert.Equal(t, Unreachable, dist[0*5+1])
	assert.Equal(t, Unreachable, dist[1*5+2])
	assert.Equal(t, 1, dist[1*5+0])
}

func TestFloodDistBlockedStart(t *testing.T) {
	g := NewAG(4, 4, nil)
	s := NewState(g)
	occ := NewBG(4, 4)
	occ.Set(Point{X: 2, Y: 2})

	count, dist := s.FloodDist(Point{X: 2, Y: 2}, &occ)
	assert.Zero(t, count, "blocked start should return 0 count")
	for _, d := range dist {
		assert.Equal(t, Unreachable, d, "all distances should be Unreachable when start is blocked")
	}
}

// --- HasSupport -------------------------------------------------------------

func TestHasSupportGrounded(t *testing.T) {
	// Wall at (2,3) → (2,2) has wall below → WBelow.
	g := NewAG(5, 4, map[Point]bool{{X: 2, Y: 3}: true})
	body := []Point{{X: 2, Y: 2}, {X: 2, Y: 1}}
	assert.True(t, HasSupport(g, body, nil, nil, nil), "body over wall should be supported")
}

func TestHasSupportFloating(t *testing.T) {
	g := NewAG(5, 5, nil)
	// Both parts in mid-air (no wall below, no occupied below).
	body := []Point{{X: 2, Y: 0}, {X: 2, Y: 1}}
	assert.False(t, HasSupport(g, body, nil, nil, nil), "floating body should not be supported")
}

func TestHasSupportByOccupied(t *testing.T) {
	g := NewAG(5, 5, nil)
	body := []Point{{X: 2, Y: 0}}
	occ := NewBG(5, 5)
	occ.Set(Point{X: 2, Y: 1}) // cell below is occupied
	assert.True(t, HasSupport(g, body, nil, &occ, nil), "body with occupied cell below should be supported")
}

func TestHasSupportBySource(t *testing.T) {
	g := NewAG(5, 5, nil)
	body := []Point{{X: 2, Y: 0}}
	src := NewBG(5, 5)
	src.Set(Point{X: 2, Y: 1}) // source below
	assert.True(t, HasSupport(g, body, &src, nil, nil), "body with source below should be supported")
}

func TestHasSupportEatenSourceIgnored(t *testing.T) {
	g := NewAG(5, 5, nil)
	body := []Point{{X: 2, Y: 0}}
	src := NewBG(5, 5)
	src.Set(Point{X: 2, Y: 1})
	eaten := Point{X: 2, Y: 1}
	assert.False(t, HasSupport(g, body, &src, nil, &eaten), "eaten source should not provide support")
}

// --- SimMove ----------------------------------------------------------------

func TestSimMoveStraight(t *testing.T) {
	// 5x5 open grid; bottom row is grounded (y==height-1).
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := NewState(g)
	body := []Point{{X: 2, Y: 4}, {X: 2, Y: 3}}

	nb, _, alive, didEat, _ := s.SimMove(body, DirUp, DirRight, nil, nil)
	require.True(t, alive, "SimMove should survive")
	assert.False(t, didEat, "SimMove should not eat without sources")
	require.NotEmpty(t, nb)
	assert.Equal(t, Point{X: 3, Y: 4}, nb[0])
	assert.Len(t, nb, 2)
}

func TestSimMoveEat(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := NewState(g)
	body := []Point{{X: 2, Y: 4}, {X: 2, Y: 3}}
	srcBG := NewBG(5, 5)
	srcBG.Set(Point{X: 3, Y: 4}) // apple to the right

	nb, _, alive, didEat, _ := s.SimMove(body, DirUp, DirRight, &srcBG, nil)
	require.True(t, alive, "should survive eating")
	assert.True(t, didEat, "should have eaten")
	require.NotEmpty(t, nb)
	assert.Len(t, nb, 3)
	assert.Equal(t, Point{X: 3, Y: 4}, nb[0])
}

func TestSimMoveWallDeath(t *testing.T) {
	// Wall at (0,0); body up against it.
	g := mkTestGrid([]string{
		"#....",
		".....",
		".....",
		".....",
		".....",
	})
	s := NewState(g)
	// Body at (1,4),(1,3),(1,2) — 3 parts, try to move into own body → collision → n≤3 → dead.
	body := []Point{{X: 1, Y: 4}, {X: 1, Y: 3}, {X: 1, Y: 2}}
	_, _, alive, _, _ := s.SimMove(body, DirUp, DirUp, nil, nil)
	assert.False(t, alive, "collision with own body (body≤3) should die")
}

// --- SrcScore ---------------------------------------------------------------

func TestSrcScore(t *testing.T) {
	g := NewAG(5, 5, nil)
	head := Point{X: 2, Y: 2}

	// Same row, 1 step right — no up-penalty, no WBelow bonus (middle of grid).
	got := SrcScore(g, head, Point{X: 3, Y: 2})
	assert.Equal(t, 1, got)

	// Target above: MDist=2, up penalty = head.Y-target.Y = 2 → total 4.
	got = SrcScore(g, head, Point{X: 2, Y: 0})
	assert.Equal(t, 4, got)
}

// --- StateHash --------------------------------------------------------------

func TestStateHash(t *testing.T) {
	body := []Point{{X: 1, Y: 2}, {X: 1, Y: 3}}
	h1 := StateHash(DirUp, body)
	h2 := StateHash(DirUp, body)
	assert.Equal(t, h1, h2, "StateHash should be deterministic")
	h3 := StateHash(DirDown, body)
	assert.NotEqual(t, h1, h3, "different facing should produce different hash")
	h4 := StateHash(DirUp, []Point{{X: 1, Y: 3}, {X: 1, Y: 2}})
	assert.NotEqual(t, h1, h4, "different body order should produce different hash")
}

// --- FiltSrc ----------------------------------------------------------------

func TestFiltSrc(t *testing.T) {
	g := NewAG(5, 5, nil)
	s := NewState(g)
	W := g.Width

	sources := []Point{{X: 1, Y: 0}, {X: 3, Y: 0}}
	myDists := make([]int, 5*5)
	enemyDists := make([]int, 5*5)
	for i := range myDists {
		myDists[i] = Unreachable
		enemyDists[i] = Unreachable
	}
	// (1,0): enemy 7 steps closer → filter out.
	myDists[0*W+1] = 10
	enemyDists[0*W+1] = 2
	// (3,0): close race → keep.
	myDists[0*W+3] = 5
	enemyDists[0*W+3] = 4

	got := s.FiltSrc(sources, myDists, enemyDists)
	require.Len(t, got, 1)
	assert.Equal(t, Point{X: 3, Y: 0}, got[0])
}

func TestFiltSrcFallback(t *testing.T) {
	g := NewAG(5, 5, nil)
	s := NewState(g)
	W := g.Width

	sources := []Point{{X: 1, Y: 0}}
	myDists := make([]int, 5*5)
	enemyDists := make([]int, 5*5)
	for i := range myDists {
		myDists[i] = Unreachable
		enemyDists[i] = Unreachable
	}
	myDists[0*W+1] = 10
	enemyDists[0*W+1] = 2

	got := s.FiltSrc(sources, myDists, enemyDists)
	require.Len(t, got, 1)
	assert.Equal(t, sources[0], got[0])
}

// --- IsSafeDir / BestSafeDir ------------------------------------------------

func TestIsSafeDir(t *testing.T) {
	dirInfo := map[Direction]*DirInfo{
		DirUp:    {Alive: true, Flood: 20},
		DirRight: {Alive: true, Flood: 3},
		DirLeft:  {Alive: false, Flood: 0},
	}
	bodyLen := 5 // thresh = 10
	assert.True(t, IsSafeDir(DirUp, dirInfo, bodyLen), "DirUp (flood 20) should be safe")
	assert.False(t, IsSafeDir(DirRight, dirInfo, bodyLen), "DirRight (flood 3 < 10) should be unsafe")
	assert.False(t, IsSafeDir(DirLeft, dirInfo, bodyLen), "DirLeft (not alive) should be unsafe")
}

func TestBestSafeDir(t *testing.T) {
	dirInfo := map[Direction]*DirInfo{
		DirUp:    {Alive: true, Flood: 10},
		DirRight: {Alive: true, Flood: 30},
		DirLeft:  {Alive: false, Flood: 0},
	}
	dir, ok := BestSafeDir(dirInfo)
	require.True(t, ok)
	assert.Equal(t, DirRight, dir)
}

func TestBestSafeDirNone(t *testing.T) {
	dirInfo := map[Direction]*DirInfo{
		DirUp: {Alive: false},
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
	s := NewState(g)
	body := []Point{{X: 2, Y: 4}, {X: 2, Y: 3}}
	occ := NewBG(5, 5)
	for _, p := range body[1:] {
		occ.Set(p)
	}

	info := s.CalcDirInfo(body, DirUp, &occ)
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
	s := NewState(g)
	body := []Point{{X: 2, Y: 4}, {X: 2, Y: 3}}

	srcBG := NewBG(5, 5)
	srcBG.Set(Point{X: 3, Y: 4}) // apple adjacent right
	sources := []Point{{X: 3, Y: 4}}
	occ := NewBG(5, 5)

	res := s.InstantEat(body, DirUp, sources, &srcBG, &occ)
	require.True(t, res.Ok, "InstantEat should find adjacent apple")
	assert.Equal(t, DirRight, res.Dir)
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
	s := NewState(g)
	body := []Point{{X: 2, Y: 4}, {X: 2, Y: 3}}

	srcBG := NewBG(5, 5)
	srcBG.Set(Point{X: 0, Y: 0}) // far away
	sources := []Point{{X: 0, Y: 0}}
	occ := NewBG(5, 5)

	res := s.InstantEat(body, DirUp, sources, &srcBG, &occ)
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
	s := NewState(g)
	body := []Point{{X: 0, Y: 4}, {X: 0, Y: 3}}
	facing := DirUp
	sources := []Point{{X: 4, Y: 4}}

	srcBG := NewBG(5, 5)
	FillBG(&srcBG, sources)
	occ := NewBG(5, 5)
	occ.Set(Point{X: 0, Y: 3})
	dirInfo := s.CalcDirInfo(body, facing, &occ)
	enemyDists := make([]int, 5*5)
	for i := range enemyDists {
		enemyDists[i] = Unreachable
	}

	deadline := time.Now().Add(100 * time.Millisecond)
	res := s.PathBFS(body, facing, sources, 10, dirInfo, enemyDists, &srcBG, &occ, deadline)
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
	s := NewState(g)
	body := []Point{{X: 0, Y: 4}, {X: 0, Y: 3}}
	facing := DirUp

	// Source to the right — should prefer moving right.
	sources := []Point{{X: 4, Y: 4}}
	srcBG := NewBG(5, 5)
	FillBG(&srcBG, sources)
	occ := NewBG(5, 5)
	occ.Set(Point{X: 0, Y: 3})
	danger := NewBG(5, 5)
	dirInfo := s.CalcDirInfo(body, facing, &occ)
	enemyDists := make([]int, 5*5)
	for i := range enemyDists {
		enemyDists[i] = Unreachable
	}

	res := s.BestAction(body, facing, sources, dirInfo, nil, enemyDists, &srcBG, &occ, &danger)
	require.True(t, res.Ok, "BestAction should return Ok")
	assert.Equal(t, DirRight, res.Dir)
}

func TestBestActionNoSources(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := NewState(g)
	body := []Point{{X: 2, Y: 4}, {X: 2, Y: 3}}
	occ := NewBG(5, 5)
	danger := NewBG(5, 5)
	srcBG := NewBG(5, 5)
	dirInfo := s.CalcDirInfo(body, DirUp, &occ)
	enemyDists := make([]int, 5*5)
	for i := range enemyDists {
		enemyDists[i] = Unreachable
	}

	res := s.BestAction(body, DirUp, nil, dirInfo, nil, enemyDists, &srcBG, &occ, &danger)
	require.True(t, res.Ok, "BestAction with no sources should still return Ok")
	assert.Equal(t, DirUp, res.Dir)
}

func TestBestActionDangerPenalty(t *testing.T) {
	g := mkTestGrid([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})
	s := NewState(g)
	body := []Point{{X: 0, Y: 4}, {X: 0, Y: 3}}
	facing := DirUp

	// Source is to the right but (1,4) — the right step — is marked danger.
	sources := []Point{{X: 4, Y: 4}}
	srcBG := NewBG(5, 5)
	FillBG(&srcBG, sources)
	occ := NewBG(5, 5)
	occ.Set(Point{X: 0, Y: 3})
	danger := NewBG(5, 5)
	danger.Set(Point{X: 1, Y: 4}) // penalise moving right

	dirInfo := s.CalcDirInfo(body, facing, &occ)
	enemyDists := make([]int, 5*5)
	for i := range enemyDists {
		enemyDists[i] = Unreachable
	}

	res := s.BestAction(body, facing, sources, dirInfo, nil, enemyDists, &srcBG, &occ, &danger)
	require.True(t, res.Ok, "BestAction should return Ok even with danger")
	// With body len 2 (>3) the danger pen is 20 — mild, so right might still win.
	// We only check it returns a valid direction.
	assert.NotEqual(t, DirNone, res.Dir, "BestAction should not return DirNone")
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
	s := NewState(g)
	allOcc := NewBG(5, 5)

	enemies := []EnemyInfo{
		{Head: Point{X: 0, Y: 4}, Facing: DirUp, BodyLen: 2, Body: []Point{{X: 0, Y: 4}, {X: 0, Y: 3}}},
	}
	allOcc.Set(Point{X: 0, Y: 4})
	allOcc.Set(Point{X: 0, Y: 3})

	result := s.CalcEnemyDist(enemies, &allOcc)
	// (0,4) is enemy head → dist 0
	assert.Zero(t, result[4*5+0])
	// (1,4) is one step from enemy → dist 1
	assert.Equal(t, 1, result[4*5+1])
}
