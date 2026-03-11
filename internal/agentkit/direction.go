package agentkit

// Direction matches the arena move set plus a sentinel "no move" value.
type Direction int

const (
	DirUp Direction = iota
	DirRight
	DirDown
	DirLeft
	DirNone
)

var DirectionNames = [4]string{"UP", "RIGHT", "DOWN", "LEFT"}

var DirectionDeltas = [5]Point{
	{X: 0, Y: -1},
	{X: 1, Y: 0},
	{X: 0, Y: 1},
	{X: -1, Y: 0},
	{X: 0, Y: 0},
}

func Opposite(dir Direction) Direction {
	switch dir {
	case DirUp:
		return DirDown
	case DirRight:
		return DirLeft
	case DirDown:
		return DirUp
	case DirLeft:
		return DirRight
	default:
		return DirNone
	}
}

func FacingFromPoints(head, neck Point) Direction {
	dx := head.X - neck.X
	dy := head.Y - neck.Y

	switch {
	case dx == 0 && dy == -1:
		return DirUp
	case dx == 1 && dy == 0:
		return DirRight
	case dx == 0 && dy == 1:
		return DirDown
	case dx == -1 && dy == 0:
		return DirLeft
	default:
		return DirNone
	}
}
