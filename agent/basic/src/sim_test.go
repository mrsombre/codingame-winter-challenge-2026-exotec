package main

import "testing"

// ---------------------------------------------------------------------------
// simMove — single-step movement with collision, eating, and gravity
// ---------------------------------------------------------------------------

// TestSimMoveNormal moves a 3-segment snake one step right on the floor.
// No apple, no collision → body slides forward, tail drops off, length stays 3.
//
// Before:  H=head  B=body
//   .H.    (head at 3,3, body at 2,3 and 1,3)
//   ###    (floor)
//
// After moving RIGHT:
//   ..H    (head at 4,3, body at 3,3 and 2,3)
func TestSimMoveNormal(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 3}, {2, 3}, {1, 3}}
	facing := DirRight
	occ := setupOcc(nil)

	nb, nf, alive, ate, _ := simMove(body, facing, DirRight, nil, &occ)
	if !alive {
		t.Fatal("snake should survive a normal move")
	}
	if ate {
		t.Error("should not eat without apples")
	}
	if len(nb) != 3 {
		t.Errorf("body length = %d, want 3", len(nb))
	}
	if nb[0] != (Point{4, 3}) {
		t.Errorf("head = %v, want {4,3}", nb[0])
	}
	if nf != DirRight {
		t.Errorf("facing = %d, want DirRight", nf)
	}
}

// TestSimMoveEatApple places an apple directly in front of the snake.
// Eating an apple grows the body by 1 (tail is NOT dropped).
//
// Before (facing right, apple at 4,3):
//   .HA    H=head A=apple
//   ###
//
// After: head moves to apple, body grows to length 4.
func TestSimMoveEatApple(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 3}, {2, 3}, {1, 3}}
	facing := DirRight
	occ := setupOcc(nil)
	srcBG := setupSrcBG([]Point{{4, 3}})

	nb, _, alive, ate, eatenAt := simMove(body, facing, DirRight, &srcBG, &occ)
	if !alive {
		t.Fatal("snake should survive eating an apple")
	}
	if !ate {
		t.Error("should have eaten the apple")
	}
	if eatenAt != (Point{4, 3}) {
		t.Errorf("eatenAt = %v, want {4,3}", eatenAt)
	}
	// Body grows: old body kept + new head prepended.
	if len(nb) != 4 {
		t.Errorf("body length after eat = %d, want 4", len(nb))
	}
	if nb[0] != (Point{4, 3}) {
		t.Errorf("head = %v, want {4,3}", nb[0])
	}
}

// TestSimMoveHitWallDie moves a length-3 snake into a wall.
// A length ≤3 snake dies on wall collision (cannot behead, too short).
func TestSimMoveHitWallDie(t *testing.T) {
	setupGrid(flatFloor)
	// Snake on the bottom row facing down — moving down hits the wall at y=4.
	body := []Point{{3, 3}, {2, 3}, {1, 3}}
	facing := DirRight
	occ := setupOcc(nil)

	_, _, alive, _, _ := simMove(body, facing, DirDown, nil, &occ)
	if alive {
		t.Error("length-3 snake should die when hitting a wall")
	}
}

// TestSimMoveHitWallBehead moves a length-4 snake into a wall.
// A snake longer than 3 segments loses its head (beheading) but survives.
// The new body starts at what was body[1] and has length-1 segments.
func TestSimMoveHitWallBehead(t *testing.T) {
	setupGrid(flatFloor)
	// Length-4 snake on the floor row, facing right, will hit right wall.
	body := []Point{{5, 3}, {4, 3}, {3, 3}, {2, 3}}
	facing := DirRight
	occ := setupOcc(nil)

	nb, _, alive, _, _ := simMove(body, facing, DirRight, nil, &occ)
	if !alive {
		t.Fatal("length-4 snake should survive beheading")
	}
	// After beheading: head removed, tail also dropped → length shrinks.
	// Wall collision: head is removed, remaining body shifts forward.
	// The result body should be what was body[1..] without the tail.
	if len(nb) != 3 {
		t.Errorf("body length after behead = %d, want 3", len(nb))
	}
}

// TestSimMoveHitOwnBody moves the snake into its own body segment.
// This triggers the same collision logic as hitting a wall.
func TestSimMoveHitOwnBody(t *testing.T) {
	setupGrid(flatFloor)
	// A U-shaped snake whose head would collide with its own body:
	// Head at (2,2) facing up, body curves right. Moving left would
	// land on body segment (1,2).
	body := []Point{{2, 2}, {2, 1}, {1, 1}, {1, 2}, {1, 3}}
	facing := DirDown
	occ := setupOcc(nil)

	_, _, alive, _, _ := simMove(body, facing, DirLeft, nil, &occ)
	// Length 5 → should survive via beheading.
	if !alive {
		t.Fatal("length-5 snake should survive self-collision (behead)")
	}
}

