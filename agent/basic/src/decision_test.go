package main

import "testing"

// ---------------------------------------------------------------------------
// isSafeDir — threshold-based safety check
// ---------------------------------------------------------------------------

// TestIsSafeDir verifies that a direction is "safe" only when its flood
// distance exceeds 2× the body length (with a minimum threshold of 4).
// This prevents the snake from entering tight corridors where it could
// get trapped.
func TestIsSafeDir(t *testing.T) {
	// Construct synthetic DirInfo values to test the threshold logic
	// without needing a full grid BFS.
	bodyLen := 5
	thresh := bodyLen * 2 // = 10

	// Direction with exactly the threshold → safe.
	var info [5]*DirInfo
	info[DirRight] = &DirInfo{flood: thresh, alive: true}
	if !isSafeDir(DirRight, info, bodyLen) {
		t.Error("flood == threshold should be safe")
	}

	// Direction with flood below threshold → unsafe.
	info[DirLeft] = &DirInfo{flood: thresh - 1, alive: true}
	if isSafeDir(DirLeft, info, bodyLen) {
		t.Error("flood < threshold should be unsafe")
	}

	// Nil dirInfo → unsafe (direction not explored).
	if isSafeDir(DirUp, info, bodyLen) {
		t.Error("nil dirInfo should be unsafe")
	}

	// Dead direction → unsafe.
	info[DirDown] = &DirInfo{flood: 100, alive: false}
	if isSafeDir(DirDown, info, bodyLen) {
		t.Error("dead direction should be unsafe")
	}
}

// TestIsSafeDirMinThreshold verifies the minimum threshold of 4 for very
// short snakes.  A length-1 snake has 2×1=2, but the code clamps to 4.
func TestIsSafeDirMinThreshold(t *testing.T) {
	var info [5]*DirInfo
	info[DirUp] = &DirInfo{flood: 3, alive: true}

	// bodyLen=1 → threshold = max(2, 4) = 4; flood=3 < 4 → unsafe.
	if isSafeDir(DirUp, info, 1) {
		t.Error("flood=3 should be unsafe for bodyLen=1 (min threshold=4)")
	}
	info[DirUp].flood = 4
	if !isSafeDir(DirUp, info, 1) {
		t.Error("flood=4 should be safe for bodyLen=1")
	}
}

// ---------------------------------------------------------------------------
// bestSafeDir — picks the direction with the highest flood distance
// ---------------------------------------------------------------------------

// TestBestSafeDir verifies that bestSafeDir returns the direction with
// the most reachable cells, regardless of which cardinal direction it is.
func TestBestSafeDir(t *testing.T) {
	var info [5]*DirInfo
	info[DirUp] = &DirInfo{flood: 10, alive: true}
	info[DirRight] = &DirInfo{flood: 25, alive: true}
	info[DirDown] = &DirInfo{flood: 15, alive: true}
	// DirLeft not set.

	dir, ok := bestSafeDir(info)
	if !ok {
		t.Fatal("should find a safe dir")
	}
	if dir != DirRight {
		t.Errorf("best = %d, want DirRight (flood 25)", dir)
	}
}

// TestBestSafeDirNone verifies that bestSafeDir returns ok=false when
// no directions are alive (all nil or dead).
func TestBestSafeDirNone(t *testing.T) {
	var info [5]*DirInfo
	_, ok := bestSafeDir(info)
	if ok {
		t.Error("no alive dirs → should return ok=false")
	}
}

// ---------------------------------------------------------------------------
// instantEat — one-step apple grab
// ---------------------------------------------------------------------------

// TestInstantEatFindsAdjacentApple places an apple directly in front of
// a 3-segment snake and verifies that instantEat correctly identifies it.
// This is the fastest eating path and gets priority over BFS search.
//
// We use a 3-segment snake so that after eating (growing to 4) the
// hasFollowupEscape check succeeds — the grown snake can always
// survive a wall/self collision via beheading (length > 3).
func TestInstantEatFindsAdjacentApple(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 3}, {2, 3}, {1, 3}}
	facing := DirRight
	occ := setupOcc(nil)
	// Apple one step to the right.
	sources := []Point{{4, 3}}
	srcBG := setupSrcBG(sources)

	result := instantEat(body, facing, sources, &srcBG, &occ)
	if !result.ok {
		t.Fatal("should find the apple")
	}
	if result.dir != DirRight {
		t.Errorf("dir = %d, want DirRight", result.dir)
	}
	if result.target != (Point{4, 3}) {
		t.Errorf("target = %v, want {4,3}", result.target)
	}
	if result.steps != 1 {
		t.Errorf("steps = %d, want 1", result.steps)
	}
}

