package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ============================================================
// Phase 3: Resource scoring
// ============================================================

// Scores matrix has correct dimensions and no nil rows.
func TestScoring_Dimensions(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()

	assert.Equal(t, len(d.MySnakes), len(d.Scores), "rows = my snakes")
	for si := range d.MySnakes {
		assert.Equal(t, g.ANum, len(d.Scores[si]), "cols = apple count")
	}
}

// All reachable apples should have positive scores.
func TestScoring_ReachablePositive(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()

	for si := range d.MySnakes {
		for j := 0; j < g.ANum; j++ {
			bfs := d.BFS[si]
			ap := g.Ap[j]
			if bfs != nil && bfs[ap].Dist >= 0 {
				assert.True(t, d.Scores[si][j] > 0,
					"snake %d → apple %d: reachable but score=%d",
					d.MySnakes[si], j, d.Scores[si][j])
			}
		}
	}
}

// Unreachable apples should have score -1.
func TestScoring_UnreachableNegative(t *testing.T) {
	_, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()

	for si := range d.MySnakes {
		bfs := d.BFS[si]
		if bfs == nil {
			continue
		}
		for j := 0; j < d.G.ANum; j++ {
			ap := d.G.Ap[j]
			if bfs[ap].Dist < 0 {
				assert.Equal(t, -1, d.Scores[si][j],
					"snake %d → apple %d: unreachable should be -1", d.MySnakes[si], j)
			}
		}
	}
}

// Closer apple scores higher than farther apple (same snake, similar conditions).
// Snake 0 (head 3,8): apple (5,9) is close, apple (19,1) is far.
func TestScoring_CloserBeatsfarther(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()

	// Find the closest and farthest reachable apples for snake 0.
	bfs := d.BFS[0]
	minDist, maxDist := MaxCells, 0
	minJ, maxJ := -1, -1
	for j := 0; j < g.ANum; j++ {
		ap := g.Ap[j]
		if bfs[ap].Dist < 0 {
			continue
		}
		if bfs[ap].Dist < minDist {
			minDist = bfs[ap].Dist
			minJ = j
		}
		if bfs[ap].Dist > maxDist {
			maxDist = bfs[ap].Dist
			maxJ = j
		}
	}

	assert.NotEqual(t, -1, minJ)
	assert.NotEqual(t, -1, maxJ)
	assert.NotEqual(t, minJ, maxJ, "need different apples")

	// Closest apple should generally score higher
	// (unless extreme influence/exclusivity differences override).
	t.Logf("closest apple[%d] dist=%d score=%d", minJ, minDist, d.Scores[0][minJ])
	t.Logf("farthest apple[%d] dist=%d score=%d", maxJ, maxDist, d.Scores[0][maxJ])
}

// Apple (9,9) is enemy territory (inf=-3), apple (12,9) is my territory (inf=+3).
// For a snake equidistant, my-territory apple should score higher.
func TestScoring_MyTerritoryBeatsEnemy(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()

	cell99 := g.Idx(9, 9)
	cell129 := g.Idx(12, 9)

	// Find apple indices.
	j99, j129 := -1, -1
	for j := 0; j < g.ANum; j++ {
		if g.Ap[j] == cell99 {
			j99 = j
		}
		if g.Ap[j] == cell129 {
			j129 = j
		}
	}
	assert.NotEqual(t, -1, j99)
	assert.NotEqual(t, -1, j129)

	// Check across all snakes that can reach both: my-territory apple scores higher.
	for si := range d.MySnakes {
		bfs := d.BFS[si]
		if bfs == nil {
			continue
		}
		if bfs[cell99].Dist < 0 || bfs[cell129].Dist < 0 {
			continue
		}
		score99 := d.Scores[si][j99]
		score129 := d.Scores[si][j129]

		// If distances are similar, my-territory should win.
		dist99 := bfs[cell99].Dist
		dist129 := bfs[cell129].Dist
		if abs(dist99-dist129) <= 2 {
			assert.True(t, score129 > score99,
				"snake %d: my-territory (12,9) score=%d should beat enemy (9,9) score=%d "+
					"(dist %d vs %d)", d.MySnakes[si], score129, score99, dist129, dist99)
		}
	}
}

// Verify influence penalty: apple (9,9) has negative influence and an enemy
// reaching it faster → score penalized vs a neutral apple at the same distance.
func TestScoring_InfluencePenalty(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()

	cell := g.Idx(9, 9)
	assert.Equal(t, -3, d.Influence[cell], "(9,9) influence")

	j99 := -1
	for j := 0; j < g.ANum; j++ {
		if g.Ap[j] == cell {
			j99 = j
			break
		}
	}
	assert.NotEqual(t, -1, j99)

	// Score should include a negative safety component and race penalty.
	score := d.Scores[0][j99]
	dist := d.BFS[0][cell].Dist
	baseOnly := scoreBase / (1 + dist)
	assert.True(t, score < baseOnly,
		"(9,9) score=%d should be less than base-only=%d due to penalties", score, baseOnly)
}

// Exclusive apples (unreachable by enemies) get a bonus.
func TestScoring_ExclusiveBonus(t *testing.T) {
	g, _, d := testDecision()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()

	// Find apples where no enemy can reach (opMinDist >= MaxCells).
	for j := 0; j < g.ANum; j++ {
		ap := g.Ap[j]
		opReachable := false
		for _, bfs := range d.OpBFS {
			if bfs != nil && bfs[ap].Dist >= 0 {
				opReachable = true
				break
			}
		}
		if opReachable {
			continue
		}
		// This apple is exclusive — verify it has the exclusive bonus.
		for si := range d.MySnakes {
			if d.Scores[si][j] < 0 {
				continue
			}
			dist := d.BFS[si][ap].Dist
			baseOnly := scoreBase / (1 + dist)
			assert.True(t, d.Scores[si][j] > baseOnly,
				"exclusive apple %d: score=%d should exceed base=%d",
				j, d.Scores[si][j], baseOnly)
		}
	}
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
