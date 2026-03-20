package main

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDecision(t *testing.T, initInput string, turnInput string) *Decision {
	t.Helper()
	s := bufio.NewScanner(strings.NewReader(initInput + turnInput))
	s.Buffer(make([]byte, 1024), 1024)
	g := Init(s)
	require.True(t, g.Turn(s))
	d := NewDecision(g)
	d.rebuildTurnMaps()
	return d
}

func TestSimulateCandidateRejectsReverse(t *testing.T) {
	d := newTestDecision(t, ""+
		"0\n"+
		"5\n"+
		"5\n"+
		".....\n"+
		".....\n"+
		".....\n"+
		".....\n"+
		"#####\n"+
		"1\n"+
		"1\n"+
		"2\n",
		""+
			"0\n"+
			"2\n"+
			"1 2,2:2,3:2,4\n"+
			"2 4,2:4,3:4,4\n",
	)
	cand := d.simulateCandidate(0, DD)
	assert.False(t, cand.Legal)
}

func TestSimulateCandidateRejectsWallAndFallDeath(t *testing.T) {
	d := newTestDecision(t, ""+
		"0\n"+
		"4\n"+
		"4\n"+
		".#..\n"+
		"....\n"+
		"....\n"+
		"....\n"+
		"1\n"+
		"1\n"+
		"2\n",
		""+
			"0\n"+
			"2\n"+
			"1 0,0:0,1:0,2\n"+
			"2 3,0:3,1:3,2\n",
	)
	assert.False(t, d.simulateCandidate(0, DR).Legal)
	assert.False(t, d.simulateCandidate(0, DU).Legal)
}

func TestSimulateCandidateEatsApple(t *testing.T) {
	d := newTestDecision(t, ""+
		"0\n"+
		"5\n"+
		"5\n"+
		".....\n"+
		".....\n"+
		".....\n"+
		".....\n"+
		"#####\n"+
		"1\n"+
		"1\n"+
		"2\n",
		""+
			"1\n"+
			"3 1\n"+
			"2\n"+
			"1 2,1:2,2:2,3\n"+
			"2 4,1:4,2:4,3\n",
	)
	cand := d.simulateCandidate(0, DR)
	require.True(t, cand.Legal)
	assert.True(t, cand.Eating)
	assert.Len(t, cand.Body, 4)
}

func TestResolveFriendlyConflictsSplitsHeads(t *testing.T) {
	d := newTestDecision(t, ""+
		"0\n"+
		"7\n"+
		"5\n"+
		".......\n"+
		".......\n"+
		".......\n"+
		".......\n"+
		"#######\n"+
		"2\n"+
		"1\n"+
		"2\n"+
		"3\n"+
		"4\n",
		""+
			"0\n"+
			"4\n"+
			"1 2,1:2,2:2,3\n"+
			"2 4,1:4,2:4,3\n"+
			"3 0,1:0,2:0,3\n"+
			"4 6,1:6,2:6,3\n",
	)

	d.rebuildTurnMaps()
	d.collectMySnakes()
	d.AssignedDir = []int{DR, DL}
	for i, snIdx := range d.MySnakes {
		for dir := 0; dir < 4; dir++ {
			d.Candidates[i][dir] = d.simulateCandidate(snIdx, dir)
			d.Candidates[i][dir].Score = float32(dir)
		}
	}
	d.resolveFriendlyConflicts()
	assert.NotEqual(t, d.Candidates[0][d.AssignedDir[0]].Head, d.Candidates[1][d.AssignedDir[1]].Head)
}