// TestInstantEatNoAppleAdjacent verifies that instantEat returns ok=false
// when no apple is within one step.
func TestInstantEatNoAppleAdjacent(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 3}, {2, 3}}
	facing := DirRight
	occ := setupOcc(nil)
	// Apple is 2 steps away — not instant.
	sources := []Point{{5, 3}}
	srcBG := setupSrcBG(sources)

	result := instantEat(body, facing, sources, &srcBG, &occ)
	if result.ok {
		t.Error("no adjacent apple → should return ok=false")
	}
}

// TestInstantEatSkipsBackward verifies that an apple behind the snake
// (in the backward direction) is not considered, since snakes cannot
// reverse.
func TestInstantEatSkipsBackward(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 3}, {2, 3}}
	facing := DirRight
	occ := setupOcc(nil)
	// Apple behind (to the left) — backward move is illegal.
	sources := []Point{{2, 3}}
	srcBG := setupSrcBG(sources)

	result := instantEat(body, facing, sources, &srcBG, &occ)
	// The apple at (2,3) IS on the snake's neck, so srcBG.Has won't match
	// a valid move direction. Even if it did, it's backward.
	if result.ok && result.dir == DirLeft {
		t.Error("should not eat backward")
	}
}

// ---------------------------------------------------------------------------
// filtSrc — remove apples the enemy can reach significantly before us
// ---------------------------------------------------------------------------

// TestFiltSrcRemovesEnemyControlled verifies that apples the enemy reaches
// 4+ steps before us are filtered out.  The threshold is enemy_dist < my_dist - 3.
// This prevents the snake from chasing apples it cannot win.
func TestFiltSrcRemovesEnemyControlled(t *testing.T) {
	setupGrid(flatFloor)
	n := W * H

	// My distances: apple A at dist 10, apple B at dist 2.
	myDists := make([]int, n)
	for i := range myDists {
		myDists[i] = Unreachable
	}
	// Enemy distances: apple A at dist 3 (enemy arrives 7 steps before us),
	// apple B at dist 5 (enemy arrives 3 steps later than us — we keep it).
	enemyDists := make([]int, n)
	for i := range enemyDists {
		enemyDists[i] = Unreachable
	}

	appleA := Point{1, 1}
	appleB := Point{3, 3}
	myDists[appleA.Y*W+appleA.X] = 10
	myDists[appleB.Y*W+appleB.X] = 2
	enemyDists[appleA.Y*W+appleA.X] = 3 // 3 < 10-3=7 → filtered
	enemyDists[appleB.Y*W+appleB.X] = 5 // 5 >= 2-3=-1 → kept

	sources := []Point{appleA, appleB}
	result := filtSrc(sources, myDists, enemyDists)

	// appleA should be filtered (enemy reaches 7 steps before us).
	// appleB should remain.
	if len(result) != 1 {
		t.Fatalf("filtSrc: got %d apples, want 1", len(result))
	}
	if result[0] != appleB {
		t.Errorf("remaining apple = %v, want %v", result[0], appleB)
	}
}

// TestFiltSrcKeepsAllWhenAllFiltered verifies the fallback: if ALL apples
// would be filtered, filtSrc returns the original list.  The snake must
// have something to target even in a losing position.
func TestFiltSrcKeepsAllWhenAllFiltered(t *testing.T) {
	setupGrid(flatFloor)
	n := W * H
	myDists := make([]int, n)
	enemyDists := make([]int, n)
	for i := range myDists {
		myDists[i] = Unreachable
		enemyDists[i] = Unreachable
	}

	apple := Point{3, 3}
	myDists[apple.Y*W+apple.X] = 20
	enemyDists[apple.Y*W+apple.X] = 1 // 1 < 20-3=17 → would filter

	sources := []Point{apple}
	result := filtSrc(sources, myDists, enemyDists)

	// All would be filtered → fallback returns original.
	if len(result) != 1 {
		t.Errorf("fallback: got %d apples, want 1", len(result))
	}
}

// ---------------------------------------------------------------------------
// dangerPenalty — scoring penalty for cells near enemies
// ---------------------------------------------------------------------------

// TestDangerPenaltyScaling verifies that the penalty scales with body
// length: short snakes (≤3) get the highest penalty, medium (≤5) get
// a mid penalty, and longer snakes get the base penalty.
// This makes short snakes more cautious near enemies.
func TestDangerPenaltyScaling(t *testing.T) {
	cell := Point{3, 3}
	var enemies []enemyInfo // no direct enemy adjacency

	// Short snake (len 2) gets shortPen.
	pen2 := dangerPenalty(cell, 2, enemies, 20, 500, 100, -500)
	// Mid snake (len 5) gets midPen.
	pen5 := dangerPenalty(cell, 5, enemies, 20, 500, 100, -500)
	// Long snake (len 8) gets basePen.
	pen8 := dangerPenalty(cell, 8, enemies, 20, 500, 100, -500)

	if pen2 != 500 {
		t.Errorf("shortPen for len=2: got %d, want 500", pen2)
	}
	if pen5 != 100 {
		t.Errorf("midPen for len=5: got %d, want 100", pen5)
	}
	if pen8 != 20 {
		t.Errorf("basePen for len=8: got %d, want 20", pen8)
	}
}

