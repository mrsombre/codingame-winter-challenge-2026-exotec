package main

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	"codingame/internal/engine"

	"github.com/stretchr/testify/assert"
)

const testSeed int64 = -4514697363674281500
const testLeague = 3

func gameOpts(opts ...int64) (int64, int) {
	if len(opts) == 0 {
		return testSeed, testLeague
	}
	if len(opts) == 1 {
		return opts[0], testLeague
	}
	return opts[0], int(opts[1])
}

// testGame creates a Game initialized from engine-generated grid (Init only, no turn data).
func testGame(opts ...int64) *Game {
	eg := engine.NewGame(gameOpts(opts...))
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})

	lines := engine.SerializeGlobalInfoFor(p0, eg)
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	return Init(s)
}

// testGameFull creates a Game with grid + first turn data (apples + snakes).
func testGameFull(opts ...int64) *Game {
	eg := engine.NewGame(gameOpts(opts...))
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})

	lines := engine.SerializeGlobalInfoFor(p0, eg)
	lines = append(lines, engine.SerializeFrameInfoFor(p0, eg)...)
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	g := Init(s)
	g.Read(s)
	return g
}

func testGridInput(lines []string) *Game {
	w := len(lines[0])
	h := len(lines)
	header := []string{
		"0",
		fmt.Sprintf("%d", w),
		fmt.Sprintf("%d", h),
	}
	all := append(header, lines...)
	all = append(all, "0") // snake count

	s := bufio.NewScanner(strings.NewReader(strings.Join(all, "\n")))
	g := Init(s)
	return g
}

func testMovePlan(gridLines []string, apples [][2]int) (*Game, *Plan) {
	g := testGridInput(gridLines)
	p := &Plan{g: g}
	g.Ap = g.Ap[:0]
	for _, a := range apples {
		g.Ap = append(g.Ap, g.Idx(a[0], a[1]))
	}
	g.ANum = len(g.Ap)
	p.Precompute()
	p.RebuildAppleMap()
	return g, p
}

// --- Dl, Dn ---

func TestDirDelta(t *testing.T) {
	assert.Equal(t, [2]int{0, -1}, Dl[DU])
	assert.Equal(t, [2]int{1, 0}, Dl[DR])
	assert.Equal(t, [2]int{0, 1}, Dl[DD])
	assert.Equal(t, [2]int{-1, 0}, Dl[DL])
}

func TestDirName(t *testing.T) {
	assert.Equal(t, "UP", Dn[DU])
	assert.Equal(t, "RIGHT", Dn[DR])
	assert.Equal(t, "DOWN", Dn[DD])
	assert.Equal(t, "LEFT", Dn[DL])
}

// --- Init ---

func TestInit(t *testing.T) {
	g := testGameFull()

	// grid dimensions: 28x15
	assert.Equal(t, 28, g.W)
	assert.Equal(t, 15, g.H)

	// snake IDs
	assert.Equal(t, 3, g.MyN)
	assert.Equal(t, 0, g.MyIDs[0])
	assert.Equal(t, 1, g.MyIDs[1])
	assert.Equal(t, 2, g.MyIDs[2])
	assert.Equal(t, 3, g.OpN)
	assert.Equal(t, 3, g.OpIDs[0])
	assert.Equal(t, 4, g.OpIDs[1])
	assert.Equal(t, 5, g.OpIDs[2])

	// apples
	assert.Equal(t, 30, g.ANum)
	assert.Equal(t, g.Idx(20, 9), g.Ap[0])
	assert.Equal(t, g.Idx(18, 11), g.Ap[g.ANum-1])

	// snakes
	assert.Equal(t, 6, g.SNum)
	assert.Equal(t, 0, g.Sn[0].ID)
	assert.Equal(t, 0, g.Sn[0].Owner)
	assert.Equal(t, 3, g.Sn[0].Len)
	assert.Equal(t, g.Idx(16, 10), g.Sn[0].Body[0])
	assert.Equal(t, 5, g.Sn[5].ID)
	assert.Equal(t, 1, g.Sn[5].Owner)
	assert.Equal(t, 3, g.Sn[5].Len)
	assert.Equal(t, g.Idx(22, 6), g.Sn[5].Body[0])
}

func TestIsWall(t *testing.T) {
	g := testGame()

	// row 0: all free
	assert.Equal(t, true, g.Cell[g.Idx(0, 0)], "cell (0,0) should be free")
	// row 6: ..........#......#.......... — position 10 is '#'
	assert.Equal(t, false, g.Cell[g.Idx(10, 6)], "cell (10,6) should be wall")
}

