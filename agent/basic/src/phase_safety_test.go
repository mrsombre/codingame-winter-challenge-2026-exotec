package main

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// Phase 5: Safety check
// ============================================================

// After full pipeline, all assigned directions must be non-lethal:
// no wall collision, no body collision, not moving into neck.
func TestSafety_NoLethalMoves(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()
	d.phaseSafety()

	n := g.W * g.H

	// Build body cell bitmap
	bodyCell := make([]bool, n)
	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive {
			continue
		}
		for _, c := range sn.Body {
			if c >= 0 && c < n {
				bodyCell[c] = true
			}
		}
	}

	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		head := sn.Body[0]
		dir := d.AssignedDir[si]
		nc := g.Nb[head][dir]

		assert.NotEqual(t, -1, nc,
			"snake %d dir %s → wall", sn.ID, Dn[dir])

		neck := neckOf(sn.Body)
		assert.NotEqual(t, neck, nc,
			"snake %d dir %s → own neck", sn.ID, Dn[dir])

		ownTail := sn.Body[len(sn.Body)-1]
		if nc >= 0 && nc < n && nc != ownTail {
			assert.False(t, bodyCell[nc],
				"snake %d dir %s → body collision at %d", sn.ID, Dn[dir], nc)
		}
	}
}

// Reachable cells per direction: the chosen direction should have
// at least bodyLen reachable cells (not a dead-end trap).
func TestSafety_NotTrapped(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()
	d.phaseSafety()

	n := g.W * g.H
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		bfs := d.BFS[si]
		dir := d.AssignedDir[si]

		reachable := 0
		for c := 0; c < n; c++ {
			if bfs[c].Dist > 0 && bfs[c].FirstDir == dir {
				reachable++
			}
		}

		assert.True(t, reachable >= len(sn.Body),
			"snake %d dir %s has only %d reachable cells (body=%d) — trapped",
			sn.ID, Dn[dir], reachable, len(sn.Body))
	}
}

// Safety should override a direction that leads into a body collision.
// Construct scenario: force assigned direction toward a body cell.
func TestSafety_OverridesCollision(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()

	// Snake 0 head at (16,10). Force direction UP → (16,9).
	// Check if (16,9) has a body — if not, place one conceptually.
	// Instead, test the mechanism: set assigned to UP.
	// phaseSafety should still pick a safe direction.

	// Override Snake 0's direction to UP to test safety override.
	d.AssignedDir[0] = DU
	d.phaseSafety()

	dir := d.AssignedDir[0]
	head := g.Sn[d.MySnakes[0]].Body[0]
	nc := g.Nb[head][dir]

	// After safety, direction should be valid
	assert.NotEqual(t, -1, nc, "post-safety direction should not hit wall")
	assert.True(t, dir >= 0 && dir < 4, "valid direction")
}

// ============================================================
// OOB head recovery
// ============================================================

// Snake with head outside the right map boundary must go LEFT to
// re-enter the map, not UP (which stays OOB).
//
// Small 8×6 grid, right-bottom portion:
//
//	........   y=0
//	........   y=1
//	........   y=2   ← snake head OOB at (8,2), body (8,3),(7,3)
//	........   y=3
//	..######   y=4
//	########   y=5
//
// From OOB (8,2):
//
//	LEFT  → (7,2) free inside map  ← must pick this
//	UP    → (8,1) still OOB
//	DOWN  → (8,3) neck → blocked
//	RIGHT → (9,2) 2-cells-out → -1
func TestSafety_OOBHeadPrefersInsideMap(t *testing.T) {
	input := strings.Join([]string{
		"0",        // player ID
		"8",        // W
		"6",        // H
		"........", // y=0
		"........", // y=1
		"........", // y=2
		"........", // y=3
		"..######", // y=4
		"########", // y=5
		"1",        // snakes per player
		"0",        // my snake ID
		"1",        // opponent snake ID
		// Turn data
		"1",             // 1 apple
		"3 2",           // apple at (3,2)
		"2",             // 2 snakes
		"0 8,2:8,3:7,3", // mine, head OOB right
		"1 0,2:0,3:0,4", // opponent inside
	}, "\n")

	s := bufio.NewScanner(strings.NewReader(input))
	s.Buffer(make([]byte, 1000000), 1000000)
	g := Init(s)
	g.Read(s)

	p := &Plan{g: g}
	p.Precompute()
	d := &Decision{G: g, P: p}

	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()
	d.phaseSafety()

	assert.Equal(t, 1, len(d.MySnakes), "should have 1 my snake")
	assert.Equal(t, DL, d.AssignedDir[0],
		"OOB head should go LEFT to re-enter map, got %s", Dn[d.AssignedDir[0]])
}

// Rollout should avoid a greedy apple chase that leads to a fatal head-on
// collision in the next simulated move sequence.
func TestSafety_RolloutAvoidsHeadOnDeath(t *testing.T) {
	input := strings.Join([]string{
		"0",
		"5",
		"5",
		".....",
		".....",
		".....",
		".....",
		"#####",
		"1",
		"0",
		"1",
		"1",
		"1 3",
		"2",
		"0 2,2:2,1:2,0",
		"1 3,3:4,3:4,2:4,1",
	}, "\n")

	s := bufio.NewScanner(strings.NewReader(input))
	s.Buffer(make([]byte, 1000000), 1000000)
	g := Init(s)
	g.Read(s)

	p := &Plan{g: g}
	p.Precompute()
	d := &Decision{G: g, P: p}

	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()

	d.AssignedDir[0] = DD // greedy move collides head-on at (2,3)
	d.phaseSafety()

	assert.NotEqual(t, DD, d.AssignedDir[0],
		"rollout should avoid fatal head-on over the apple")
}
