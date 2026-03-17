package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// testBFSGame creates a simple 7x5 grid with one solid surface at y=3.
// Grid layout:
//
//	.......   y=0
//	.......   y=1
//	.......   y=2
//	.......   y=3  <- surface x=0..6
//	#######   y=4
func testBFSGame() *Game {
	g := testGridInput([]string{
		".......",
		".......",
		".......",
		".......",
		"#######",
	})
	return g
}

func TestSurfaceReachBasicDirect(t *testing.T) {
	g := testBFSGame()
	// Apple to the LEFT of head — directly reachable (no neck block)
	g.Ap = []int{g.Idx(0, 3)}
	g.ANum = 1
	g.BuildSurfaceGraph()

	g.MyN = 1
	g.MyIDs = [MaxPSn]int{0}
	g.OpN = 0
	g.SNum = 1
	// Head at (1,3), neck at (2,3) → DR blocked, DL free
	g.Sn[0] = Snake{
		ID: 0, Owner: 0, Alive: true,
		Body: []int{g.Idx(1, 3), g.Idx(2, 3), g.Idx(3, 3)},
		Len:  3,
	}

	reach := surfaceReach(g, &g.Sn[0], true)
	assert.True(t, len(reach) > 0, "should find apple at (0,3)")
	assert.Equal(t, g.Idx(0, 3), reach[0].Apple)
	assert.Equal(t, 1, reach[0].Dist, "1 step left from head")
	assert.Equal(t, DL, reach[0].FirstDir)
}

func TestSurfaceReachBehindNeckUnreachable(t *testing.T) {
	g := testBFSGame()
	// Apple at (5,3) — to the RIGHT of head, neck blocks DR.
	// On a single flat surface with no other surfaces to route around,
	// the apple is genuinely unreachable this turn.
	g.Ap = []int{g.Idx(5, 3)}
	g.ANum = 1
	g.BuildSurfaceGraph()

	g.MyN = 1
	g.MyIDs = [MaxPSn]int{0}
	g.OpN = 0
	g.SNum = 1
	g.Sn[0] = Snake{
		ID: 0, Owner: 0, Alive: true,
		Body: []int{g.Idx(1, 3), g.Idx(2, 3), g.Idx(3, 3)},
		Len:  3,
	}

	reach := surfaceReach(g, &g.Sn[0], true)
	for _, r := range reach {
		assert.NotEqual(t, g.Idx(5, 3), r.Apple,
			"apple directly blocked by neck with no alternate route should not be found")
	}
}

func TestSurfaceReachBlockedByAppleLinkLength(t *testing.T) {
	g := testGridInput([]string{
		".......",
		".......",
		".......",
		".......",
		".......",
		"#######",
	})
	g.Ap = []int{g.Idx(3, 0)}
	g.ANum = 1
	g.BuildSurfaceGraph()

	sn := &Snake{
		ID: 0, Owner: 0, Alive: true,
		Body: []int{g.Idx(3, 4), g.Idx(2, 4), g.Idx(1, 4)},
		Len:  3,
	}

	reach := surfaceReach(g, sn, true)
	assert.False(t, hasReachApple(reach, g.Idx(3, 0)),
		"apple link longer than snake length should be rejected")
}

func TestPhaseBFSPopulatesTargets(t *testing.T) {
	g := testBFSGame()
	g.Ap = []int{g.Idx(0, 3)}
	g.ANum = 1
	g.BuildSurfaceGraph()

	g.SNum = 2
	g.Sn[0] = Snake{
		ID: 0, Owner: 0, Alive: true,
		Body: []int{g.Idx(1, 3), g.Idx(2, 3), g.Idx(3, 3)},
		Len:  3,
	}
	g.Sn[1] = Snake{
		ID: 3, Owner: 1, Alive: true,
		Body: []int{g.Idx(5, 3), g.Idx(4, 3), g.Idx(3, 3)},
		Len:  3,
	}

	d := &Decision{G: g, P: &Plan{G: g}}
	d.phaseBFS()

	assert.Equal(t, []int{0}, d.MySnakes)
	assert.Equal(t, []int{1}, d.OpSnakes)
	assert.Len(t, d.BFS.MyReach[0], 1)
	assert.Equal(t, g.Idx(0, 3), d.BFS.MyReach[0][0].Apple)
	assert.Equal(t, g.Idx(0, 3), d.Assigned[0])
	assert.Equal(t, DL, d.AssignedDir[0])
	assert.Len(t, d.BFS.OpReach[0], 1)
}