// TestDangerPenaltyHuntBonus verifies that when a big snake is adjacent
// to a small enemy (enemy ≤3, us >3), the penalty becomes negative
// (huntBonus), encouraging aggressive pursuit of weak enemies.
func TestDangerPenaltyHuntBonus(t *testing.T) {
	// Enemy at (2,3) facing right — adjacent cells include (3,3).
	enemies := []enemyInfo{
		{head: Point{2, 3}, facing: DirRight, bodyLen: 2},
	}
	// Our snake is length 6 (> 3), enemy is length 2 (≤ 3).
	// Cell (3,3) is one step right of the enemy head → hunt bonus.
	pen := dangerPenalty(Point{3, 3}, 6, enemies, 20, 500, 100, -500)
	if pen != -500 {
		t.Errorf("hunt bonus: got %d, want -500", pen)
	}
}

// ---------------------------------------------------------------------------
// calcDirInfo — per-direction reachability analysis
// ---------------------------------------------------------------------------

// TestCalcDirInfoAliveAndFlood verifies that calcDirInfo computes flood
// distances for each valid direction.  On an open grid, all non-backward
// directions should be alive with positive flood counts.
//
// We place the snake at (3,2) — the centre of the open area — so that
// UP (3,1) and DOWN (3,3) are both open cells.  On the bottom row (y=3)
// moving DOWN would hit a wall.
func TestCalcDirInfoAliveAndFlood(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 2}, {2, 2}}
	facing := DirRight
	occ := setupOcc(body[1:]) // mark body except head as occupied

	info := calcDirInfo(body, facing, &occ)

	// Facing right from (3,2) → UP, RIGHT, DOWN are valid (not LEFT/backward).
	// All three neighbours (3,1), (4,2), (3,3) are open cells.
	for _, dir := range []Direction{DirUp, DirRight, DirDown} {
		di := info[dir]
		if di == nil {
			t.Errorf("dir %d: info is nil", dir)
			continue
		}
		if !di.alive {
			t.Errorf("dir %d: should be alive", dir)
		}
		if di.flood <= 0 {
			t.Errorf("dir %d: flood = %d, want > 0", dir, di.flood)
		}
	}

	// Backward direction (LEFT) should be nil (not explored).
	if info[DirLeft] != nil {
		t.Error("backward direction should be nil")
	}
}

// ---------------------------------------------------------------------------
// limitedSupportTargets — cap and sort targets for support job planning
// ---------------------------------------------------------------------------

// TestLimitedSupportTargets verifies that the function caps targets to 4
// and sorts them by Y descending (prefer high targets) then X ascending.
func TestLimitedSupportTargets(t *testing.T) {
	targets := []Point{
		{1, 1}, {2, 3}, {3, 2}, {4, 1}, {5, 3},
	}
	result := limitedSupportTargets(targets)
	if len(result) != 4 {
		t.Fatalf("got %d targets, want 4", len(result))
	}
	// Sorted by Y desc, then X asc: (2,3), (5,3), (3,2), (1,1) or (4,1).
	if result[0].Y < result[1].Y {
		t.Error("targets should be sorted by Y descending")
	}
}

// TestLimitedSupportTargetsFewInputs verifies that when there are ≤4
// targets, the original slice is returned unchanged.
func TestLimitedSupportTargetsFewInputs(t *testing.T) {
	targets := []Point{{1, 1}, {2, 2}}
	result := limitedSupportTargets(targets)
	if len(result) != 2 {
		t.Errorf("got %d, want 2 (pass-through)", len(result))
	}
}

// ---------------------------------------------------------------------------
// hasFollowupEscape — validates that eating an apple doesn't trap us
// ---------------------------------------------------------------------------

// TestHasFollowupEscapeOpenGrid verifies that after eating an apple on an
// open grid, the snake always has at least one valid follow-up move.
func TestHasFollowupEscapeOpenGrid(t *testing.T) {
	setupGrid(flatFloor)
	// After eating, the body grows but on an open grid there's always an escape.
	body := []Point{{4, 3}, {3, 3}, {2, 3}}
	facing := DirRight
	occ := setupOcc(nil)
	srcBG := setupSrcBG([]Point{{4, 3}})

	if !hasFollowupEscape(body, facing, &srcBG, &occ, Point{4, 3}) {
		t.Error("should have escape on open grid")
	}
}
