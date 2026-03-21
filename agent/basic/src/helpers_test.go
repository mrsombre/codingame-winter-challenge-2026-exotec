package main

// setupGrid initialises the global variables (W, H, grid, state, rsc) from
// an ASCII map where '#' is a wall and '.' (or any other char) is open space.
// Every test that touches globals must call this first so the functions under
// test see a consistent world.
//
// Example map (7×5, border walls + one internal wall at 3,2):
//
//	#######
//	#.....#
//	#..#..#
//	#.....#
//	#######
func setupGrid(lines []string) {
	H = len(lines)
	W = len(lines[0])
	walls := make(map[Point]bool)
	for y, row := range lines {
		for x, ch := range row {
			if ch == '#' {
				walls[Point{x, y}] = true
			}
		}
	}
	grid = NewAG(W, H, walls)
	state = NewState(grid)
	rsc = newRefScratch(W, H)
}

// setupOcc builds a BitGrid marking every point in pts as occupied.
// Useful for constructing the "occupied" argument that many functions require.
func setupOcc(pts []Point) BitGrid {
	bg := NewBG(W, H)
	for _, p := range pts {
		bg.Set(p)
	}
	return bg
}

// setupSrcBG builds a BitGrid marking every point in sources as an apple.
func setupSrcBG(sources []Point) BitGrid {
	bg := NewBG(W, H)
	for _, p := range sources {
		bg.Set(p)
	}
	return bg
}

// --- Commonly reused grids ---

// flatFloor is a 7×5 grid with border walls and a completely open interior.
//
//	#######
//	#.....#
//	#.....#
//	#.....#
//	#######
var flatFloor = []string{
	"#######",
	"#.....#",
	"#.....#",
	"#.....#",
	"#######",
}

// gridWithPillar is a 7×5 grid with one internal wall at (3,2).
// The pillar creates interesting BFS / support-structure scenarios.
//
//	#######
//	#.....#
//	#..#..#
//	#.....#
//	#######
var gridWithPillar = []string{
	"#######",
	"#.....#",
	"#..#..#",
	"#.....#",
	"#######",
}

// tallGrid is a 7×8 grid with a floating platform (walls at y=4, x=2..4).
// Snakes above the platform have support; snakes below can fall.
//
//	#######
//	#.....#
//	#.....#
//	#.....#
//	#.###.#
//	#.....#
//	#.....#
//	#######
var tallGrid = []string{
	"#######",
	"#.....#",
	"#.....#",
	"#.....#",
	"#.###.#",
	"#.....#",
	"#.....#",
	"#######",
}
