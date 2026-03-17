package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimulateMove_WallKillsLengthThree(t *testing.T) {
	g, p := testMovePlan([]string{
		".#.",
		"...",
		"...",
	}, nil)

	body := []int{
		g.Idx(0, 0),
		g.Idx(0, 1),
		g.Idx(0, 2),
	}

	got, alive := p.simulateMove(body, DR)
	assert.False(t, alive)
	assert.Nil(t, got)
}

func TestSimulateMove_WallBeheadsLengthFour(t *testing.T) {
	g, p := testMovePlan([]string{
		".#.",
		"...",
		"...",
		"...",
	}, nil)

	body := []int{
		g.Idx(0, 0),
		g.Idx(0, 1),
		g.Idx(0, 2),
		g.Idx(0, 3),
	}

	got, alive := p.simulateMove(body, DR)
	require.True(t, alive)
	assert.Equal(t, []int{
		g.Idx(0, 0),
		g.Idx(0, 1),
		g.Idx(0, 2),
	}, append([]int(nil), got...))
}

func TestSimulateMove_EnteringOldTailIsSafe(t *testing.T) {
	g, p := testMovePlan([]string{
		"...",
		"...",
		"...",
	}, nil)

	body := []int{
		g.Idx(1, 1),
		g.Idx(1, 2),
		g.Idx(0, 2),
		g.Idx(0, 1),
	}

	got, alive := p.simulateMove(body, DL)
	require.True(t, alive)
	assert.Equal(t, []int{
		g.Idx(0, 1),
		g.Idx(1, 1),
		g.Idx(1, 2),
		g.Idx(0, 2),
	}, append([]int(nil), got...))
}

func TestSimulateMove_ApplePreservesTailBeforeBodyCollision(t *testing.T) {
	g, p := testMovePlan([]string{
		"...",
		"...",
		"...",
	}, [][2]int{{0, 1}})

	body := []int{
		g.Idx(1, 1),
		g.Idx(1, 2),
		g.Idx(0, 2),
		g.Idx(0, 1),
	}

	got, alive := p.simulateMove(body, DL)
	require.True(t, alive)
	assert.Equal(t, body, append([]int(nil), got...))
}
