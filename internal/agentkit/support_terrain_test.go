package agentkit

import "testing"

func TestSTerrainSeed18UpperAppleNeedsLengthFour(t *testing.T) {
	terrain := sTerrainFromLayout([]string{
		"..................",
		"...##........##...",
		"...#....##....#...",
		"...#..........#...",
		"..#............#..",
		"..................",
		".##............##.",
		"...#..........#...",
		"##....######....##",
		"##################",
	})

	body := []Point{{X: 2, Y: 1}, {X: 2, Y: 2}, {X: 2, Y: 3}}
	target := Point{X: 7, Y: 0}

	if got := terrain.MinBodyLen(body, target); got != 4 {
		t.Fatalf("MinBodyLen() = %d, want 4", got)
	}
}

func TestSTerrainApprNodesPreferLowestSupport(t *testing.T) {
	terrain := sTerrainFromLayout([]string{
		".....",
		"..#..",
		".....",
		"#####",
	})

	got := terrain.ApprNodes(Point{X: 2, Y: 0})
	if len(got) != 1 {
		t.Fatalf("len(ApprNodes) = %d, want 1", len(got))
	}
	if terrain.Nodes[got[0]].Pos != (Point{X: 2, Y: 0}) {
		t.Fatalf("approach node = %+v, want {2 0}", terrain.Nodes[got[0]].Pos)
	}
}

func TestSTerrainSeed1001CenterBotNeedsLengthFive(t *testing.T) {
	terrain := sTerrainFromLayout([]string{
		".#............................#.",
		".#............................#.",
		"..#..........................#..",
		"...........#........#...........",
		"...........##......##...........",
		"................................",
		".##....#................#....##.",
		".......#......#..#......#.......",
		".........##....##....##.........",
		"#......##..............##......#",
		".##.........##....##.........##.",
		"........##..#......#..##........",
		".#.....####.#......#.####.....#.",
		".#....####..#..##..#..####....#.",
		".#..######..#..##..#..######..#.",
		"#############..##..#############",
		"################################",
	})

	body := []Point{{X: 14, Y: 13}, {X: 14, Y: 14}, {X: 14, Y: 15}}
	target := Point{X: 11, Y: 6}

	if got := terrain.MinBodyLen(body, target); got != 5 {
		t.Fatalf("MinBodyLen() = %d, want 5", got)
	}
}

func TestSTerrainSeed1001TopAppleApproaches(t *testing.T) {
	layout := []string{
		".#............................#.",
		".#............................#.",
		"..#..........................#..",
		"...........#........#...........",
		"...........##......##...........",
		"................................",
		".##....#................#....##.",
		".......#......#..#......#.......",
		".........##....##....##.........",
		"#......##..............##......#",
		".##.........##....##.........##.",
		"........##..#......#..##........",
		".#.....####.#......#.####.....#.",
		".#....####..#..##..#..####....#.",
		".#..######..#..##..#..######..#.",
		"#############..##..#############",
		"################################",
	}
	apples := []Point{
		{X: 4, Y: 1}, {X: 27, Y: 1}, {X: 1, Y: 3}, {X: 30, Y: 3},
		{X: 2, Y: 7}, {X: 29, Y: 7}, {X: 3, Y: 7}, {X: 28, Y: 7},
		{X: 8, Y: 1}, {X: 23, Y: 1}, {X: 5, Y: 5}, {X: 26, Y: 5},
		{X: 11, Y: 6}, {X: 20, Y: 6}, {X: 2, Y: 8}, {X: 29, Y: 8},
		{X: 3, Y: 12}, {X: 28, Y: 12},
	}

	state := stateFromLayout(layout, apples)
	approaches := TgtAppr(&state, Point{X: 4, Y: 1})
	if len(approaches) == 0 {
		t.Fatal("TgtAppr() returned no approaches")
	}

	assertApproach := func(support Point, minL int) {
		t.Helper()
		for _, a := range approaches {
			if a.Cell == support && a.MinL == minL {
				return
			}
		}
		t.Fatalf("missing approach cell=%+v minL=%d in %+v", support, minL, approaches)
	}

	assertApproach(Point{X: 2, Y: 2}, 2)
	assertApproach(Point{X: 5, Y: 5}, 4)
	assertApproach(Point{X: 8, Y: 1}, 4)
}

func TestSTerrainSeed1001CloseSupFor116(t *testing.T) {
	layout := []string{
		".#............................#.",
		".#............................#.",
		"..#..........................#..",
		"...........#........#...........",
		"...........##......##...........",
		"................................",
		".##....#................#....##.",
		".......#......#..#......#.......",
		".........##....##....##.........",
		"#......##..............##......#",
		".##.........##....##.........##.",
		"........##..#......#..##........",
		".#.....####.#......#.####.....#.",
		".#....####..#..##..#..####....#.",
		".#..######..#..##..#..######..#.",
		"#############..##..#############",
		"################################",
	}
	apples := []Point{
		{X: 4, Y: 1}, {X: 27, Y: 1}, {X: 1, Y: 3}, {X: 30, Y: 3},
		{X: 2, Y: 7}, {X: 29, Y: 7}, {X: 3, Y: 7}, {X: 28, Y: 7},
		{X: 8, Y: 1}, {X: 23, Y: 1}, {X: 5, Y: 5}, {X: 26, Y: 5},
		{X: 11, Y: 6}, {X: 20, Y: 6}, {X: 2, Y: 8}, {X: 29, Y: 8},
		{X: 3, Y: 12}, {X: 28, Y: 12},
	}

	state := stateFromLayout(layout, apples)
	got := CloseSup(&state, Point{X: 11, Y: 6})

	assertHas := func(support Point, minL int) {
		t.Helper()
		for _, a := range got {
			if a.Cell == support && a.MinL == minL {
				return
			}
		}
		t.Fatalf("missing closest cell=%+v minL=%d in %+v", support, minL, got)
	}
	assertMissing := func(support Point) {
		t.Helper()
		for _, a := range got {
			if a.Cell == support {
				t.Fatalf("unexpected duplicate cell=%+v in %+v", support, got)
			}
		}
	}

	assertHas(Point{X: 10, Y: 8}, 2)
	assertHas(Point{X: 14, Y: 7}, 3)
	assertHas(Point{X: 12, Y: 10}, 4)
	assertMissing(Point{X: 13, Y: 10})
	assertMissing(Point{X: 15, Y: 8})
}

