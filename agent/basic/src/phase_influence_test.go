package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// Phase 2: Influence (Voronoi territory)
// ============================================================

// Map layout (snakes + key apples, new 28x15 grid):
//
//   Snake positions (turn 0):
//     My  Snake 0 head=(16,10), Snake 1 head=(0,6),  Snake 2 head=(5,6)
//     Op  Snake 3 head=(11,10), Snake 4 head=(27,6), Snake 5 head=(22,6)
//
//   Key apples:
//     (9,9)  ap[24]: opBest=4 (Snake3), myBest=6 (Snake2) → enemy territory
//     (18,9) ap[25]: myBest=4 (Snake0), opBest=6 (Snake5) → my territory
//     (8,1)  ap[18]: myBest=11, opBest=19 → my territory (closer mine)
//     (19,1) ap[19]: opBest=13, myBest=15 → enemy territory (closer enemy)
//     (7,6)  ap[12]: myBest=3 (Snake2), opBest=9 (Snake3) → my territory
//     (20,6) ap[13]: opBest=3 (Snake5), myBest=9 (Snake0) → enemy territory

// Apple (9,9): Op Snake 3 (head 11,10) reaches it in 4 steps.
// My Snake 2 (5,6) takes 6 steps.
// → negative influence (enemy territory).
func TestInfluence_Apple_9_9_EnemyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(9, 9)
	inf := d.Influence[cell]
	assert.Equal(t, -2, inf, "(9,9) influence")
}

// Apple (18,9): My Snake 0 (head 16,10) reaches it in 4 steps.
// Op Snake 5 (22,6) takes 6 steps.
// → positive influence (my territory).
func TestInfluence_Apple_18_9_MyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(18, 9)
	inf := d.Influence[cell]
	assert.Equal(t, 2, inf, "(18,9) influence")
}

// Apple (8,1): My Snake 2 (5,6) is 11 steps away; nearest enemy is 19 steps.
// → strong positive influence (my territory).
func TestInfluence_Apple_8_1_MyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(8, 1)
	inf := d.Influence[cell]
	assert.True(t, inf > 0, "(8,1) influence=%d, want positive", inf)
}

// Apple (19,1): Op Snake 5 (22,6) reaches in 13 steps.
// My Snake 0 (16,10) is 15 steps away.
// → negative influence (enemy territory).
func TestInfluence_Apple_19_1_EnemyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(19, 1)
	inf := d.Influence[cell]
	assert.True(t, inf < 0, "(19,1) influence=%d, want negative (enemy territory)", inf)
}

// Apple (7,6): My Snake 2 (head 5,6) is only 3 steps away.
// Nearest enemy (Snake3 at 11,10) is 9 steps.
// → positive influence (my territory).
func TestInfluence_Apple_7_6_MyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(7, 6)
	inf := d.Influence[cell]
	assert.True(t, inf > 0, "(7,6) influence=%d, want positive", inf)
}

// Apple (20,6): Op Snake 5 (head 22,6) is only 3 steps away.
// My Snake 0 (16,10) is 9 steps.
// → negative influence (enemy territory).
func TestInfluence_Apple_20_6_EnemyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(20, 6)
	inf := d.Influence[cell]
	assert.True(t, inf < 0, "(20,6) influence=%d, want negative (enemy territory)", inf)
}

// Row 9 apple pair: (9,9) and (18,9) have opposite influence signs.
// Op Snake 3 (11,10) dominates (9,9); My Snake 0 (16,10) dominates (18,9) symmetrically.
func TestInfluence_SymmetricPair(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	left := d.Influence[g.Idx(9, 9)]
	right := d.Influence[g.Idx(18, 9)]
	assert.True(t, left < 0 && right > 0,
		"(9,9) inf=%d and (18,9) inf=%d should have opposite signs", left, right)
}
