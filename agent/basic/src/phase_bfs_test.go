package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// Phase 1: BFS for both sides
// ============================================================

// Verify BFS produces results for all alive snakes (3 mine + 3 enemy).
func TestPhaseBFS_BothSides(t *testing.T) {
	_, _, d := testDecision()
	d.phaseBFS()

	assert.Equal(t, 3, len(d.MySnakes), "my snakes count")
	assert.Equal(t, 3, len(d.OpSnakes), "enemy snakes count")
	assert.Equal(t, 3, len(d.BFS), "my BFS results count")
	assert.Equal(t, 3, len(d.OpBFS), "enemy BFS results count")

	for i, bfs := range d.BFS {
		assert.NotNil(t, bfs, "my BFS[%d] nil", i)
	}
	for i, bfs := range d.OpBFS {
		assert.NotNil(t, bfs, "op BFS[%d] nil", i)
	}
}

// Snake 0 (mine, head 16,10) should reach (18,9) in 4 steps going RIGHT
// after running BFS for all snakes (not just mine).
func TestPhaseBFS_Snake0_Dist(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()

	// Snake 0 is first in MySnakes
	bfs := d.BFS[0]
	target := g.Idx(18, 9)
	assert.Equal(t, 4, bfs[target].Dist, "Snake 0 → (18,9) dist")
	assert.Equal(t, DR, bfs[target].FirstDir, "Snake 0 → (18,9) dir")
}
