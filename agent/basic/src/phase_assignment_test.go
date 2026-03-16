package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
