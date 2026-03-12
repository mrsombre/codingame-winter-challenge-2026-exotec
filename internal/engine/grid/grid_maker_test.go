// Package grid
package grid

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testArenaPositiveSeed = int64(468706172918629800)
	testArenaNegativeSeed = int64(-468706172918629800)
)

var testArenaExpectedInitialInputNegative = strings.Join([]string{
	"0",
	"20",
	"11",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....#..#....#..#....",
	"####################",
	"3",
	"0",
	"1",
	"2",
	"3",
	"4",
	"5",
	"18",
	"11 6",
	"8 6",
	"11 0",
	"8 0",
	"16 1",
	"3 1",
	"12 6",
	"7 6",
	"4 3",
	"15 3",
	"6 6",
	"13 6",
	"0 7",
	"19 7",
	"8 7",
	"11 7",
	"2 8",
	"17 8",
	"6",
	"0 4,6:4,7:4,8",
	"1 6,7:6,8:6,9",
	"2 18,7:18,8:18,9",
	"3 15,6:15,7:15,8",
	"4 13,7:13,8:13,9",
	"5 1,7:1,8:1,9",
}, "\n")

var testArenaExpectedInitialInputPositive = strings.Join([]string{
	"0",
	"18",
	"10",
	"..................",
	"........##........",
	"..................",
	"..................",
	"..................",
	"..................",
	"..................",
	"..#............#..",
	".##.....##.....##.",
	"##################",
	"2",
	"0",
	"1",
	"2",
	"3",
	"10",
	"16 0",
	"1 0",
	"6 0",
	"11 0",
	"2 0",
	"15 0",
	"7 3",
	"10 3",
	"4 6",
	"13 6",
	"4",
	"0 10,6:10,7:10,8",
	"1 3,6:3,7:3,8",
	"2 7,6:7,7:7,8",
	"3 14,6:14,7:14,8",
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
	a := buildInitialInput(testArenaPositiveSeed, 1)
	b := buildInitialInput(testArenaPositiveSeed, 1)
	assert.Equal(t, a, b, "GridMaker should be deterministic for the same seed")
}

func TestGridMakerNegativeParityCheck(t *testing.T) {
	got := buildInitialInput(testArenaNegativeSeed, 1)
	assert.Equalf(t, testArenaExpectedInitialInputNegative, got, "arena parity mismatch for seed=%d", testArenaNegativeSeed)
}

func TestGridMakerPositiveParityCheck(t *testing.T) {
	got := buildInitialInput(testArenaPositiveSeed, 1)
	assert.Equalf(t, testArenaExpectedInitialInputPositive, got, "arena parity mismatch for seed=%d", testArenaPositiveSeed)
}
