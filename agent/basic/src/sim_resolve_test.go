package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveMove_HeadHeadCollisionBeheadsBoth(t *testing.T) {
	g, p := testMovePlan([]string{
		".....",
		".....",
		".....",
		".....",
		"#####",
	}, nil)

	snakes := []Snake{
		{
			ID:    0,
			Alive: true,
			Body: []int{
				g.Idx(2, 3),
				g.Idx(1, 3),
				g.Idx(0, 3),
				g.Idx(0, 2),
			},
			Len: 4,
		},
		{
			ID:    1,
			Alive: true,
			Body: []int{
				g.Idx(2, 3),
				g.Idx(3, 3),
				g.Idx(4, 3),
				g.Idx(4, 2),
			},
			Len: 4,
		},
	}

	p.resolveMove(snakes)

	require.True(t, snakes[0].Alive)
	require.True(t, snakes[1].Alive)
	assert.Equal(t, []int{g.Idx(1, 3), g.Idx(0, 3), g.Idx(0, 2)}, snakes[0].Body)
	assert.Equal(t, []int{g.Idx(3, 3), g.Idx(4, 3), g.Idx(4, 2)}, snakes[1].Body)
}

func TestResolveMove_LengthThreeDiesOnBodyCollision(t *testing.T) {
	g, p := testMovePlan([]string{
		".....",
		".....",
		".....",
		".....",
		"#####",
	}, nil)

	snakes := []Snake{
		{
			ID:    0,
			Alive: true,
			Body:  []int{g.Idx(2, 2), g.Idx(1, 2), g.Idx(0, 2)},
			Len:   3,
		},
		{
			ID:    1,
			Alive: true,
			Body:  []int{g.Idx(4, 3), g.Idx(3, 3), g.Idx(2, 2)},
			Len:   3,
		},
	}

	p.resolveMove(snakes)

	assert.False(t, snakes[0].Alive)
	assert.Nil(t, snakes[0].Body)
	require.True(t, snakes[1].Alive)
	assert.Equal(t, []int{g.Idx(4, 3), g.Idx(3, 3), g.Idx(2, 2)}, snakes[1].Body)
}

func TestResolveMove_LowerSnakeGroundsUpperAfterFall(t *testing.T) {
	g, p := testMovePlan([]string{
		"...",
		"...",
		"...",
		"...",
		"...",
		"...",
		"###",
	}, nil)

	snakes := []Snake{
		{
			ID:    0,
			Alive: true,
			Body:  []int{g.Idx(1, 3)},
			Len:   1,
		},
		{
			ID:    1,
			Alive: true,
			Body:  []int{g.Idx(1, 1)},
			Len:   1,
		},
	}

	p.resolveMove(snakes)

	require.True(t, snakes[0].Alive)
	require.True(t, snakes[1].Alive)
	assert.Equal(t, []int{g.Idx(1, 5)}, snakes[0].Body)
	assert.Equal(t, []int{g.Idx(1, 4)}, snakes[1].Body)
}