func TestStateRebuildSupMirrorsSeed1001(t *testing.T) {
	layout := []string{
		".#............................#.",
		".#............................#.",
		"..#..........................#..",
		"...........#........#...........",
		"...........##......##...........",
		"................................",
		".##....#................#....##.",
		".......#......#..#......#.......",
		".........##....##....##.........",
		"#......##..............##......#",
		".##.........##....##.........##.",
		"........##..#......#..##........",
		".#.....####.#......#.####.....#.",
		".#....####..#..##..#..####....#.",
		".#..######..#..##..#..######..#.",
		"#############..##..#############",
		"################################",
	}
	apples := []Point{
		{X: 4, Y: 1}, {X: 27, Y: 1}, {X: 1, Y: 3}, {X: 30, Y: 3},
		{X: 2, Y: 7}, {X: 29, Y: 7}, {X: 3, Y: 7}, {X: 28, Y: 7},
		{X: 8, Y: 1}, {X: 23, Y: 1}, {X: 5, Y: 5}, {X: 26, Y: 5},
		{X: 11, Y: 6}, {X: 20, Y: 6}, {X: 2, Y: 8}, {X: 29, Y: 8},
		{X: 3, Y: 12}, {X: 28, Y: 12},
	}

	state := stateFromLayout(layout, apples)
	state.RebuildSup()

	left := state.AppleSup[Point{X: 4, Y: 1}]
	right := state.AppleSup[Point{X: 27, Y: 1}]
	if len(left) == 0 || len(right) == 0 {
		t.Fatalf("expected mirrored apple supports, got left=%+v right=%+v", left, right)
	}

	assertHas := func(approaches []TAppr, support Point, minL int) {
		t.Helper()
		for _, a := range approaches {
			if a.Cell == support && a.MinL == minL {
				return
			}
		}
		t.Fatalf("missing cell=%+v minL=%d in %+v", support, minL, approaches)
	}

	assertHas(left, Point{X: 2, Y: 2}, 2)
	assertHas(right, Point{X: 29, Y: 2}, 2)
}

func TestStateRebuildSupFor206UsesLocalSupports(t *testing.T) {
	layout := []string{
		".#............................#.",
		".#............................#.",
		"..#..........................#..",
		"...........#........#...........",
		"...........##......##...........",
		"................................",
		".##....#................#....##.",
		".......#......#..#......#.......",
		".........##....##....##.........",
		"#......##..............##......#",
		".##.........##....##.........##.",
		"........##..#......#..##........",
		".#.....####.#......#.####.....#.",
		".#....####..#..##..#..####....#.",
		".#..######..#..##..#..######..#.",
		"#############..##..#############",
		"################################",
	}
	apples := []Point{
		{X: 4, Y: 1}, {X: 27, Y: 1}, {X: 1, Y: 3}, {X: 30, Y: 3},
		{X: 2, Y: 7}, {X: 29, Y: 7}, {X: 3, Y: 7}, {X: 28, Y: 7},
		{X: 8, Y: 1}, {X: 23, Y: 1}, {X: 5, Y: 5}, {X: 26, Y: 5},
		{X: 11, Y: 6}, {X: 20, Y: 6}, {X: 2, Y: 8}, {X: 29, Y: 8},
		{X: 3, Y: 12}, {X: 28, Y: 12},
	}

	state := stateFromLayout(layout, apples)
	state.RebuildSup()
	got := state.AppleSup[Point{X: 20, Y: 6}]

	assertHas := func(support Point, minL int) {
		t.Helper()
		for _, a := range got {
			if a.Cell == support && a.MinL == minL {
				return
			}
		}
		t.Fatalf("missing cell=%+v minL=%d in %+v", support, minL, got)
	}
	assertMissing := func(support Point) {
		t.Helper()
		for _, a := range got {
			if a.Cell == support {
				t.Fatalf("unexpected cell=%+v in %+v", support, got)
			}
		}
	}

	assertHas(Point{X: 21, Y: 8}, 2)
	assertHas(Point{X: 17, Y: 7}, 3)
	assertHas(Point{X: 19, Y: 10}, 4)
	assertMissing(Point{X: 20, Y: 6})
}

func sTerrainFromLayout(layout []string) *STerrain {
	walls := make(map[Point]bool)
	for y, row := range layout {
		for x, ch := range row {
			if ch == '#' {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}
	grid := NewAG(len(layout[0]), len(layout), walls)
	return NewSTerrain(grid)
}

func bitGridFromPoints(width, height int, points []Point) BitGrid {
	grid := NewBG(width, height)
	for _, p := range points {
		grid.Set(p)
	}
	return grid
}

func stateFromLayout(layout []string, apples []Point) State {
	walls := make(map[Point]bool)
	for y, row := range layout {
		for x, ch := range row {
			if ch == '#' {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}
	grid := NewAG(len(layout[0]), len(layout), walls)
	state := NewState(grid)
	for _, p := range apples {
		state.Apples.Set(p)
	}
	return state
}
