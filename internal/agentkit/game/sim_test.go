package game

import (
	"testing"

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
