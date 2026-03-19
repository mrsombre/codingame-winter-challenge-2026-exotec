package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func hasSurfReach(results []SurfReach, surfID int) (SurfReach, bool) {
	for _, sr := range results {
		if sr.SurfID == surfID {
			return sr, true
		}
	}
	return SurfReach{}, false
}

func TestSurfBFSGrounded(t *testing.T) {
	// Snake on ground, surfaces to left and right
	//  .....
	//  .....
	//  ##.##
	g := testGridInput([]string{
		".....",
		".....",
		"##.##",
	})

	g.SNum = 1
	sn := &g.Sn[0]
	sn.ID = 0
	sn.Owner = 0
	sn.Alive = true
	sn.Body = []int{g.Idx(2, 0), g.Idx(2, 1), g.Idx(2, 2)} // vertical, head up, gap at (2,2) is free
	sn.Len = 3
	sn.Dir = DU
	sn.Sp = 2 // tail on ground (y=2 is free but below y=2 is OOB=grounded)

	// Actually the gap at (2,2) is free — surfaces are at y=1 above ##
	// Left surface: (0,1) and (1,1) — cells above ## at y=2
	// Right surface: (3,1) and (4,1)
	leftSID := g.SurfAt[g.Idx(0, 1)]
	rightSID := g.SurfAt[g.Idx(3, 1)]
	assert.True(t, leftSID >= 0, "left surface exists")
	assert.True(t, rightSID >= 0, "right surface exists")

	sim := NewSim(g)
	sim.RebuildAppleMap()
	results := sim.SurfBFS(sn)

	assert.True(t, len(results) > 0, "should find surfaces")

	if sr, ok := hasSurfReach(results, leftSID); ok {
		assert.Equal(t, DL, sr.FirstDir, "left surface reached by moving left")
	}
	if sr, ok := hasSurfReach(results, rightSID); ok {
		assert.Equal(t, DR, sr.FirstDir, "right surface reached by moving right")
	}
}

func TestSurfBFSTailSupport(t *testing.T) {
	// Snake hanging vertically with only tail on ground
	// UP should be rejected (noop)
	//  .....
	//  .....
	//  .....
	//  .....
	//  #####
	g := testGridInput([]string{
		".....",
		".....",
		".....",
		".....",
		"#####",
	})

	g.SNum = 1
	sn := &g.Sn[0]
	sn.ID = 0
	sn.Owner = 0
	sn.Alive = true
	sn.Body = []int{g.Idx(2, 1), g.Idx(2, 2), g.Idx(2, 3)} // vertical, head at (2,1)
	sn.Len = 3
	sn.Dir = DU
	sn.Sp = 2 // tail at (2,3) on wall

	// Ground surface at y=3 (above #####)
	groundSID := g.SurfAt[g.Idx(0, 3)]
	assert.True(t, groundSID >= 0, "ground surface exists")

	sim := NewSim(g)
	sim.RebuildAppleMap()
	results := sim.SurfBFS(sn)

	// Should find the ground surface via LEFT or RIGHT (diagonal fall)
	sr, ok := hasSurfReach(results, groundSID)
	assert.True(t, ok, "ground surface should be reachable")
	assert.NotEqual(t, DU, sr.FirstDir, "UP should not be the first dir (noop)")
	assert.True(t, sr.FirstDir == DL || sr.FirstDir == DR, "should reach via L/R diagonal")
}

