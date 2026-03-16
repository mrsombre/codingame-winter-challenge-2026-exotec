package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// Phase 2: Influence (Voronoi territory)
// ============================================================

// Map layout (snakes + key apples):
//
//   .....5................   y=0  Snake 5 (enemy, 5,0)  |  Snake 2 (mine, 16,0)
//   ..a..........a..a.a..   y=1  apples: (2,1) (14,1) (7,1)? (19,1)
//   ......................   y=2
//   ..a.##...####...##.a.   y=3  apples: (2,3) (19,3)
//   #....................#   y=4
//   ##..4##..#..#.1##...##   y=5  Snake 4 (enemy, 7,5) | Snake 1 (mine, 14,5)
//   ##.a...a.#..#.a....a##   y=6  apples: (2,6) (6,6) (15,6) (19,6)
//   ####..#........#..####   y=7
//   ...0..##......##..3...   y=8  Snake 0 (mine, 3,8) | Snake 3 (enemy, 18,8)
//   .#...a....##.a......#.   y=9  apples: (9,9) (12,9)
//   ##..##############..##   y=10
//   ######################   y=11

// Apple (9,9): Enemy Snake 4 (head 7,5) falls down fast and reaches
// (9,9) in ~3 steps. My Snake 0 (3,8) takes 6 steps along row 9.
// Gravity makes this enemy territory despite horizontal proximity.
// → negative influence (enemy territory).
func TestInfluence_Apple_9_9_EnemyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(9, 9)
	inf := d.Influence[cell]
	assert.Equal(t, -3, inf, "(9,9) influence")
}

// Apple (12,9): My Snake 1 (head 14,5) falls down and reaches
// (12,9) faster than any enemy snake.
// → positive influence (my territory).
func TestInfluence_Apple_12_9_MyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(12, 9)
	inf := d.Influence[cell]
	assert.Equal(t, 3, inf, "(12,9) influence")
}

// Apple (19,1): Snake 2 (mine, 16,0) is 3 steps right.
// No enemy snake is nearby.
// → strong positive influence.
func TestInfluence_Apple_19_1_MyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(19, 1)
	inf := d.Influence[cell]
	assert.True(t, inf > 0, "(19,1) influence=%d, want positive", inf)
}

// Apple (2,1): Snake 5 (enemy, 5,0) is ~3 steps left.
// My closest snake (Snake 2 at 16,0) is far.
// → negative influence (enemy territory).
func TestInfluence_Apple_2_1_EnemyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(2, 1)
	inf := d.Influence[cell]
	assert.True(t, inf < 0, "(2,1) influence=%d, want negative (enemy territory)", inf)
}

// Apple (15,6): Snake 1 (mine, 14,5) is ~1-2 steps.
// → positive influence (my territory).
func TestInfluence_Apple_15_6_MyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(15, 6)
	inf := d.Influence[cell]
	assert.True(t, inf > 0, "(15,6) influence=%d, want positive", inf)
}

// Apple (6,6): Snake 4 (enemy, 7,5) is ~1-2 steps.
// → negative influence (enemy territory).
func TestInfluence_Apple_6_6_EnemyTerritory(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(6, 6)
	inf := d.Influence[cell]
	assert.True(t, inf < 0, "(6,6) influence=%d, want negative (enemy territory)", inf)
}

// Row 9 apple pair: (9,9) and (12,9) have opposite influence signs.
// Due to gravity, enemy Snake 4 (7,5) dominates (9,9) via downfall,
// while my Snake 1 (14,5) dominates (12,9) symmetrically.
func TestInfluence_SymmetricPair(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	left := d.Influence[g.Idx(9, 9)]
	right := d.Influence[g.Idx(12, 9)]
	assert.True(t, left < 0 && right > 0,
		"(9,9) inf=%d and (12,9) inf=%d should have opposite signs", left, right)
}
