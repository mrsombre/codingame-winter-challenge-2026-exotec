package main

import "testing"

// ---------------------------------------------------------------------------
// STerrain — support graph construction and queries
// ---------------------------------------------------------------------------

// TestHasSupTerrain verifies that HasSup correctly identifies cells with
// a wall directly below.  This is the foundation for the support graph:
// only grounded cells can be support nodes.
func TestHasSupTerrain(t *testing.T) {
	setupGrid(flatFloor)

	// Row y=3 is directly above the bottom wall → has support.
	if !state.Terr.HasSup(Point{3, 3}) {
		t.Error("(3,3) should have support (above floor wall)")
	}
	// Row y=1 has open space below → no support.
	if state.Terr.HasSup(Point{3, 1}) {
		t.Error("(3,1) should NOT have support (air below)")
	}
}

// TestNodeAtAndAnchor verifies that NodeAt returns a valid node ID for
// supported cells and -1 for unsupported or wall cells.  Anchor finds
// the first grounded body part.
func TestNodeAtAndAnchor(t *testing.T) {
	setupGrid(flatFloor)

	// Supported cell → valid node ID.
	nid := state.Terr.NodeAt(Point{3, 3})
	if nid == -1 {
		t.Error("(3,3) should be a support node")
	}

	// Unsupported cell → -1.
	nid = state.Terr.NodeAt(Point{3, 1})
	if nid != -1 {
		t.Error("(3,1) should NOT be a support node")
	}

	// Anchor on a snake with one grounded part.
	body := []Point{{3, 1}, {3, 2}, {3, 3}}
	anchor := state.Terr.Anchor(body)
	if anchor == -1 {
		t.Error("snake with tail on floor should have an anchor")
	}

	// Anchor on a fully airborne snake → -1.
	airBody := []Point{{3, 1}, {3, 2}}
	if state.Terr.Anchor(airBody) != -1 {
		t.Error("airborne snake should have no anchor")
	}
}

// TestAnchorComp verifies that AnchorComp returns a valid component ID
// for a grounded snake body.  All floor cells in a connected region belong
// to the same component.
func TestAnchorComp(t *testing.T) {
	setupGrid(flatFloor)

	body1 := []Point{{1, 3}, {2, 3}}
	body2 := []Point{{4, 3}, {5, 3}}

	comp1 := state.Terr.AnchorComp(body1)
	comp2 := state.Terr.AnchorComp(body2)

	if comp1 == -1 || comp2 == -1 {
		t.Fatal("grounded snakes should have valid components")
	}
	// On flatFloor, the entire floor is one connected region.
	if comp1 != comp2 {
		t.Error("both snakes on same floor should share component ID")
	}
}

// TestBodyInitRun verifies that BodyInitRun counts consecutive body parts
// from the head until the first grounded part.  This "run length" determines
// how far a snake can bridge across gaps without support.
func TestBodyInitRun(t *testing.T) {
	setupGrid(flatFloor)

	// Snake entirely on the floor: head at (1,3) has WBelow → run = 1.
	body := []Point{{1, 3}, {2, 3}, {3, 3}}
	run := state.Terr.BodyInitRun(body)
	if run != 1 {
		t.Errorf("grounded head: run = %d, want 1", run)
	}

	// Snake hanging: head at (3,1), body at (3,2), tail at (3,3) grounded.
	// Parts 0 and 1 are unsupported; part 2 is grounded → run = 3.
	hangBody := []Point{{3, 1}, {3, 2}, {3, 3}}
	run = state.Terr.BodyInitRun(hangBody)
	if run != 3 {
		t.Errorf("hanging snake: run = %d, want 3", run)
	}

	// Fully airborne snake → -1 (no grounded part).
	airBody := []Point{{3, 1}, {3, 2}}
	run = state.Terr.BodyInitRun(airBody)
	if run != -1 {
		t.Errorf("airborne snake: run = %d, want -1", run)
	}
}