// TestSimMoveGravityFall verifies that a snake with no support falls due
// to gravity until it lands.  On the flatFloor grid, a snake in the air
// at y=1 should fall down to y=3 (the row above the bottom wall).
//
// This tests the gravity loop inside simMove that repeatedly increments Y
// until hasSupport returns true.
func TestSimMoveGravityFall(t *testing.T) {
	setupGrid(flatFloor)
	// Snake at top of open area, facing right. Moving right keeps it in the air.
	body := []Point{{2, 1}, {1, 1}}
	facing := DirRight
	occ := setupOcc(nil)

	nb, _, alive, _, _ := simMove(body, facing, DirRight, nil, &occ)
	if !alive {
		t.Fatal("snake should survive gravity fall to floor")
	}
	// After gravity, the snake should be on the floor row (y=3, above wall at y=4).
	if nb[0].Y != 3 {
		t.Errorf("head Y after fall = %d, want 3 (floor)", nb[0].Y)
	}
	if nb[1].Y != 3 {
		t.Errorf("neck Y after fall = %d, want 3 (floor)", nb[1].Y)
	}
}

// TestSimMoveStaysOnPlatform verifies that a snake walking on a platform
// (wall directly below) does NOT fall.  This is the positive case for
// hasSupport.
func TestSimMoveStaysOnPlatform(t *testing.T) {
	setupGrid(tallGrid)
	// Snake on top of the platform: platform walls at y=4 (x=2,3,4).
	// Walking on y=3 (row above platform) should stay at y=3.
	body := []Point{{3, 3}, {2, 3}}
	facing := DirRight
	occ := setupOcc(nil)

	nb, _, alive, _, _ := simMove(body, facing, DirRight, nil, &occ)
	if !alive {
		t.Fatal("snake should survive walking on platform")
	}
	if nb[0].Y != 3 {
		t.Errorf("head Y = %d, want 3 (stayed on platform)", nb[0].Y)
	}
}

// TestSimMoveEatAndStayGrounded verifies that a snake can eat an apple
// while remaining supported.  The apple provides support (sources.Has(below)),
// so a snake standing on an apple doesn't fall when eating it, because
// other body parts may still have support.
func TestSimMoveEatAndStayGrounded(t *testing.T) {
	setupGrid(flatFloor)
	// Snake on floor row facing right, apple directly ahead.
	body := []Point{{2, 3}, {1, 3}}
	facing := DirRight
	occ := setupOcc(nil)
	srcBG := setupSrcBG([]Point{{3, 3}})

	nb, _, alive, ate, _ := simMove(body, facing, DirRight, &srcBG, &occ)
	if !alive {
		t.Fatal("should survive eating on floor")
	}
	if !ate {
		t.Error("should eat the apple")
	}
	// Body grows and stays on floor.
	if len(nb) != 3 {
		t.Errorf("body length = %d, want 3", len(nb))
	}
	if nb[0].Y != 3 {
		t.Errorf("head Y = %d, want 3 (floor)", nb[0].Y)
	}
}

// ---------------------------------------------------------------------------
// hasSupport — checks whether any body part rests on a wall, occupied cell,
//              or apple cell
// ---------------------------------------------------------------------------

// TestHasSupportOnFloor verifies that a snake on the bottom row (above
// the wall) is supported.
func TestHasSupportOnFloor(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 3}, {2, 3}}
	if !hasSupport(body, nil, nil, nil) {
		t.Error("snake on floor should have support")
	}
}

// TestHasSupportInAir verifies that a snake floating with no wall, body,
// or apple below is NOT supported.
func TestHasSupportInAir(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 1}, {2, 1}}
	if hasSupport(body, nil, nil, nil) {
		t.Error("snake in air should NOT have support")
	}
}

// TestHasSupportOnApple verifies that an apple below a body part provides
// support.  Apples act as temporary platforms.
func TestHasSupportOnApple(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 1}, {2, 1}}
	srcBG := setupSrcBG([]Point{{3, 2}}) // apple below head
	if !hasSupport(body, &srcBG, nil, nil) {
		t.Error("apple below should provide support")
	}
}

// TestHasSupportOnOccupied verifies that another snake's body below
// provides support.  Snakes can rest on each other.
func TestHasSupportOnOccupied(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 1}, {2, 1}}
	occ := setupOcc([]Point{{3, 2}}) // another snake's body below head
	if !hasSupport(body, nil, &occ, nil) {
		t.Error("occupied cell below should provide support")
	}
}

// ---------------------------------------------------------------------------
// simulateOneTurn — full turn simulation with beheading and gravity
// ---------------------------------------------------------------------------

