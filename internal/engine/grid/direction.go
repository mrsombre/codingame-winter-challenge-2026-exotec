// Package grid
// Source: source/src/main/java/com/codingame/game/grid/Direction.java
package grid

import "fmt"

type Direction int

const (
	DirNorth Direction = iota
	DirEast
	DirSouth
	DirWest
	DirUnset
)

var dirCoords = map[Direction]Coord{
	DirNorth: {0, -1},
	DirEast:  {1, 0},
	DirSouth: {0, 1},
	DirWest:  {-1, 0},
	DirUnset: {0, 0},
}

var dirAliases = map[Direction]string{
	DirNorth: "N",
	DirEast:  "E",
	DirSouth: "S",
	DirWest:  "W",
	DirUnset: "X",
}

func (d Direction) Coord() Coord {
	return dirCoords[d]
}

func (d Direction) String() string {
	if alias, ok := dirAliases[d]; ok {
		return alias
	}
	return dirAliases[DirUnset]
}

func (d Direction) Opposite() Direction {
	switch d {
	case DirNorth:
		return DirSouth
	case DirEast:
		return DirWest
	case DirSouth:
		return DirNorth
	case DirWest:
		return DirEast
	default:
		return DirUnset
	}
}

func DirectionFromCoord(c Coord) Direction {
	for dir, dc := range dirCoords {
		if dc == c {
			return dir
		}
	}
	return DirUnset
}

func DirectionFromAlias(alias string) (Direction, error) {
	switch alias {
	case "N":
		return DirNorth, nil
	case "E":
		return DirEast, nil
	case "S":
		return DirSouth, nil
	case "W":
		return DirWest, nil
	default:
		return DirUnset, fmt.Errorf("%s is not a direction alias", alias)
	}
}

func DirectionFromName(name string) (Direction, bool) {
	switch name {
	case "UP":
		return DirNorth, true
	case "DOWN":
		return DirSouth, true
	case "LEFT":
		return DirWest, true
	case "RIGHT":
		return DirEast, true
	default:
		return DirUnset, false
	}
}
