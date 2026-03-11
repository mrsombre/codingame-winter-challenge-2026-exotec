package engine

import (
	"strings"
	"testing"
)

func TestProbeClimbTrapFallsBack(t *testing.T) {
	layout := []string{
		"...",
		".#.",
		".#.",
		".#.",
		"###",
	}
	start := []Coord{
		{X: 0, Y: 1},
		{X: 0, Y: 2},
		{X: 0, Y: 3},
	}

	alive, got, applesLeft := simulateProbeTurn(layout, start, nil, DirNorth)
	if !alive {
		t.Fatalf("bird died, expected trap to fall back alive")
	}
	if applesLeft != 0 {
		t.Fatalf("applesLeft = %d, want 0", applesLeft)
	}
	if !sameCoords(got, start) {
		t.Fatalf("body after UP = %s, want %s", coordsString(got), coordsString(start))
	}
	if !hasSupport(layout, got, nil) {
		t.Fatalf("alive body must end supported: %s", coordsString(got))
	}
}

func TestProbeClimbTrapWithApple(t *testing.T) {
	layout := []string{
		"...",
		".#.",
		".#.",
		".#.",
		"###",
	}
	start := []Coord{
		{X: 0, Y: 1},
		{X: 0, Y: 2},
		{X: 0, Y: 3},
	}
	apples := []Coord{{X: 0, Y: 0}}

	alive, got, applesLeft := simulateProbeTurn(layout, start, apples, DirNorth)
	t.Logf("apple lure result: alive=%v body=%s apples_left=%d", alive, coordsString(got), applesLeft)
	if alive && !hasSupport(layout, got, nil) {
		t.Fatalf("alive body must end supported: %s", coordsString(got))
	}
}

func TestProbeStepHeightThreeNeedsLengthFourToStepRight(t *testing.T) {
	layout := []string{
		"...",
		".#.",
		".#.",
		".#.",
		"###",
	}

	short := []Coord{
		{X: 0, Y: 1},
		{X: 0, Y: 2},
		{X: 0, Y: 3},
	}
	alive, got, _ := simulateProbeTurn(layout, short, nil, DirEast)
	if alive {
		t.Fatalf("length-3 bird should die stepping right into a 3-high wall, got %s", coordsString(got))
	}

	long := []Coord{
		{X: 0, Y: 0},
		{X: 0, Y: 1},
		{X: 0, Y: 2},
		{X: 0, Y: 3},
	}
	alive, got, _ = simulateProbeTurn(layout, long, nil, DirEast)
	if !alive {
		t.Fatalf("length-4 bird should survive stepping right over a 3-high wall")
	}
	want := []Coord{
		{X: 1, Y: 0},
		{X: 0, Y: 0},
		{X: 0, Y: 1},
		{X: 0, Y: 2},
	}
	if !sameCoords(got, want) {
		t.Fatalf("body after length-4 RIGHT = %s, want %s", coordsString(got), coordsString(want))
	}
}

func simulateProbeTurn(layout []string, body []Coord, apples []Coord, dir Direction) (bool, []Coord, int) {
	game, bird := newProbeGame(layout, body, apples)
	bird.Direction = dir
	game.PerformGameUpdate(0)
	return bird.Alive, append([]Coord(nil), bird.Body...), len(game.Grid.Apples)
}

func newProbeGame(layout []string, body []Coord, apples []Coord) (*Game, *Bird) {
	height := len(layout)
	width := len(layout[0])
	grid := NewGrid(width, height)

	for y, row := range layout {
		for x, ch := range row {
			if ch == '#' {
				grid.GetXY(x, y).Type = TileWall
			}
		}
	}
	grid.Apples = append(grid.Apples, apples...)

	player := NewPlayer(0)
	bird := NewBird(0, player)
	bird.Alive = true
	bird.Body = append([]Coord(nil), body...)
	player.birds = append(player.birds, bird)

	return &Game{
		Players: []*Player{player},
		Grid:    grid,
	}, bird
}

func sameCoords(a, b []Coord) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func coordsString(body []Coord) string {
	parts := make([]string, len(body))
	for i, c := range body {
		parts[i] = c.IntString()
	}
	return strings.Join(parts, " | ")
}

func hasSupport(layout []string, body []Coord, apples []Coord) bool {
	appleSet := map[Coord]bool{}
	for _, a := range apples {
		appleSet[a] = true
	}
	bodySet := map[Coord]bool{}
	for _, c := range body {
		bodySet[c] = true
	}
	for _, c := range body {
		below := Coord{X: c.X, Y: c.Y + 1}
		if bodySet[below] {
			continue
		}
		if below.Y >= 0 && below.Y < len(layout) && below.X >= 0 && below.X < len(layout[0]) && layout[below.Y][below.X] == '#' {
			return true
		}
		if appleSet[below] {
			return true
		}
	}
	return false
}
