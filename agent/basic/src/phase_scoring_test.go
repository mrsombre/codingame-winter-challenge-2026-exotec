package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPhaseScoringPrefersImmediateApple(t *testing.T) {
	g := testBFSGame()
	g.Ap = []int{g.Idx(0, 3), g.Idx(6, 3)}
	g.ANum = len(g.Ap)
	g.BuildSurfaceGraph()

	g.SNum = 1
	g.Sn[0] = Snake{
		ID: 0, Owner: 0, Alive: true,
		Body: []int{g.Idx(1, 3), g.Idx(2, 3), g.Idx(3, 3)},
		Len:  3,
	}

	d := &Decision{G: g, P: &Plan{G: g}}
	d.phaseBFS()
	d.phaseScoring()

	assert.Equal(t, DL, d.AssignedDir[0], "greedy scoring should take the immediate apple")
	assert.Equal(t, g.Idx(0, 3), d.Assigned[0])
}