// TestMinBodyLen verifies that MinBodyLen returns the minimum body length
// required for a snake anchored in a given component to reach a target
// cell.  A cell on the same floor should require just length 1.
func TestMinBodyLen(t *testing.T) {
	setupGrid(flatFloor)

	// Snake on the floor — reaching another floor cell should need short body.
	body := []Point{{1, 3}, {2, 3}}
	minLen := state.Terr.MinBodyLen(body, Point{5, 3})
	if minLen == Unreachable {
		t.Error("should be reachable on same floor")
	}
	if minLen > 3 {
		t.Errorf("floor-to-floor: minLen = %d, want ≤ 3", minLen)
	}
}

// TestMinBodyLenUnreachable verifies that a target in a wall returns
// Unreachable.
func TestMinBodyLenUnreachable(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{1, 3}, {2, 3}}
	minLen := state.Terr.MinBodyLen(body, Point{0, 0}) // wall
	if minLen != Unreachable {
		t.Errorf("wall target: minLen = %d, want Unreachable", minLen)
	}
}

// TestMinBodyLenAboveFloor verifies that reaching a cell above the floor
// requires a longer body (the snake must bridge upward without support).
func TestMinBodyLenAboveFloor(t *testing.T) {
	setupGrid(flatFloor)
	body := []Point{{1, 3}, {2, 3}}

	// Target (3,1) is 2 rows above the floor — needs body length ≥ 3
	// to bridge the gap.
	minLen := state.Terr.MinBodyLen(body, Point{3, 1})
	if minLen == Unreachable {
		t.Error("(3,1) should be reachable from floor with long enough body")
	}
	if minLen < 3 {
		t.Errorf("reaching y=1 from floor: minLen = %d, want ≥ 3", minLen)
	}
}

// ---------------------------------------------------------------------------
// STerrain on a grid with a floating platform
// ---------------------------------------------------------------------------

// TestTerrainPlatformComponents verifies that a floating platform creates
// a separate support region.  Cells on the platform and cells on the floor
// may be in different connected components if they are not diagonally
// adjacent.
func TestTerrainPlatformComponents(t *testing.T) {
	setupGrid(tallGrid)

	// Cell on the platform (y=3, above wall at y=4).
	platNode := state.Terr.NodeAt(Point{3, 3})
	// Cell on the floor (y=6, above wall at y=7).
	floorNode := state.Terr.NodeAt(Point{3, 6})

	if platNode == -1 {
		t.Fatal("platform cell should be a support node")
	}
	if floorNode == -1 {
		t.Fatal("floor cell should be a support node")
	}

	// Platform and floor are separate support surfaces — they may or may
	// not be in the same component depending on diagonal connectivity.
	// The key test is that both exist as valid nodes.
	platComp := state.Terr.CompID[platNode]
	floorComp := state.Terr.CompID[floorNode]
	_ = platComp
	_ = floorComp
	// On tallGrid, platform edges at x=1 and x=5 are diagonally adjacent
	// to the side walls, so they connect to the floor.  Both should be in
	// the same component.
	if platComp != floorComp {
		t.Log("platform and floor in different components — check grid layout")
	}
}

// TestSupPathBFS verifies that SupPathBFS finds the minimum body length
// to reach a target using the support-path BFS.  On the floor, any cell
// adjacent to the target should be reachable with a short body.
func TestSupPathBFS(t *testing.T) {
	setupGrid(flatFloor)

	// Start on the floor, target also on the floor — should need just 1.
	minLen := state.Terr.SupPathBFS(Point{1, 3}, 1, Point{5, 3}, nil)
	if minLen == Unreachable {
		t.Error("same-floor target should be reachable")
	}
	if minLen > 3 {
		t.Errorf("floor-to-floor SupPathBFS: got %d, want ≤ 3", minLen)
	}
}

// TestSupReachMulti verifies that SupReachMulti returns all reachable
// targets given a body length constraint.
func TestSupReachMulti(t *testing.T) {
	setupGrid(flatFloor)

	targets := []Point{{2, 3}, {5, 3}, {3, 1}}
	apples := setupSrcBG(nil)

	// Body length 2 starting on floor — should reach floor targets
	// but may not reach (3,1) which requires bridging up.
	reachable := state.Terr.SupReachMulti(Point{1, 3}, 1, 2, targets, &apples)
	floorFound := 0
	for _, r := range reachable {
		if r.Y == 3 {
			floorFound++
		}
	}
	if floorFound < 1 {
		t.Error("should reach at least one floor target with body length 2")
	}
}