func TestSurfBFSBlockedByOpponent(t *testing.T) {
	// Opponent blocks path to a surface
	//  .....
	//  .....
	//  .....
	//  #####
	g := testGridInput([]string{
		".....",
		".....",
		".....",
		"#####",
	})

	g.SNum = 2
	// My snake at (2,0) heading up
	sn := &g.Sn[0]
	sn.ID = 0
	sn.Owner = 0
	sn.Alive = true
	sn.Body = []int{g.Idx(2, 0), g.Idx(2, 1), g.Idx(2, 2)}
	sn.Len = 3
	sn.Dir = DU
	sn.Sp = 2

	// Opponent blocking right side at y=2 — horizontal wall of body
	op := &g.Sn[1]
	op.ID = 1
	op.Owner = 1
	op.Alive = true
	op.Body = []int{g.Idx(3, 2), g.Idx(4, 2), g.Idx(4, 1)}
	op.Len = 3
	op.Dir = DL

	g.MyN = 1
	g.MyIDs = [MaxPSn]int{0}
	g.OpN = 1
	g.OpIDs = [MaxPSn]int{1}

	sim := NewSim(g)
	sim.RebuildAppleMap()
	results := sim.SurfBFS(sn)

	// If the opponent blocks the right path, the right-side surface should
	// NOT be reachable (or only reachable via a longer path avoiding the opponent)
	for _, sr := range results {
		// Verify no result has landing on an obstacle cell
		assert.False(t, sim.obstacleMap[sr.Landing], "landing should not be on obstacle")
	}
}

func TestSurfBFSMovableTail(t *testing.T) {
	// Opponent tail is movable (no apple nearby), path goes through it
	//  .....
	//  .....
	//  .....
	//  #####
	g := testGridInput([]string{
		".....",
		".....",
		".....",
		"#####",
	})
	g.ANum = 0
	g.Ap = g.Ap[:0]

	g.SNum = 2
	sn := &g.Sn[0]
	sn.ID = 0
	sn.Owner = 0
	sn.Alive = true
	sn.Body = []int{g.Idx(2, 0), g.Idx(2, 1), g.Idx(2, 2)}
	sn.Len = 3
	sn.Dir = DU
	sn.Sp = 2

	// Opponent: tail at (3,2), head at (3,0) — tail is movable (no apples)
	op := &g.Sn[1]
	op.ID = 1
	op.Owner = 1
	op.Alive = true
	op.Body = []int{g.Idx(3, 0), g.Idx(3, 1), g.Idx(3, 2)}
	op.Len = 3
	op.Dir = DU

	g.MyN = 1
	g.MyIDs = [MaxPSn]int{0}
	g.OpN = 1
	g.OpIDs = [MaxPSn]int{1}

	sim := NewSim(g)
	sim.RebuildAppleMap()

	// Verify tail is movable
	assert.True(t, sim.isTailMovable(op), "opponent tail should be movable (no apples nearby)")

	// Verify (3,2) is NOT an obstacle
	sim.buildObstacleMap(sn.ID)
	assert.False(t, sim.obstacleMap[g.Idx(3, 2)], "movable tail should not be obstacle")
	// But (3,0) and (3,1) ARE obstacles
	assert.True(t, sim.obstacleMap[g.Idx(3, 0)], "opponent head should be obstacle")
	assert.True(t, sim.obstacleMap[g.Idx(3, 1)], "opponent body should be obstacle")
}

func TestSurfBFSAlreadyOnSurface(t *testing.T) {
	// Snake already on a surface — should find neighboring surfaces
	//  .....
	//  ##.##
	//  .....
	//  #####
	g := testGridInput([]string{
		".....",
		"##.##",
		".....",
		"#####",
	})

	g.SNum = 1
	sn := &g.Sn[0]
	sn.ID = 0
	sn.Owner = 0
	sn.Alive = true
	sn.Body = []int{g.Idx(1, 0), g.Idx(0, 0), g.Idx(-1, 0)} // on left surface
	sn.Len = 3
	sn.Dir = DR
	sn.Sp = 0 // head is supported

	surfHead := g.SurfAt[g.Idx(1, 0)]
	assert.True(t, surfHead >= 0, "head should be on surface")

	sim := NewSim(g)
	sim.RebuildAppleMap()
	results := sim.SurfBFS(sn)

	// Should find at least the ground surface
	groundSID := g.SurfAt[g.Idx(0, 2)]
	assert.True(t, groundSID >= 0, "ground surface exists")

	_, found := hasSurfReach(results, groundSID)
	assert.True(t, found, "ground surface should be reachable")
}
