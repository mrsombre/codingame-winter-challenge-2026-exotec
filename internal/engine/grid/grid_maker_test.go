// Package grid
package grid

import (
	"fmt"
	"strings"
	"testing"
)

const (
	testArenaSeed = int64(1001)
)

// javaExpectedInitialInput is the expected output as produced by the Java referee.
// When CG's seed derivation is understood, TestGridMakerInitialInput should
// be updated to reproduce this exactly.
var testArenaExpectedInitialInput = strings.Join([]string{
	"0",
	"32",
	"17",
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
	"4",
	"0",
	"1",
	"2",
	"3",
	"4",
	"5",
	"6",
	"7",
	"18",
	"4 1",
	"27 1",
	"1 3",
	"30 3",
	"2 7",
	"29 7",
	"3 7",
	"28 7",
	"8 1",
	"23 1",
	"5 5",
	"26 5",
	"11 6",
	"20 6",
	"2 8",
	"29 8",
	"3 12",
	"28 12",
	"8",
	"0 18,7:18,8:18,9",
	"1 7,3:7,4:7,5",
	"2 14,13:14,14:14,15",
	"3 21,9:21,10:21,11",
	"4 13,7:13,8:13,9",
	"5 24,3:24,4:24,5",
	"6 17,13:17,14:17,15",
	"7 10,9:10,10:10,11",
}, "\n")

func buildInitialInput(seed int64, leagueLevel int) string {
	rng := NewSHA1PRNG(seed)
	gm := NewGridMaker(rng, leagueLevel)
	g := gm.Make()

	var lines []string
	lines = append(lines, "0")
	lines = append(lines, fmt.Sprintf("%d", g.Width))
	lines = append(lines, fmt.Sprintf("%d", g.Height))
	for y := 0; y < g.Height; y++ {
		var row strings.Builder
		for x := 0; x < g.Width; x++ {
			if g.GetXY(x, y).Type == TileWall {
				row.WriteByte('#')
			} else {
				row.WriteByte('.')
			}
		}
		lines = append(lines, row.String())
	}
	spawnIslands := g.DetectSpawnIslands()
	birdBodies := make([][]Coord, len(spawnIslands))
	for i, island := range spawnIslands {
		birdBodies[i] = sortedCoords(island)
	}
	birdsPerPlayer := len(birdBodies)
	lines = append(lines, fmt.Sprintf("%d", birdsPerPlayer))
	for i := 0; i < birdsPerPlayer; i++ {
		lines = append(lines, fmt.Sprintf("%d", i))
	}
	for i := 0; i < birdsPerPlayer; i++ {
		lines = append(lines, fmt.Sprintf("%d", birdsPerPlayer+i))
	}
	lines = append(lines, fmt.Sprintf("%d", len(g.Apples)))
	for _, c := range g.Apples {
		lines = append(lines, c.IntString())
	}
	totalBirds := birdsPerPlayer * 2
	lines = append(lines, fmt.Sprintf("%d", totalBirds))
	for i, body := range birdBodies {
		parts := make([]string, len(body))
		for j, c := range body {
			parts[j] = fmt.Sprintf("%d,%d", c.X, c.Y)
		}
		lines = append(lines, fmt.Sprintf("%d %s", i, strings.Join(parts, ":")))
	}
	for i, body := range birdBodies {
		parts := make([]string, len(body))
		for j, c := range body {
			opp := g.Opposite(c)
			parts[j] = fmt.Sprintf("%d,%d", opp.X, opp.Y)
		}
		lines = append(lines, fmt.Sprintf("%d %s", birdsPerPlayer+i, strings.Join(parts, ":")))
	}
	return strings.Join(lines, "\n")
}

// TestGridMakerSeedDeterminism verifies the GridMaker is deterministic for a given seed.
func TestGridMakerSeedDeterminism(t *testing.T) {
	a := buildInitialInput(testArenaSeed, 1)
	b := buildInitialInput(testArenaSeed, 1)
	if a != b {
		t.Error("GridMaker is not deterministic for the same seed")
	}
}

func TestGridMakerArenaParity(t *testing.T) {
	got := buildInitialInput(testArenaSeed, 1)
	if got != testArenaExpectedInitialInput {
		t.Fatalf("arena parity mismatch for seed=%d\n\n--- got ---\n%s\n\n--- want ---\n%s", testArenaSeed, got, testArenaExpectedInitialInput)
	}
}
