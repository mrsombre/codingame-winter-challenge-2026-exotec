package main

import "testing"

// ---------------------------------------------------------------------------
// fBody — fixed-size body buffer used by the simulation scratch space
// ---------------------------------------------------------------------------

// TestFBodySetAndSlice verifies that set() stores body parts and slice()
// returns exactly those parts.  fBody is used inside refScratch to avoid
// allocations during the one-turn simulation.
func TestFBodySetAndSlice(t *testing.T) {
	var fb fBody
	pts := []Point{{3, 2}, {3, 3}, {3, 4}}
	fb.set(pts)

	got := fb.slice()
	if len(got) != 3 {
		t.Fatalf("slice len = %d, want 3", len(got))
	}
	for i, p := range pts {
		if got[i] != p {
			t.Errorf("slice[%d] = %v, want %v", i, got[i], p)
		}
	}
}

// TestFBodyFacing verifies that facing() derives the direction from the
// first two body parts (head and neck).  A single-segment body defaults
// to DirUp because there is no neck to infer direction from.
func TestFBodyFacing(t *testing.T) {
	var fb fBody

	// Two-segment body: head at (3,2), neck at (3,3) → facing up.
	fb.set([]Point{{3, 2}, {3, 3}})
	if f := fb.facing(); f != DirUp {
		t.Errorf("2-seg up: got %d, want DirUp", f)
	}

	// Head at (4,3), neck at (3,3) → facing right.
	fb.set([]Point{{4, 3}, {3, 3}})
	if f := fb.facing(); f != DirRight {
		t.Errorf("2-seg right: got %d, want DirRight", f)
	}

	// Single segment → DirUp (default).
	fb.set([]Point{{3, 3}})
	if f := fb.facing(); f != DirUp {
		t.Errorf("1-seg: got %d, want DirUp", f)
	}
}

// TestFBodyContains verifies the linear scan used to check whether a body
// occupies a given cell.  This is used during collision detection in the
// one-turn simulation.
func TestFBodyContains(t *testing.T) {
	var fb fBody
	fb.set([]Point{{1, 1}, {2, 1}, {3, 1}})

	if !fb.contains(Point{2, 1}) {
		t.Error("should contain (2,1)")
	}
	if fb.contains(Point{4, 1}) {
		t.Error("should not contain (4,1)")
	}
}

// ---------------------------------------------------------------------------
// VMoves — valid moves from a position given facing direction
// ---------------------------------------------------------------------------

// TestVMoves verifies that VMoves returns grid-legal directions that are
// not backward.  VMoves combines grid connectivity (no walls) with the
// facing constraint (no reversal).
func TestVMoves(t *testing.T) {
	setupGrid(flatFloor)

	// Open centre facing right → UP, RIGHT, DOWN (3 dirs, no LEFT).
	moves := state.VMoves(Point{3, 2}, DirRight)
	if len(moves) != 3 {
		t.Fatalf("centre facing right: got %d moves, want 3", len(moves))
	}
	for _, d := range moves {
		if d == DirLeft {
			t.Error("should not include backward direction DirLeft")
		}
	}

	// Corner (1,1) facing down: walls block UP and LEFT, backward is UP.
	// Only valid dirs from the grid are RIGHT and DOWN, minus backward (UP)
	// → RIGHT and DOWN.
	moves = state.VMoves(Point{1, 1}, DirDown)
	if len(moves) != 2 {
		t.Fatalf("corner facing down: got %d moves, want 2", len(moves))
	}

	// DirNone (length-1 snake): all grid-legal directions, no backward filter.
	moves = state.VMoves(Point{3, 2}, DirNone)
	if len(moves) != 4 {
		t.Fatalf("centre facing none: got %d moves, want 4", len(moves))
	}
}
