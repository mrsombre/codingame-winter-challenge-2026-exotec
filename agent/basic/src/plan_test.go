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
// Unit tests — BFS, surfaces, etc.
// ============================================================

func TestLandY(t *testing.T) {
	_, p := testPlan()
	g := p.g

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
			cell := p.g.Idx(x, s.Y)
			assert.Equal(t, id, p.SurfAt[cell], "SurfAt(%d,%d)", x, s.Y)
		}
	}
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
