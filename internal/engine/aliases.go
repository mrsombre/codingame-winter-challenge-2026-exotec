package engine

import (
	actionpkg "codingame/internal/engine/action"
	gridpkg "codingame/internal/engine/grid"
)

type Rng = gridpkg.Rng

type Coord = gridpkg.Coord
type Direction = gridpkg.Direction
type Grid = gridpkg.Grid
type GridMaker = gridpkg.GridMaker
type Tile = gridpkg.Tile
type TileType = gridpkg.TileType

type Action = actionpkg.Action
type ActionType = actionpkg.ActionType

const (
	DirNorth = gridpkg.DirNorth
	DirEast  = gridpkg.DirEast
	DirSouth = gridpkg.DirSouth
	DirWest  = gridpkg.DirWest
	DirUnset = gridpkg.DirUnset

	TileEmpty = gridpkg.TileEmpty
	TileWall  = gridpkg.TileWall

	ActionTypeMoveUp    = actionpkg.ActionTypeMoveUp
	ActionTypeMoveDown  = actionpkg.ActionTypeMoveDown
	ActionTypeMoveLeft  = actionpkg.ActionTypeMoveLeft
	ActionTypeMoveRight = actionpkg.ActionTypeMoveRight
	ActionTypeMark      = actionpkg.ActionTypeMark
	ActionTypeWait      = actionpkg.ActionTypeWait
)

func NewGrid(width, height int) *Grid {
	return gridpkg.NewGrid(width, height)
}

func NewGridMaker(random Rng, leagueLevel int) *GridMaker {
	return gridpkg.NewGridMaker(random, leagueLevel)
}

func NewSHA1PRNG(seed int64) Rng {
	return gridpkg.NewSHA1PRNG(seed)
}

func DirectionFromCoord(c Coord) Direction {
	return gridpkg.DirectionFromCoord(c)
}

func DirectionFromAlias(alias string) (Direction, error) {
	return gridpkg.DirectionFromAlias(alias)
}

func DirectionFromName(name string) (Direction, bool) {
	return gridpkg.DirectionFromName(name)
}

func ParseAction(command string) (*Action, error) {
	return actionpkg.ParseAction(command)
}

func sortedCoords(coords map[Coord]bool) []Coord {
	result := make([]Coord, 0, len(coords))
	for c := range coords {
		result = append(result, c)
	}
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Less(result[i]) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}
