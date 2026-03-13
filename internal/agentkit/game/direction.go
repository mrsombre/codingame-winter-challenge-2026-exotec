package game

type Direction int

const (
	DirUp Direction = iota
	DirRight
	DirDown
	DirLeft
	DirNone
)

var DirName = [4]string{"UP", "RIGHT", "DOWN", "LEFT"}

var DirDelta = [5]Point{
	{X: 0, Y: -1},
	{X: 1, Y: 0},
	{X: 0, Y: 1},
	{X: -1, Y: 0},
	{X: 0, Y: 0},
}

var oppDir = [5]Direction{DirDown, DirLeft, DirUp, DirRight, DirNone}

func Opp(dir Direction) Direction {
	return oppDir[dir]
}

// facingTbl maps (dy+1)*3+(dx+1) to Direction for adjacent cells.
var facingTbl = [9]Direction{
	DirNone,  // dy=-1, dx=-1
	DirUp,    // dy=-1, dx=0
	DirNone,  // dy=-1, dx=1
	DirLeft,  // dy=0,  dx=-1
	DirNone,  // dy=0,  dx=0
	DirRight, // dy=0,  dx=1
	DirNone,  // dy=1,  dx=-1
	DirDown,  // dy=1,  dx=0
	DirNone,  // dy=1,  dx=1
}

func FacingPts(head, neck Point) Direction {
	dx := head.X - neck.X
	dy := head.Y - neck.Y
	if dx < -1 || dx > 1 || dy < -1 || dy > 1 {
		return DirNone
	}
	return facingTbl[(dy+1)*3+(dx+1)]
}

// LegalDirs returns the 3 non-back directions for facing.
// facing must not be DirNone.
func LegalDirs(facing Direction) [3]Direction {
	back := Opp(facing)
	var out [3]Direction
	n := 0
	for d := DirUp; d <= DirLeft; d++ {
		if d != back {
			out[n] = d
			n++
		}
	}
	return out
}

// ValidDirs returns up to 4 non-back directions.
// Handles DirNone (returns all 4 cardinal directions).
func ValidDirs(facing Direction) ([4]Direction, int) {
	if facing == DirNone {
		return [4]Direction{DirUp, DirRight, DirDown, DirLeft}, 4
	}
	back := Opp(facing)
	var out [4]Direction
	n := 0
	for d := DirUp; d <= DirLeft; d++ {
		if d != back {
			out[n] = d
			n++
		}
	}
	return out, n
}
