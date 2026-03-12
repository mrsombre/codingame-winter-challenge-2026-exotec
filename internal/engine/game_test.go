package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitClearsEnclosedSpawnHeadWalls(t *testing.T) {
	grid := NewGrid(7, 5)
	grid.Spawns = []Coord{
		{X: 2, Y: 1},
		{X: 2, Y: 2},
		{X: 2, Y: 3},
	}
	grid.GetXY(1, 1).Type = TileWall
	grid.GetXY(3, 1).Type = TileWall
	grid.GetXY(5, 1).Type = TileWall

	game := &Game{Grid: grid}
	p0 := NewPlayer(0)
	p1 := NewPlayer(1)

	game.Init([]*Player{p0, p1})

	require.Len(t, p0.GetBirds(), 1)
	require.Len(t, p1.GetBirds(), 1)
	assert.Equal(t, TileEmpty, game.Grid.GetXY(1, 1).Type)
	assert.Equal(t, TileWall, game.Grid.GetXY(3, 1).Type)
	assert.Equal(t, TileEmpty, game.Grid.GetXY(5, 1).Type)
	assert.Equal(t, Coord{X: 2, Y: 1}, p0.GetBirds()[0].HeadPos())
	assert.Equal(t, Coord{X: 4, Y: 1}, p1.GetBirds()[0].HeadPos())
}