func TestNeighbors(t *testing.T) {
	g := testGame()

	// Nb: all neighbors have real indices, -1 only at expanded grid edge
	tests := []struct {
		name string
		x, y int
		wantNb  [4]int // [UP, RIGHT, DOWN, LEFT]
		wantNbm [4]int // valid moves (no walls)
	}{
		{"corner top-left", 0, 0,
			[4]int{g.Idx(0, -1), g.Idx(1, 0), g.Idx(0, 1), g.Idx(-1, 0)},
			[4]int{g.Idx(0, -1), g.Idx(1, 0), g.Idx(0, 1), g.Idx(-1, 0)}},
		{"corner top-right", 27, 0,
			[4]int{g.Idx(27, -1), g.Idx(28, 0), g.Idx(27, 1), g.Idx(26, 0)},
			[4]int{g.Idx(27, -1), g.Idx(28, 0), g.Idx(27, 1), g.Idx(26, 0)}},
		// (0,14) wall: UP/RIGHT are walls → Nb has indices, Nbm has -1
		{"corner bot-left", 0, 14,
			[4]int{g.Idx(0, 13), g.Idx(1, 14), g.Idx(0, 15), g.Idx(-1, 14)},
			[4]int{-1, -1, g.Idx(0, 15), g.Idx(-1, 14)}},
		// (27,14) wall: UP/LEFT are walls
		{"corner bot-right", 27, 14,
			[4]int{g.Idx(27, 13), g.Idx(28, 14), g.Idx(27, 15), g.Idx(26, 14)},
			[4]int{-1, g.Idx(28, 14), g.Idx(27, 15), -1}},
		// (10,1) all free
		{"all free", 10, 1,
			[4]int{g.Idx(10, 0), g.Idx(11, 1), g.Idx(10, 2), g.Idx(9, 1)},
			[4]int{g.Idx(10, 0), g.Idx(11, 1), g.Idx(10, 2), g.Idx(9, 1)}},
		// (10,6) wall cell: Nb has all neighbors, Nbm = -1 for self (wall) neighbors check not needed — Nbm filters wall AT neighbor
		{"wall cell", 10, 6,
			[4]int{g.Idx(10, 5), g.Idx(11, 6), g.Idx(10, 7), g.Idx(9, 6)},
			[4]int{g.Idx(10, 5), g.Idx(11, 6), g.Idx(10, 7), g.Idx(9, 6)}},
		// (5,13) free but walls below/left
		{"near bottom", 5, 13,
			[4]int{g.Idx(5, 12), g.Idx(6, 13), g.Idx(5, 14), g.Idx(4, 13)},
			[4]int{g.Idx(5, 12), g.Idx(6, 13), -1, -1}},
	}
	for _, tt := range tests {
		idx := g.Idx(tt.x, tt.y)
		assert.Equal(t, tt.wantNb, g.Nb[idx], "%s Nb", tt.name)
		assert.Equal(t, tt.wantNbm, g.Nbm[idx], "%s Nbm", tt.name)
	}
}

// --- XY ---

func TestIdxXYRoundtrip(t *testing.T) {
	g := testGame()

	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			rx, ry := g.XY(g.Idx(x, y))
			assert.Equal(t, x, rx)
			assert.Equal(t, y, ry)
		}
	}
}

// --- IsMy ---

func TestIsMy(t *testing.T) {
	g := testGame()

	tests := []struct {
		id   int
		want bool
	}{
		{0, true},
		{1, true},
		{2, true},
		{3, false},
		{4, false},
		{5, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, g.IsMy(tt.id), "IsMy(%d)", tt.id)
	}
}

// --- Read ---

func TestRead(t *testing.T) {
	g := testGameFull()

	// verify initial state from engine (new 28x15 map has 30 apples)
	assert.Equal(t, 30, g.ANum)
	assert.Equal(t, 6, g.SNum)
	assert.Equal(t, 3, g.Sn[0].Len)
	assert.Equal(t, 3, g.Sn[5].Len)

	// simulate second turn: snake 0 grew, some apples eaten
	turn2 := strings.Join([]string{
		"2",
		"9 9",
		"18 9",
		"2",
		"0 17,10:16,10:16,11:16,12",
		"3 11,9:11,10:11,11",
	}, "\n")
	g.Read(bufio.NewScanner(strings.NewReader(turn2)))

	// apples reduced
	assert.Equal(t, 2, g.ANum)
	assert.Equal(t, g.Idx(9, 9), g.Ap[0])
	assert.Equal(t, g.Idx(18, 9), g.Ap[1])

	// snake 0 grew from 3 to 4
	assert.Equal(t, 0, g.Sn[0].ID)
	assert.Equal(t, 0, g.Sn[0].Owner)
	assert.Equal(t, 4, g.Sn[0].Len)
	assert.Equal(t, g.Idx(17, 10), g.Sn[0].Body[0])

	// snake 3 (opponent)
	assert.Equal(t, 3, g.Sn[1].ID)
	assert.Equal(t, 1, g.Sn[1].Owner)
	assert.Equal(t, 3, g.Sn[1].Len)
	assert.Equal(t, g.Idx(11, 9), g.Sn[1].Body[0])
}

// --- ParseBody ---

func TestParseBody(t *testing.T) {
	g := testGame()

	tests := []struct {
		name    string
		input   string
		wantLen int
		wantIdx []int
	}{
		{
			"single cell",
			"5,3",
			1,
			[]int{g.Idx(5, 3)},
		},
		{
			"three cells",
			"0,0:1,0:2,0",
			3,
			[]int{g.Idx(0, 0), g.Idx(1, 0), g.Idx(2, 0)},
		},
		{
			"vertical body",
			"10,5:10,6:10,7:10,8",
			4,
			[]int{g.Idx(10, 5), g.Idx(10, 6), g.Idx(10, 7), g.Idx(10, 8)},
		},
		{
			"two digit coords",
			"17,0:17,1:17,2",
			3,
			[]int{g.Idx(17, 0), g.Idx(17, 1), g.Idx(17, 2)},
		},
	}
	for _, tt := range tests {
		body := g.ParseBody(tt.input)
		assert.Equal(t, tt.wantLen, len(body), tt.name)
		for i, want := range tt.wantIdx {
			assert.Equal(t, want, body[i], "%s body[%d]", tt.name, i)
		}
	}
}

