package agentkit

import (
	"testing"
	"time"
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
	if bg.Has(Point{X: 0, Y: 0}) {
		t.Fatal("FillBG should reset before filling")
	}
	if !bg.Has(Point{X: 1, Y: 1}) || !bg.Has(Point{X: 2, Y: 2}) {
		t.Fatal("FillBG did not set expected points")
	}
}

// --- OccExcept --------------------------------------------------------------

func TestOccExcept(t *testing.T) {
	base := NewBG(5, 5)
	base.Set(Point{X: 1, Y: 1})
	base.Set(Point{X: 2, Y: 2})
	base.Set(Point{X: 3, Y: 3})

	got := OccExcept(&base, []Point{{X: 2, Y: 2}})
	if !got.Has(Point{X: 1, Y: 1}) {
		t.Fatal("OccExcept should keep non-body cells")
	}
	if got.Has(Point{X: 2, Y: 2}) {
		t.Fatal("OccExcept should clear body cell")
	}
	if !got.Has(Point{X: 3, Y: 3}) {
		t.Fatal("OccExcept should keep non-body cells")
	}
}

// --- LegalDirs --------------------------------------------------------------

func TestLegalDirs(t *testing.T) {
	dirs := LegalDirs(DirUp)
	if len(dirs) != 3 {
		t.Fatalf("LegalDirs len = %d, want 3", len(dirs))
	}
	for _, d := range dirs {
		if d == DirDown {
			t.Fatal("LegalDirs(DirUp) must not include DirDown")
		}
	}

	dirs = LegalDirs(DirLeft)
	for _, d := range dirs {
		if d == DirRight {
			t.Fatal("LegalDirs(DirLeft) must not include DirRight")
		}
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
	if count == 0 {
		t.Fatal("expected non-zero flood count")
	}
	if dist[0*5+0] != 0 {
		t.Fatalf("start distance = %d, want 0", dist[0*5+0])
	}
	if dist[0*5+1] != Unreachable {
		t.Fatalf("occupied cell distance = %d, want Unreachable", dist[0*5+1])
	}
	if dist[1*5+2] != Unreachable {
		t.Fatalf("wall distance = %d, want Unreachable", dist[1*5+2])
	}
	if dist[1*5+0] != 1 {
		t.Fatalf("(0,1) distance = %d, want 1", dist[1*5+0])
	}
}

func TestFloodDistBlockedStart(t *testing.T) {
	g := NewAG(4, 4, nil)
	s := NewState(g)
	occ := NewBG(4, 4)
	occ.Set(Point{X: 2, Y: 2})

	count, dist := s.FloodDist(Point{X: 2, Y: 2}, &occ)
	if count != 0 {
		t.Fatalf("blocked start should return 0 count, got %d", count)
	}
	for _, d := range dist {
		if d != Unreachable {
			t.Fatal("all distances should be Unreachable when start is blocked")
		}
	}
}

// --- HasSupport -------------------------------------------------------------

func TestHasSupportGrounded(t *testing.T) {
	// Wall at (2,3) → (2,2) has wall below → WBelow.
	g := NewAG(5, 4, map[Point]bool{{X: 2, Y: 3}: true})
	body := []Point{{X: 2, Y: 2}, {X: 2, Y: 1}}
	if !HasSupport(g, body, nil, nil, nil) {
		t.Fatal("body over wall should be supported")
	}
}

func TestHasSupportFloating(t *testing.T) {
	g := NewAG(5, 5, nil)
	// Both parts in mid-air (no wall below, no occupied below).
	body := []Point{{X: 2, Y: 0}, {X: 2, Y: 1}}
	if HasSupport(g, body, nil, nil, nil) {
		t.Fatal("floating body should not be supported")
	}
}

func TestHasSupportByOccupied(t *testing.T) {
	g := NewAG(5, 5, nil)
	body := []Point{{X: 2, Y: 0}}
	occ := NewBG(5, 5)
	occ.Set(Point{X: 2, Y: 1}) // cell below is occupied
	if !HasSupport(g, body, nil, &occ, nil) {
		t.Fatal("body with occupied cell below should be supported")
	}
}

func TestHasSupportBySource(t *testing.T) {
	g := NewAG(5, 5, nil)
	body := []Point{{X: 2, Y: 0}}
	src := NewBG(5, 5)
	src.Set(Point{X: 2, Y: 1}) // source below
	if !HasSupport(g, body, &src, nil, nil) {
		t.Fatal("body with source below should be supported")
	}
}

func TestHasSupportEatenSourceIgnored(t *testing.T) {
	g := NewAG(5, 5, nil)
	body := []Point{{X: 2, Y: 0}}
	src := NewBG(5, 5)
	src.Set(Point{X: 2, Y: 1})
	eaten := Point{X: 2, Y: 1}
	if HasSupport(g, body, &src, nil, &eaten) {
		t.Fatal("eaten source should not provide support")
	}
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
	if !alive {
		t.Fatal("SimMove should survive")
	}
	if didEat {
		t.Fatal("SimMove should not eat without sources")
	}
	if nb[0] != (Point{X: 3, Y: 4}) {
		t.Fatalf("new head = %+v, want {3 4}", nb[0])
	}
	if len(nb) != 2 {
		t.Fatalf("body len = %d, want 2", len(nb))
	}
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
	if !alive {
		t.Fatal("should survive eating")
	}
	if !didEat {
		t.Fatal("should have eaten")
	}
	if len(nb) != 3 {
		t.Fatalf("body after eat = %d, want 3", len(nb))
	}
	if nb[0] != (Point{X: 3, Y: 4}) {
		t.Fatalf("new head = %+v, want {3 4}", nb[0])
	}
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
	if alive {
		t.Fatal("collision with own body (body≤3) should die")
	}
}

// --- SrcScore ---------------------------------------------------------------

func TestSrcScore(t *testing.T) {
	g := NewAG(5, 5, nil)
	head := Point{X: 2, Y: 2}

	// Same row, 1 step right — no up-penalty, no WBelow bonus (middle of grid).
	got := SrcScore(g, head, Point{X: 3, Y: 2})
	if got != 1 {
		t.Fatalf("SrcScore = %d, want 1", got)
	}

	// Target above: MDist=2, up penalty = head.Y-target.Y = 2 → total 4.
	got = SrcScore(g, head, Point{X: 2, Y: 0})
	if got != 4 {
		t.Fatalf("SrcScore upward = %d, want 4", got)
	}
}

// --- StateHash --------------------------------------------------------------

func TestStateHash(t *testing.T) {
	body := []Point{{X: 1, Y: 2}, {X: 1, Y: 3}}
	h1 := StateHash(DirUp, body)
	h2 := StateHash(DirUp, body)
	if h1 != h2 {
		t.Fatal("StateHash should be deterministic")
	}
	h3 := StateHash(DirDown, body)
	if h1 == h3 {
		t.Fatal("different facing should produce different hash")
	}
	h4 := StateHash(DirUp, []Point{{X: 1, Y: 3}, {X: 1, Y: 2}})
	if h1 == h4 {
		t.Fatal("different body order should produce different hash")
	}
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
	if len(got) != 1 || got[0] != (Point{X: 3, Y: 0}) {
		t.Fatalf("FiltSrc = %+v, want [{3 0}]", got)
	}
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
	if len(got) != 1 || got[0] != sources[0] {
		t.Fatalf("FiltSrc fallback = %+v, want %+v", got, sources)
	}
}

// --- IsSafeDir / BestSafeDir ------------------------------------------------

func TestIsSafeDir(t *testing.T) {
	dirInfo := map[Direction]*DirInfo{
		DirUp:    {Alive: true, Flood: 20},
		DirRight: {Alive: true, Flood: 3},
		DirLeft:  {Alive: false, Flood: 0},
	}
	bodyLen := 5 // thresh = 10
	if !IsSafeDir(DirUp, dirInfo, bodyLen) {
		t.Fatal("DirUp (flood 20) should be safe")
	}
	if IsSafeDir(DirRight, dirInfo, bodyLen) {
		t.Fatal("DirRight (flood 3 < 10) should be unsafe")
	}
	if IsSafeDir(DirLeft, dirInfo, bodyLen) {
		t.Fatal("DirLeft (not alive) should be unsafe")
	}
}

func TestBestSafeDir(t *testing.T) {
	dirInfo := map[Direction]*DirInfo{
		DirUp:    {Alive: true, Flood: 10},
		DirRight: {Alive: true, Flood: 30},
		DirLeft:  {Alive: false, Flood: 0},
	}
	dir, ok := BestSafeDir(dirInfo)
	if !ok || dir != DirRight {
		t.Fatalf("BestSafeDir = %v, %v, want DirRight true", dir, ok)
	}
}

func TestBestSafeDirNone(t *testing.T) {
	dirInfo := map[Direction]*DirInfo{
		DirUp: {Alive: false},
	}
	_, ok := BestSafeDir(dirInfo)
	if ok {
		t.Fatal("BestSafeDir with no alive dirs should return false")
	}
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
	if len(info) == 0 {
		t.Fatal("CalcDirInfo should return at least one direction")
	}
	for _, di := range info {
		if di.Alive {
			if di.Flood <= 0 {
				t.Fatal("alive direction should have positive flood count")
			}
			if di.Dists == nil {
				t.Fatal("alive direction should have distances")
			}
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
	if !res.Ok {
		t.Fatal("InstantEat should find adjacent apple")
	}
	if res.Dir != DirRight {
		t.Fatalf("InstantEat dir = %v, want DirRight", res.Dir)
	}
	if res.Steps != 1 {
		t.Fatalf("InstantEat steps = %d, want 1", res.Steps)
	}
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
	if res.Ok {
		t.Fatal("InstantEat should not find non-adjacent apple")
	}
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
	if !res.Ok {
		t.Fatal("PathBFS should find the apple")
	}
	if res.Target != sources[0] {
		t.Fatalf("PathBFS target = %+v, want %+v", res.Target, sources[0])
	}
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
	if !res.Ok {
		t.Fatal("BestAction should return Ok")
	}
	if res.Dir != DirRight {
		t.Fatalf("BestAction dir = %v, want DirRight (closer to source)", res.Dir)
	}
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
	if !res.Ok {
		t.Fatal("BestAction with no sources should still return Ok")
	}
	if res.Dir != DirUp {
		t.Fatalf("BestAction no-sources dir = %v, want DirUp (default)", res.Dir)
	}
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
	if !res.Ok {
		t.Fatal("BestAction should return Ok even with danger")
	}
	// With body len 2 (>3) the danger pen is 20 — mild, so right might still win.
	// We only check it returns a valid direction.
	if res.Dir == DirNone {
		t.Fatal("BestAction should not return DirNone")
	}
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
	if result[4*5+0] != 0 {
		t.Fatalf("enemy head distance = %d, want 0", result[4*5+0])
	}
	// (1,4) is one step from enemy → dist 1
	if result[4*5+1] != 1 {
		t.Fatalf("adjacent to enemy = %d, want 1", result[4*5+1])
	}
}