// TestSimulateOneTurnHeadOnCollision verifies that when two snakes move
// into the same cell, the collision logic triggers beheading for both.
// This is the key mechanic for head-on confrontations.
func TestSimulateOneTurnHeadOnCollision(t *testing.T) {
	setupGrid(flatFloor)
	// Two snakes on the floor facing each other, about to collide at (3,3).
	mine := []botEntry{
		{id: 0, body: []Point{{2, 3}, {1, 3}}},
	}
	enemies := []enemyInfo{
		{head: Point{4, 3}, facing: DirLeft, bodyLen: 2, body: []Point{{4, 3}, {5, 3}}},
	}
	sources := []Point{}

	// Both move toward each other → meet at (3,3).
	outcome := simulateOneTurn(&rsc, mine, enemies, []Direction{DirRight}, []Direction{DirLeft}, sources)

	// Both are length-2, which is ≤3, so both should die.
	if outcome.deaths[0] == 0 {
		t.Error("our snake should die in head-on collision (length 2)")
	}
	if outcome.deaths[1] == 0 {
		t.Error("enemy should die in head-on collision (length 2)")
	}
}

// TestSimulateOneTurnSafeMove verifies that a simple safe move results
// in no losses or deaths.
func TestSimulateOneTurnSafeMove(t *testing.T) {
	setupGrid(flatFloor)
	mine := []botEntry{
		{id: 0, body: []Point{{2, 3}, {1, 3}}},
	}
	sources := []Point{}

	outcome := simulateOneTurn(&rsc, mine, nil, []Direction{DirRight}, nil, sources)
	if outcome.deaths[0] != 0 || outcome.losses[0] != 0 {
		t.Errorf("safe move should have no deaths/losses, got deaths=%d losses=%d",
			outcome.deaths[0], outcome.losses[0])
	}
}

// ---------------------------------------------------------------------------
// outcomeRisk — scoring for worst-case analysis
// ---------------------------------------------------------------------------

// TestOutcomeRisk verifies that the risk formula heavily penalises our
// deaths and rewards enemy deaths.
func TestOutcomeRisk(t *testing.T) {
	// Our death is very bad.
	risk := outcomeRisk(oneTurnOutcome{deaths: [2]int{1, 0}})
	if risk <= 0 {
		t.Errorf("our death should give positive risk, got %d", risk)
	}

	// Enemy death reduces risk (negative contribution).
	riskEnemy := outcomeRisk(oneTurnOutcome{deaths: [2]int{0, 1}})
	if riskEnemy >= 0 {
		t.Errorf("enemy death alone should give negative risk, got %d", riskEnemy)
	}

	// Our death + enemy death: our death dominates.
	riskBoth := outcomeRisk(oneTurnOutcome{deaths: [2]int{1, 1}})
	if riskBoth <= 0 {
		t.Errorf("our death should dominate even with enemy death, got %d", riskBoth)
	}
}

// ---------------------------------------------------------------------------
// Head out of bounds — the engine allows the head to be outside the arena
// horizontally (X < 0 or X >= W) as long as the snake still has support.
// ---------------------------------------------------------------------------

// oobLeftGrid has NO wall on the left border, a wall at (1,1) blocking
// rightward movement from the head position, and a floor at the bottom.
//
//	........   y=0
//	.#......   y=1
//	........   y=2
//	........   y=3
//	########   y=4
var oobLeftGrid = []string{
	"........",
	".#......",
	"........",
	"........",
	"########",
}

// TestSimMoveHeadOOBLeft verifies that a snake can move its head to X=-1
// (out of bounds) without dying, as long as the remaining body has support.
// The engine treats OOB as "not a wall" — only in-bounds wall tiles cause
// beheading.
func TestSimMoveHeadOOBLeft(t *testing.T) {
	setupGrid(oobLeftGrid)
	// Snake standing vertically at X=0, head at (0,1), body (0,2), tail (0,3).
	// Facing UP. Wall at (1,1) blocks rightward movement.
	body := []Point{{0, 1}, {0, 2}, {0, 3}}
	facing := DirUp
	occ := setupOcc(nil)

	// Move LEFT → head goes to (-1,1), which is OOB but not a wall.
	nb, _, alive, _, _ := simMove(body, facing, DirLeft, nil, &occ)
	if !alive {
		t.Fatal("snake should survive moving head OOB left")
	}
	// After gravity the snake falls until tail rests on floor (y=4 wall).
	// Expected landing: head (-1,2), body (0,2), tail (0,3).
	if nb[0].X != -1 {
		t.Errorf("head X = %d, want -1 (OOB)", nb[0].X)
	}
	// Tail should be resting just above floor.
	lastPart := nb[len(nb)-1]
	if lastPart.Y != 3 {
		t.Errorf("tail Y = %d, want 3 (above floor)", lastPart.Y)
	}
}

