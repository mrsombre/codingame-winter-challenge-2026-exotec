package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPlan() (*Game, *Plan) {
	g := testGameFull()
	p := &Plan{g: g}
	p.Precompute()
	return g, p
}

// ============================================================
// Acceptance tests — specific scenarios on the seed map (22×12)
// ============================================================

// Snake 0: head (3,8), body [(3,8),(3,9),(3,10)], bodyLen=3
// Target: apple at (9,9)
//
// Grid context:
//   ......##......##......   y=8
//   .#........##........#.   y=9  ← path: 4,9 → 5,9 → ... → 9,9
//   ##..##############..##   y=10 ← wall (support)
//   ######################   y=11
//
// Expected: RIGHT 6 steps along row 9
//   right→(4,9) right→(5,9) right→(6,9) right→(7,9) right→(8,9) right→(9,9)
func TestAccept_Snake0_To_9_9(t *testing.T) {
	g, p := testPlan()

	body := g.Sn[0].Body // [(3,8),(3,9),(3,10)]
	target := g.Idx(9, 9)

	// --- BFS ---
	bfs := p.BFSFindAll(body)
	assert.Equal(t, 6, bfs[target].Dist, "BFS dist")
	assert.Equal(t, DR, bfs[target].FirstDir, "BFS firstDir")

	// verify full path cells
	path := []struct{ x, y, dist int }{
		{4, 9, 1}, {5, 9, 2}, {6, 9, 3},
		{7, 9, 4}, {8, 9, 5}, {9, 9, 6},
	}
	for _, step := range path {
		cell := g.Idx(step.x, step.y)
		assert.Equal(t, step.dist, bfs[cell].Dist, "(%d,%d) dist", step.x, step.y)
		assert.Equal(t, DR, bfs[cell].FirstDir, "(%d,%d) dir", step.x, step.y)
	}

	// --- Surface graph estimate ---
	dist, dir := p.EstimateDist(body, target)
	assert.True(t, dist >= 0, "EstimateDist reachable")
	assert.Equal(t, DR, dir, "EstimateDist firstDir")
}

// Snake 2: head (16,0), body [(16,0),(16,1),(16,2)], bodyLen=3
// Target: apple at (14,1)
//
// Grid context:
//   ......................   y=0  head=16
//   ......................   y=1  body=16  target=(14,1)
//   ......................   y=2  body=16
//   ....##...####...##....   y=3  (16,3)='#' support
//
// Real game (2 turns): left→(15,1); left→(14,1) eats apple, grows to 4 segs,
//   tail stays on wall → no fall.
//
// BFS LIMITATION: from (15,1) ag=2, left to (14,1) gives ag=3=bodyLen → fall.
// BFS doesn't model apple eating (body growth from 3→4 prevents the fall).
// So (14,1) is unreachable in BFS — a known limitation for apples in free space
// that require eating to maintain support.
func TestAccept_Snake2_To_14_1(t *testing.T) {
	g, p := testPlan()

	body := g.Sn[2].Body // [(16,0),(16,1),(16,2)]
	target := g.Idx(14, 1)

	// BFS models apple eating: stepping onto apple grows body by 1,
	// preventing fall. Real game: left→(15,1); left→(14,1) eats apple.
	bfs := p.BFSFindAll(body)
	assert.Equal(t, 2, bfs[target].Dist, "BFS: (14,1) reachable in 2 steps")
	assert.Equal(t, DL, bfs[target].FirstDir, "BFS: first move LEFT")
}

// Snake 2: head (16,0), body [(16,0),(16,1),(16,2)], bodyLen=3
// Target: apple at (19,6)
//
// Grid context:
//   ##.......#..#.......##   y=6  target=(19,6)
//   ####..#........#..####   y=7  (19,7)='#' support
//
// BFS path (5 steps, body-aware fall):
//   right→(17,1) ag=2  [first move, fall 1 cell]
//   right→(18,2) ag=1  [ag=3→fall, min(LandY cols 16-18)=2, body[1] grounded]
//   down→(18,3) ag=2
//   down→(18,6) ag=0   [ag=3→fall, body extends up col 18, lands on (18,7)='#']
//   right→(19,6) ag=0  [grounded on (19,7)]
func TestAccept_Snake2_To_19_6(t *testing.T) {
	g, p := testPlan()

	body := g.Sn[2].Body // [(16,0),(16,1),(16,2)]
	target := g.Idx(19, 6)

	bfs := p.BFSFindAll(body)
	assert.Equal(t, 5, bfs[target].Dist, "BFS dist 5 steps")
	assert.Equal(t, DR, bfs[target].FirstDir, "BFS firstDir RIGHT")

	// Verify intermediate cells along the path
	path := []struct{ x, y, dist int }{
		{18, 2, 2}, // fall limited by body cols → y=2 not y=6
		{18, 3, 3},
		{18, 6, 4}, // second fall → (18,6)
		{19, 6, 5}, // target
	}
	for _, step := range path {
		cell := g.Idx(step.x, step.y)
		r := bfs[cell]
		assert.Equal(t, step.dist, r.Dist, "(%d,%d) dist", step.x, step.y)
	}
}
