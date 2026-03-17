package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// Phase 4: Assignment with influence scoring
// ============================================================

// All 3 snakes should be assigned (30 apples available).
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
// With 30 apples and 3 snakes, at least 2 of 3 should target my territory.
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

// Verify BFS distance and influence values that feed into Phase 3 scoring.
// Apple (9,9): BFS dist=8 from Snake 0 (head 16,10), influence=-2 (enemy territory).
func TestAssignment_ScoringInputs(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()

	cell := g.Idx(9, 9)
	assert.Equal(t, 8, d.BFS[0][cell].Dist, "Snake 0 raw dist to (9,9)")
	assert.Equal(t, -2, d.Influence[cell], "(9,9) influence")
}