// TestSimMoveHeadOOBRight verifies the same behavior on the right side.
//
//	........   y=0
//	......#.   y=1  wall at (6,1)
//	........   y=2
//	........   y=3
//	########   y=4
var oobRightGrid = []string{
	"........",
	"......#.",
	"........",
	"........",
	"########",
}

func TestSimMoveHeadOOBRight(t *testing.T) {
	setupGrid(oobRightGrid)
	// Snake at X=7 (rightmost open column), head at (7,1), facing UP.
	body := []Point{{7, 1}, {7, 2}, {7, 3}}
	facing := DirUp
	occ := setupOcc(nil)

	// Move RIGHT → head goes to (8,1), which is OOB (X >= W).
	nb, _, alive, _, _ := simMove(body, facing, DirRight, nil, &occ)
	if !alive {
		t.Fatal("snake should survive moving head OOB right")
	}
	if nb[0].X != 8 {
		t.Errorf("head X = %d, want 8 (OOB right)", nb[0].X)
	}
}

// TestSimMoveOOBEscapeSequence verifies the full escape path described in
// the issue: a snake trapped at X=0 with a wall to its right can escape
// by moving the head OOB left, then maneuvering back in-bounds.
//
// Note: simMove returns a slice of a shared buffer, so we must copy the
// body between calls.
func TestSimMoveOOBEscapeSequence(t *testing.T) {
	setupGrid(oobLeftGrid)
	body := []Point{{0, 1}, {0, 2}, {0, 3}}
	facing := DirUp
	occ := setupOcc(nil)

	copyBody := func(b []Point) []Point {
		c := make([]Point, len(b))
		copy(c, b)
		return c
	}

	// Step 1: Move LEFT → head OOB
	nb, nf, alive, _, _ := simMove(body, facing, DirLeft, nil, &occ)
	if !alive {
		t.Fatal("step 1 (LEFT): should survive")
	}
	nb = copyBody(nb)

	// Step 2: Move UP from OOB position
	nb2, nf2, alive2, _, _ := simMove(nb, nf, DirUp, nil, &occ)
	if !alive2 {
		t.Fatal("step 2 (UP): should survive")
	}
	nb2 = copyBody(nb2)

	// Step 3: Move RIGHT → head back in-bounds
	nb3, nf3, alive3, _, _ := simMove(nb2, nf2, DirRight, nil, &occ)
	if !alive3 {
		t.Fatal("step 3 (RIGHT): should survive")
	}
	nb3 = copyBody(nb3)

	// Step 4: Move RIGHT again → fully free
	nb4, _, alive4, _, _ := simMove(nb3, nf3, DirRight, nil, &occ)
	if !alive4 {
		t.Fatal("step 4 (RIGHT): should survive")
	}

	// Final head should be in-bounds with X >= 1.
	if nb4[0].X < 1 {
		t.Errorf("final head X = %d, want >= 1 (back in bounds, free)", nb4[0].X)
	}
}

// TestVMovesOOBHead verifies that VMoves returns valid directions when
// the head is at an out-of-bounds position.
func TestVMovesOOBHead(t *testing.T) {
	setupGrid(oobLeftGrid)
	pos := Point{-1, 2} // OOB left
	facing := DirLeft

	dirs := state.VMoves(pos, facing)
	if len(dirs) == 0 {
		t.Fatal("VMoves should return directions for OOB head position")
	}
	// Should have 3 directions (all except backward = DirRight)
	if len(dirs) != 3 {
		t.Errorf("VMoves returned %d directions, want 3 (all except backward)", len(dirs))
	}
	// Should NOT contain the backward direction (DirRight is opposite of DirLeft)
	for _, d := range dirs {
		if d == DirRight {
			t.Error("VMoves should not contain backward direction (DirRight)")
		}
	}
}

// TestSimulateOneTurnOOBNotBeheaded verifies that the full turn simulation
// does NOT behead a snake whose head is out of bounds (matching engine).
func TestSimulateOneTurnOOBNotBeheaded(t *testing.T) {
	setupGrid(oobLeftGrid)
	mine := []botEntry{
		{id: 0, body: []Point{{0, 1}, {0, 2}, {0, 3}}},
	}
	sources := []Point{}

	outcome := simulateOneTurn(&rsc, mine, nil, []Direction{DirLeft}, nil, sources)
	if outcome.deaths[0] != 0 {
		t.Error("snake moving head OOB should NOT die (engine allows it)")
	}
	if outcome.losses[0] != 0 {
		t.Errorf("no losses expected, got %d", outcome.losses[0])
	}
}
