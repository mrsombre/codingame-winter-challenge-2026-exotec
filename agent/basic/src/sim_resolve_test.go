package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Shared 8x7 grid for resolve tests:
//
//	   01234567
//	0  ........
//	1  ##..##..
//	2  ........
//	3  ........
//	4  ........
//	5  ........
//	6  ########
//
// Walls at y=1: (0,1)(1,1) and (4,1)(5,1). Ground: full row y=6.
// Surfaces at y=0: left (0,0)-(1,0), right (4,0)-(5,0).
func resolveGrid() (*Game, *Sim) {
	g := testGridInput([]string{
		"........",
		"##..##..",
		"........",
		"........",
		"........",
		"........",
		"########",
	})
	s := NewSim(g)
	s.RebuildAppleMap()
	return g, s
}

func TestResolveGapCrossingNoFall(t *testing.T) {
	// Snake moved RIGHT across gap — head on right surface, tail over gap
	// Head is grounded → whole snake stays, no fall
	//
	//   01234567
	//   ..th..      row 0: tail(3,0) over gap, head(4,0) on surface
	//   ##..##..    row 1: walls
	//
	// head at (4,0): below (4,1) is wall → grounded!
	// neck at (3,0): below (3,1) is free — hanging
	// tail at (2,0): below (2,1) is free — hanging
	// snake stays because head is supported
	g, s := resolveGrid()

	snakes := []Snake{{
		ID: 0, Owner: 0, Alive: true,
		Body: []int{g.Idx(4, 0), g.Idx(3, 0), g.Idx(2, 0)},
		Len:  3,
	}}

	s.resolveMove(snakes)

	assert.True(t, snakes[0].Alive)
	assert.Equal(t, g.Idx(4, 0), snakes[0].Body[0], "head stays on surface")
	assert.Equal(t, g.Idx(3, 0), snakes[0].Body[1], "neck stays over gap")
	assert.Equal(t, g.Idx(2, 0), snakes[0].Body[2], "tail stays over gap")
}

func TestResolveFallToFloor(t *testing.T) {
	// Snake moved DOWN off block edge — no segment grounded, falls to floor
	//
	// Snake was on left surface edge, moved DOWN. Now fully in air:
	//   (2,2)(2,1)(2,0)  — vertical, head at y=2
	//
	// (2,2) below (2,3) free
	// (2,1) below (2,2) free (head cell, not wall)
	// (2,0) below (2,1) free
	// → all airborne, falls until tail reaches y=5 (above wall y=6)
	// result: (2,5)(2,4)(2,3)
	g, s := resolveGrid()

	snakes := []Snake{{
		ID: 0, Owner: 0, Alive: true,
		Body: []int{g.Idx(2, 2), g.Idx(2, 1), g.Idx(2, 0)},
		Len:  3,
	}}

	s.resolveMove(snakes)

	assert.True(t, snakes[0].Alive)
	assert.Equal(t, g.Idx(2, 5), snakes[0].Body[0], "head fell to y=5")
	assert.Equal(t, g.Idx(2, 4), snakes[0].Body[1], "neck at y=4")
	assert.Equal(t, g.Idx(2, 3), snakes[0].Body[2], "tail at y=3")
}
