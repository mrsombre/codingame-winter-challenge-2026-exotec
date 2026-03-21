package main

import "testing"

// ---------------------------------------------------------------------------
// Direction helpers
// ---------------------------------------------------------------------------

// TestOpp verifies that Opp returns the opposite direction for all four
// cardinals and DirNone for DirNone.  This is the most basic invariant
// used throughout movement and collision logic.
func TestOpp(t *testing.T) {
	cases := [][2]Direction{
		{DirUp, DirDown},
		{DirDown, DirUp},
		{DirLeft, DirRight},
		{DirRight, DirLeft},
		{DirNone, DirNone},
	}
	for _, c := range cases {
		if got := Opp(c[0]); got != c[1] {
			t.Errorf("Opp(%d) = %d, want %d", c[0], got, c[1])
		}
	}
}

// TestFacingPts infers the direction a snake faces from its head and neck.
// The facing direction is critical for determining which moves are valid
// (a snake cannot reverse into itself).
func TestFacingPts(t *testing.T) {
	// Head one step right of neck → facing right.
	if got := FacingPts(Point{3, 2}, Point{2, 2}); got != DirRight {
		t.Errorf("right: got %d, want %d", got, DirRight)
	}
	// Head one step above neck → facing up.
	if got := FacingPts(Point{3, 1}, Point{3, 2}); got != DirUp {
		t.Errorf("up: got %d, want %d", got, DirUp)
	}
	// Head one step below neck → facing down.
	if got := FacingPts(Point{3, 3}, Point{3, 2}); got != DirDown {
		t.Errorf("down: got %d, want %d", got, DirDown)
	}
	// Head one step left of neck → facing left.
	if got := FacingPts(Point{2, 2}, Point{3, 2}); got != DirLeft {
		t.Errorf("left: got %d, want %d", got, DirLeft)
	}
	// Non-adjacent points → DirNone (shouldn't happen in practice, but guard).
	if got := FacingPts(Point{0, 0}, Point{5, 5}); got != DirNone {
		t.Errorf("non-adjacent: got %d, want DirNone", got)
	}
}

// TestValidDirs checks that validDirs filters out the backward direction.
// A snake facing right should not be offered DirLeft, etc.
// When facing is DirNone (length-1 snake) all four directions are valid.
func TestValidDirs(t *testing.T) {
	// Facing right → 3 dirs, none of them left.
	dirs, n := validDirs(DirRight)
	if n != 3 {
		t.Fatalf("facing right: got %d dirs, want 3", n)
	}
	for _, d := range dirs[:n] {
		if d == DirLeft {
			t.Error("facing right should not include DirLeft")
		}
	}

	// DirNone → all 4 directions.
	dirs, n = validDirs(DirNone)
	if n != 4 {
		t.Fatalf("facing none: got %d dirs, want 4", n)
	}
	seen := map[Direction]bool{}
	for _, d := range dirs[:n] {
		seen[d] = true
	}
	for d := DirUp; d <= DirLeft; d++ {
		if !seen[d] {
			t.Errorf("DirNone: missing direction %d", d)
		}
	}
}

// ---------------------------------------------------------------------------
// Point arithmetic
// ---------------------------------------------------------------------------

// TestAdd verifies vector addition used for moving a point in a direction.
func TestAdd(t *testing.T) {
	p := Add(Point{3, 4}, DirDelta[DirUp])
	if p != (Point{3, 3}) {
		t.Errorf("Add UP: got %v, want {3,3}", p)
	}
	p = Add(Point{3, 4}, DirDelta[DirRight])
	if p != (Point{4, 4}) {
		t.Errorf("Add RIGHT: got %v, want {4,4}", p)
	}
}

// TestMDist verifies Manhattan distance, the core heuristic for
// apple targeting and enemy proximity.
func TestMDist(t *testing.T) {
	if d := MDist(Point{1, 1}, Point{4, 5}); d != 7 {
		t.Errorf("MDist({1,1},{4,5}) = %d, want 7", d)
	}
	// Same point → distance 0.
	if d := MDist(Point{3, 3}, Point{3, 3}); d != 0 {
		t.Errorf("MDist same point = %d, want 0", d)
	}
}

// ---------------------------------------------------------------------------
// BitGrid
// ---------------------------------------------------------------------------

