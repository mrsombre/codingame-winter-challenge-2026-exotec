package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

	assert.Equal(t, 4, terrain.MinBodyLen(body, target))
}

func TestSTerrainApprNodesPreferLowestSupport(t *testing.T) {
	terrain := sTerrainFromLayout([]string{
		".....",
		"..#..",
		".....",
		"#####",
	})

	got := TerrApprNodes(terrain, Point{X: 2, Y: 0})
	require.Len(t, got, 1)
	assert.Equal(t, Point{X: 2, Y: 0}, terrain.Nodes[got[0]].Pos)
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

	assert.Equal(t, 5, terrain.MinBodyLen(body, target))
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
	require.NotEmpty(t, approaches, "TgtAppr() returned no approaches")

	assertApproach := func(support Point, minL int) {
		t.Helper()
		found := false
		for _, a := range approaches {
			if a.Cell == support && a.MinL == minL {
				found = true
				break
			}
		}
		assert.Truef(t, found, "missing approach cell=%+v minL=%d in %+v", support, minL, approaches)
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
		found := false
		for _, a := range got {
			if a.Cell == support && a.MinL == minL {
				found = true
				break
			}
		}
		assert.Truef(t, found, "missing closest cell=%+v minL=%d in %+v", support, minL, got)
	}
	assertMissing := func(support Point) {
		t.Helper()
		found := false
		for _, a := range got {
			if a.Cell == support {
				found = true
				break
			}
		}
		assert.Falsef(t, found, "unexpected duplicate cell=%+v in %+v", support, got)
	}

	assertHas(Point{X: 10, Y: 8}, 2)
	assertHas(Point{X: 14, Y: 7}, 3)
	assertHas(Point{X: 12, Y: 10}, 4)
	assertMissing(Point{X: 13, Y: 10})
	assertMissing(Point{X: 15, Y: 8})
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
