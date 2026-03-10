// Package grid
// Source: source/src/main/java/com/codingame/game/grid/Coord.java
package grid

import "fmt"

type Coord struct {
	X int
	Y int
}

func (c Coord) Add(other Coord) Coord {
	return Coord{X: c.X + other.X, Y: c.Y + other.Y}
}

func (c Coord) AddXY(x, y int) Coord {
	return Coord{X: c.X + x, Y: c.Y + y}
}

func (c Coord) ManhattanTo(other Coord) int {
	dx := c.X - other.X
	if dx < 0 {
		dx = -dx
	}
	dy := c.Y - other.Y
	if dy < 0 {
		dy = -dy
	}
	return dx + dy
}

func (c Coord) String() string {
	return fmt.Sprintf("(%d, %d)", c.X, c.Y)
}

func (c Coord) IntString() string {
	return fmt.Sprintf("%d %d", c.X, c.Y)
}

func (c Coord) Less(other Coord) bool {
	if c.X != other.X {
		return c.X < other.X
	}
	return c.Y < other.Y
}