// TestBitGrid exercises the Set / Has / Clear / Reset cycle on a BitGrid.
// BitGrid is the workhorse data structure for tracking occupancy, apples,
// and danger zones.  Off-grid coordinates must be silently ignored.
func TestBitGrid(t *testing.T) {
	bg := NewBG(10, 10)

	// A fresh grid has nothing set.
	if bg.Has(Point{3, 3}) {
		t.Error("fresh grid should not have (3,3)")
	}

	// Set and verify.
	bg.Set(Point{3, 3})
	if !bg.Has(Point{3, 3}) {
		t.Error("after Set, (3,3) should be present")
	}

	// Clear and verify.
	bg.Clear(Point{3, 3})
	if bg.Has(Point{3, 3}) {
		t.Error("after Clear, (3,3) should be absent")
	}

	// Set multiple, then Reset clears all.
	bg.Set(Point{0, 0})
	bg.Set(Point{9, 9})
	bg.Reset()
	if bg.Has(Point{0, 0}) || bg.Has(Point{9, 9}) {
		t.Error("Reset should clear all bits")
	}

	// Out-of-bounds coordinates must not panic and report false.
	bg.Set(Point{-1, 0})
	if bg.Has(Point{-1, 0}) {
		t.Error("out-of-bounds Set should be a no-op")
	}
	bg.Set(Point{10, 10})
	if bg.Has(Point{10, 10}) {
		t.Error("out-of-bounds Set should be a no-op")
	}
}

// ---------------------------------------------------------------------------
// AGrid — wall detection and cell connectivity
// ---------------------------------------------------------------------------

// TestAGridWalls verifies that NewAG correctly marks walls and open cells.
// Uses the flatFloor grid where only the border is walled.
func TestAGridWalls(t *testing.T) {
	setupGrid(flatFloor)

	// Border cells are walls.
	if !grid.IsWall(Point{0, 0}) {
		t.Error("(0,0) should be a wall")
	}
	if !grid.IsWall(Point{6, 4}) {
		t.Error("(6,4) should be a wall")
	}
	// Interior cell is open.
	if grid.IsWall(Point{3, 2}) {
		t.Error("(3,2) should be open")
	}
	// Out-of-bounds treated as wall.
	if !grid.IsWall(Point{-1, 0}) {
		t.Error("out-of-bounds should be a wall")
	}
}

// TestAGridWBelow verifies the "wall below" precomputation.
// A cell has WBelow=true when either y == height-1 (floor) or the cell
// directly below is a wall.  This is the foundation for gravity support.
func TestAGridWBelow(t *testing.T) {
	setupGrid(gridWithPillar)

	// Cell (3,1) is directly above the pillar at (3,2) → wall below.
	if !grid.WBelow(Point{3, 1}) {
		t.Error("(3,1) should have wall below (pillar at 3,2)")
	}
	// Cell (1,3) is one row above the floor → wall below.
	if !grid.WBelow(Point{1, 3}) {
		t.Error("(1,3) should have wall below (floor)")
	}
	// Cell (2,1) has open space below (2,2) → no wall below.
	if grid.WBelow(Point{2, 1}) {
		t.Error("(2,1) should NOT have wall below")
	}
}

// TestAGridCDirs verifies that CDirs returns only non-wall directions.
// A corner cell should have fewer exits than a centre cell.
func TestAGridCDirs(t *testing.T) {
	setupGrid(flatFloor)

	// Centre of open area (3,2): all 4 neighbours are open.
	dirs := grid.CDirs(Point{3, 2})
	if len(dirs) != 4 {
		t.Errorf("centre: got %d dirs, want 4", len(dirs))
	}

	// Corner of open area (1,1): walls to the left and above.
	dirs = grid.CDirs(Point{1, 1})
	if len(dirs) != 2 {
		t.Errorf("top-left corner: got %d dirs, want 2 (right, down)", len(dirs))
	}

	// Wall cell returns nil.
	dirs = grid.CDirs(Point{0, 0})
	if dirs != nil {
		t.Error("wall cell should return nil dirs")
	}
}

// TestAdjCells verifies that adjCells returns non-wall neighbours of a cell.
// Used by terrain BFS to find adjacent landing spots.
func TestAdjCells(t *testing.T) {
	setupGrid(gridWithPillar)

	// Cell (3,1) has the pillar (wall) below at (3,2) and border wall above at (3,0).
	// Only RIGHT (4,1) and LEFT (2,1) are open → 2 neighbours.
	adj, n := adjCells(grid, Point{3, 1})
	_ = adj
	if n != 2 {
		t.Errorf("adjCells(3,1) near pillar: got %d, want 2", n)
	}

	// Centre of open area (3,2) in flatFloor → 4 neighbours.
	setupGrid(flatFloor)
	adj, n = adjCells(grid, Point{3, 2})
	_ = adj
	if n != 4 {
		t.Errorf("adjCells(3,2) open area: got %d, want 4", n)
	}
}
