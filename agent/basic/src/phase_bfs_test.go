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
	p := &Plan{G: g}
	p.Init()

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

	sn := &g.Sn[0]
	head := sn.Body[0]
	sid := g.SurfAt[head]
	entries := []SurfReach{{SurfID: sid, Dist: 0, FirstDir: -1, Landing: head}}
	reach := surfGraphReach(g, entries, sn.Len, head, sn.Dir)
	assert.True(t, len(reach) > 0, "should find apple at (0,3)")
	assert.Equal(t, g.Idx(0, 3), reach[0].Apple)
	assert.Equal(t, DL, reach[0].FirstDir)
}

func TestPhaseBFSPopulatesTargets(t *testing.T) {
	g := testBFSGame()
	g.Ap = []int{g.Idx(0, 3)}
	g.ANum = 1
	p := &Plan{G: g}
	p.Init()

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

	d := &Decision{G: g, P: p}
	d.phaseBFS()

	assert.Equal(t, []int{0}, d.MySnakes)
	assert.Equal(t, []int{1}, d.OpSnakes)
	mySnIdx := d.MySnakes[0]
	assert.Len(t, d.BFS.Reach[mySnIdx], 1)
	assert.Equal(t, g.Idx(0, 3), d.BFS.Reach[mySnIdx][0].Apple)
	assert.Equal(t, -1, d.Assigned[0], "phaseBFS no longer assigns targets")
	opSnIdx := d.OpSnakes[0]
	assert.Len(t, d.BFS.Reach[opSnIdx], 1)
}
