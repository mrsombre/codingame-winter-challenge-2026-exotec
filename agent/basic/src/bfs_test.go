package main

import "testing"

// ---------------------------------------------------------------------------
// stateHash — deterministic FNV-1a hash for BFS deduplication
// ---------------------------------------------------------------------------

// TestStateHashDeterministic verifies that stateHash produces identical
// hashes for identical inputs.  The BFS uses this hash as a visited-set
// key, so non-deterministic hashing would cause duplicate exploration or
// missed states.
func TestStateHashDeterministic(t *testing.T) {
	body := []Point{{3, 3}, {2, 3}, {1, 3}}
	h1 := stateHash(DirRight, body)
	h2 := stateHash(DirRight, body)
	if h1 != h2 {
		t.Error("same input should produce same hash")
	}
}

// TestStateHashDiffersOnInput verifies that different inputs produce
// different hashes (collision resistance).  While hash collisions are
// theoretically possible, these specific inputs should not collide.
func TestStateHashDiffersOnInput(t *testing.T) {
	bodyA := []Point{{3, 3}, {2, 3}, {1, 3}}
	bodyB := []Point{{3, 3}, {2, 3}, {1, 2}} // last segment differs

	hA := stateHash(DirRight, bodyA)
	hB := stateHash(DirRight, bodyB)
	if hA == hB {
		t.Error("different bodies should (almost certainly) produce different hashes")
	}

	// Same body, different facing.
	hC := stateHash(DirUp, bodyA)
	if hA == hC {
		t.Error("different facing should produce different hash")
	}
}

// ---------------------------------------------------------------------------
// floodDist — BFS flood from a single point
// ---------------------------------------------------------------------------

// TestFloodDistOpenGrid verifies flood distance on an open grid.
// From the centre of a 5×3 open area, every reachable cell should have
// a finite distance and the total count should match the open cell count.
func TestFloodDistOpenGrid(t *testing.T) {
	setupGrid(flatFloor) // 7×5 grid, 5×3 open interior

	start := Point{3, 2}
	count, dist := floodDist(start, nil)

	// 5×3 = 15 open cells.
	if count != 15 {
		t.Errorf("flood count = %d, want 15", count)
	}
	// Start cell has distance 0.
	if dist[start.Y*W+start.X] != 0 {
		t.Error("start cell should have distance 0")
	}
	// A wall cell stays Unreachable.
	if dist[0] != Unreachable {
		t.Error("wall cell (0,0) should be Unreachable")
	}
}

// TestFloodDistWithBlocker verifies that blocked cells are not traversed.
// This simulates body or enemy occupancy blocking pathfinding.
func TestFloodDistWithBlocker(t *testing.T) {
	setupGrid(flatFloor)

	// Block the centre column at x=3, splitting the grid.
	blocker := setupOcc([]Point{{3, 1}, {3, 2}, {3, 3}})
	start := Point{1, 2}
	count, dist := floodDist(start, &blocker)

	// Left half: x=1,2 × y=1,2,3 = 6 cells.
	if count != 6 {
		t.Errorf("flood with blocker: count = %d, want 6", count)
	}
	// Cell on the other side of the blocker should be unreachable.
	if dist[2*W+5] != Unreachable {
		t.Error("cell past blocker should be Unreachable")
	}
}

// TestFloodDistFromWall verifies that starting from a wall cell returns
// count 0 (no cells reachable).
func TestFloodDistFromWall(t *testing.T) {
	setupGrid(flatFloor)
	count, _ := floodDist(Point{0, 0}, nil)
	if count != 0 {
		t.Errorf("flood from wall: count = %d, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// cmdFlood — BFS from a snake head with simulation-aware landing
// ---------------------------------------------------------------------------

// TestCmdFloodBasic verifies that cmdFlood explores cells reachable from
// the snake head after one simulated step.  Unlike floodDist, cmdFlood
// uses simMove for the first step to account for gravity and collisions,
// then standard BFS for subsequent steps.
func TestCmdFloodBasic(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{3, 3}, {2, 3}}
	facing := DirRight
	occ := setupOcc(nil)

	count, dist := cmdFlood(body, facing, &occ)

	// Should reach a substantial portion of the grid.
	if count < 5 {
		t.Errorf("cmdFlood count = %d, want ≥ 5", count)
	}
	// Head cell has distance 0.
	if dist[body[0].Y*W+body[0].X] != 0 {
		t.Error("head should have distance 0")
	}
}

// TestCmdFloodBlockedHead verifies that cmdFlood returns 0 when the head
// is on a wall or occupied cell (degenerate case).
func TestCmdFloodBlockedHead(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{0, 0}} // head on a wall
	occ := setupOcc(nil)

	count, _ := cmdFlood(body, DirUp, &occ)
	if count != 0 {
		t.Errorf("blocked head: count = %d, want 0", count)
	}
}
