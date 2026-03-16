package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func testDecision() (*Game, *Plan, *Decision) {
	g := testGameFull()
	p := &Plan{g: g}
	p.Precompute()
	return g, p, &Decision{G: g, P: p}
}

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

// Snake 0 (mine, head 3,8) should still reach (9,9) in 6 steps
// after running BFS for all snakes (not just mine).
func TestPhaseBFS_Snake0_Dist(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()

	// Snake 0 is first in MySnakes
	bfs := d.BFS[0]
	target := g.Idx(9, 9)
	assert.Equal(t, 6, bfs[target].Dist, "Snake 0 → (9,9) dist")
	assert.Equal(t, DR, bfs[target].FirstDir, "Snake 0 → (9,9) dir")
}

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

// ============================================================
// Phase 4: Assignment with influence scoring
// ============================================================

// All 3 snakes should be assigned (12 apples available).
// No duplicate assignments. All directions valid.
func TestAssignment_Valid(t *testing.T) {
	_, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()

	for si := range d.MySnakes {
		assert.NotEqual(t, -1, d.Assigned[si], "snake %d unassigned", si)
	}

	// No duplicate apple assignments
	seen := make(map[int]bool)
	for _, ap := range d.Assigned {
		if ap != -1 {
			assert.False(t, seen[ap], "apple %d assigned twice", ap)
			seen[ap] = true
		}
	}

	// All directions are valid
	for si := range d.MySnakes {
		dir := d.AssignedDir[si]
		assert.True(t, dir >= 0 && dir < 4, "invalid dir %d for snake %d", dir, si)
	}
}

// Assigned apples should prefer my territory (positive influence).
// With 12 apples and 3 snakes, at least 2 of 3 should target my territory.
func TestAssignment_PrefersMyTerritory(t *testing.T) {
	_, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()

	myTerritory := 0
	for _, ap := range d.Assigned {
		if ap >= 0 && d.Influence[ap] > 0 {
			myTerritory++
		}
	}
	assert.True(t, myTerritory >= 2,
		"only %d/%d assigned to my territory, want ≥2", myTerritory, len(d.Assigned))
}

// Verify influence penalty is applied to scoring.
// Apple (9,9): raw BFS dist=6, influence=-3 → score=6+6=12 (penalized).
// Without penalty the score would be 6. The penalty doubles the enemy lead.
func TestAssignment_InfluencePenalty(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(9, 9)
	rawDist := d.BFS[0][cell].Dist
	inf := d.Influence[cell]

	assert.Equal(t, 6, rawDist, "Snake 0 raw dist to (9,9)")
	assert.Equal(t, -3, inf, "(9,9) influence")

	// Effective score with penalty: dist + abs(inf)*2
	expectedScore := rawDist + (-inf) * 2
	assert.Equal(t, 12, expectedScore, "penalized score for (9,9)")
}
