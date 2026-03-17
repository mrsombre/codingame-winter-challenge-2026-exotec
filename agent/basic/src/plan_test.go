package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testPlan() (*Game, *Plan) {
	g := testGameFull()
	p := &Plan{Sim: NewSim(g)}
	p.Precompute()
	return g, p
}

// ============================================================
// Unit tests — BFS, surfaces, etc.
// ============================================================

func TestLandY(t *testing.T) {
	_, p := testPlan()
	g := p.G

	// Bottom row walls: LandY = y (wall itself)
	for x := 0; x < g.W; x++ {
		idx := g.Idx(x, g.H-1)
		if !g.Cell[idx] {
			assert.Equal(t, g.H-1, p.LandY[idx], "wall at (%d,%d)", x, g.H-1)
		}
	}

	// Free cells above bottom wall: LandY > y
	for y := 0; y < g.H-1; y++ {
		for x := 0; x < g.W; x++ {
			idx := g.Idx(x, y)
			if g.Cell[idx] {
				assert.True(t, p.LandY[idx] >= y, "LandY(%d,%d) should be >= y", x, y)
			}
		}
	}
}

func TestSurfaces(t *testing.T) {
	_, p := testPlan()

	assert.True(t, len(p.Surfs) > 0, "should detect surfaces")

	// Every surface cell should have SurfAt pointing to its surface.
	for id, s := range p.Surfs {
		for x := s.Left; x <= s.Right; x++ {
			cell := p.G.Idx(x, s.Y)
			assert.Equal(t, id, p.SurfAt[cell], "SurfAt(%d,%d)", x, s.Y)
		}
	}
}

func hasLink(adj []SurfLink, to, cost, minBody int) bool {
	for _, l := range adj {
		if l.To == to && l.Cost == cost && l.MinBody == minBody {
			return true
		}
	}
	return false
}

// TestSurfacesDetail locks in surface detection for the testSeed grid (28x15).
//
//	y= 5 |..........0......1..........|
//	y= 6 |.........2#......#3.........|
//	y= 7 |..4...5..#........#..6...7..|
//	y= 8 |8.#.99#..............#aa.#.b|
//	y=10 |#cc.##................##.dd#|
//	y=11 |###e....................f###|
//	y=12 |####g..h...iiiiii...j..k####|
//	y=13 |#####ll#mmm######nnn#oo#####|
func TestSurfacesDetail(t *testing.T) {
	_, p := testPlan()

	// Exact count
	assert.Equal(t, 25, len(p.Surfs), "surface count")

	// Spot-check key surfaces
	tests := []struct {
		id          int
		y, left, right int
	}{
		{0, 5, 10, 10},  // single cell above wall
		{1, 5, 17, 17},  // symmetric partner
		{9, 8, 4, 5},    // 2-cell ledge
		{18, 12, 11, 16}, // widest surface (6 cells)
		{21, 13, 5, 6},   // bottom platform
		{24, 13, 21, 22}, // bottom right
	}
	for _, tt := range tests {
		s := p.Surfs[tt.id]
		assert.Equal(t, tt.y, s.Y, "S%d Y", tt.id)
		assert.Equal(t, tt.left, s.Left, "S%d Left", tt.id)
		assert.Equal(t, tt.right, s.Right, "S%d Right", tt.id)
	}

	// Adjacency spot-checks (Manhattan edge-to-edge)
	assert.True(t, hasLink(p.SurfAdj[0], 2, 2, 2), "S0→S2 (10,5)→(9,6) cost=2")
	assert.True(t, hasLink(p.SurfAdj[2], 0, 2, 2), "S2→S0 reverse")
	assert.True(t, hasLink(p.SurfAdj[18], 22, 2, 2), "S18→S22 (11,12)→(10,13)")
	assert.True(t, hasLink(p.SurfAdj[18], 23, 2, 2), "S18→S23 (16,12)→(17,13)")
	assert.True(t, hasLink(p.SurfAdj[21], 16, 2, 2), "S21→S16 (5,13)→(4,12)")
	assert.True(t, hasLink(p.SurfAdj[21], 17, 2, 2), "S21→S17 (6,13)→(7,12)")

	// Same-Y connection
	assert.True(t, hasLink(p.SurfAdj[17], 18, 4, 4), "S17→S18 same-Y (7,12)→(11,12) cost=4")
	assert.True(t, hasLink(p.SurfAdj[18], 17, 4, 4), "S18→S17 reverse")

	// Bidirectionality: every A→B has B→A
	for i, links := range p.SurfAdj {
		for _, l := range links {
			assert.True(t, hasLink(p.SurfAdj[l.To], i, l.Cost, l.MinBody),
				"S%d→S%d cost=%d missing reverse", i, l.To, l.Cost)
		}
	}

	// SurfDist: body-length-gated reachability
	d4 := p.SurfDist(21, 0, 5)
	assert.True(t, d4 > 0, "S21→S0 reachable with bodyLen=5, got %d", d4)

	d1 := p.SurfDist(21, 0, 1)
	assert.Equal(t, -1, d1, "S21→S0 unreachable with bodyLen=1")
}

func TestBFSFindAll_HeadReachable(t *testing.T) {
	g, p := testPlan()

	body := g.Sn[0].Body
	bfs := p.BFSFindAll(body)
	assert.NotNil(t, bfs)

	head := body[0]
	assert.Equal(t, 0, bfs[head].Dist, "head dist = 0")
}

func TestBFSFindAll_WallUnreachable(t *testing.T) {
	g, p := testPlan()

	body := g.Sn[0].Body
	bfs := p.BFSFindAll(body)

	// Wall cells should be unreachable.
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			cell := g.Idx(x, y)
			if !g.Cell[cell] {
				assert.Equal(t, -1, bfs[cell].Dist, "wall (%d,%d) should be unreachable", x, y)
			}
		}
	}
}
