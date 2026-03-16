package main

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	"codingame/internal/engine"

	"github.com/stretchr/testify/assert"
)

const testSeed int64 = -6896487110651623000

// testGame creates a Game initialized from engine-generated grid (Init only, no turn data).
func testGame() *Game {
	eg := engine.NewGame(testSeed, 3)
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})

	lines := engine.SerializeGlobalInfoFor(p0, eg)
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	return Init(s)
}

// testGameFull creates a Game with grid + first turn data (apples + snakes).
func testGameFull() *Game {
	eg := engine.NewGame(testSeed, 3)
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

	// grid dimensions: 22x12
	assert.Equal(t, 22, g.W)
	assert.Equal(t, 12, g.H)

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
	assert.Equal(t, 12, g.ANum)
	assert.Equal(t, g.Idx(9, 9), g.Ap[0])
	assert.Equal(t, g.Idx(12, 9), g.Ap[1])

	// snakes
	assert.Equal(t, 6, g.SNum)
	assert.Equal(t, 0, g.Sn[0].ID)
	assert.Equal(t, 0, g.Sn[0].Owner)
	assert.Equal(t, 3, g.Sn[0].Len)
	assert.Equal(t, g.Idx(3, 8), g.Sn[0].Body[0])
	assert.Equal(t, 5, g.Sn[5].ID)
	assert.Equal(t, 1, g.Sn[5].Owner)
	assert.Equal(t, 3, g.Sn[5].Len)
	assert.Equal(t, g.Idx(5, 0), g.Sn[5].Body[0])
}

func TestIsWall(t *testing.T) {
	g := testGame()

	// row 0: all free
	assert.Equal(t, true, g.Cell[g.Idx(0, 0)], "cell (0,0) should be free")
	// row 3: ....##...####...##.... — position 4 is '#'
	assert.Equal(t, false, g.Cell[g.Idx(4, 3)], "cell (4,3) should be wall")
}

func TestNeighbors(t *testing.T) {
	g := testGame()

	// W=22, H=12, OobBase=264
	// OOB layout: top(264+x), bottom(286+x), left(308+y), right(320+y)
	tests := []struct {
		name string
		x, y int
		want [4]int // [UP, RIGHT, DOWN, LEFT]
	}{
		{"corner top-left", 0, 0, [4]int{264, 1, 22, 308}},
		{"corner top-right", 21, 0, [4]int{285, 320, 43, 20}},
		{"corner bot-left", 0, 11, [4]int{-1, -1, 286, 319}},
		{"corner bot-right", 21, 11, [4]int{-1, 331, 307, -1}},
		// (10,1) all free neighbors — unchanged
		{"all free", 10, 1, [4]int{10, 33, 54, 31}},
		// (10,11) wall: UP/RIGHT/LEFT walls, DOWN=OOB bottom border
		{"wall with oob", 10, 11, [4]int{-1, -1, 296, -1}},
		// (4,3)='#': UP=(4,2) free, RIGHT=(5,3) wall, DOWN=(4,4) free, LEFT=(3,3) free
		{"mixed wall", 4, 3, [4]int{48, -1, 92, 69}},
	}
	for _, tt := range tests {
		nb := g.Nb[g.Idx(tt.x, tt.y)]
		assert.Equal(t, tt.want, nb, tt.name)
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

	// verify initial state from engine
	assert.Equal(t, 12, g.ANum)
	assert.Equal(t, 6, g.SNum)
	assert.Equal(t, 3, g.Sn[0].Len)
	assert.Equal(t, 3, g.Sn[5].Len)

	// simulate second turn: snake 0 grew, some apples eaten
	turn := &testTurnInput{
		Apples: [][2]int{{9, 9}, {12, 9}},
		Snakes: []struct {
			ID   int
			Body [][2]int
		}{
			{0, [][2]int{{3, 7}, {3, 8}, {3, 9}, {3, 10}}},
			{3, [][2]int{{18, 7}, {18, 8}, {18, 9}}},
		},
	}
	g.Read(turn.Scanner())

	// apples reduced
	assert.Equal(t, 2, g.ANum)
	assert.Equal(t, g.Idx(9, 9), g.Ap[0])
	assert.Equal(t, g.Idx(12, 9), g.Ap[1])

	// snake 0 grew from 3 to 4
	assert.Equal(t, 0, g.Sn[0].ID)
	assert.Equal(t, 0, g.Sn[0].Owner)
	assert.Equal(t, 4, g.Sn[0].Len)
	assert.Equal(t, g.Idx(3, 7), g.Sn[0].Body[0])

	// snake 3 (opponent)
	assert.Equal(t, 3, g.Sn[1].ID)
	assert.Equal(t, 1, g.Sn[1].Owner)
	assert.Equal(t, 3, g.Sn[1].Len)
	assert.Equal(t, g.Idx(18, 7), g.Sn[1].Body[0])
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

// helpers

type testTurnInput struct {
	Apples [][2]int
	Snakes []struct {
		ID   int
		Body [][2]int
	}
}

func (in *testTurnInput) Scanner() *bufio.Scanner {
	var lines []string

	lines = append(lines, fmt.Sprintf("%d", len(in.Apples)))
	for _, a := range in.Apples {
		lines = append(lines, fmt.Sprintf("%d %d", a[0], a[1]))
	}

	lines = append(lines, fmt.Sprintf("%d", len(in.Snakes)))
	for _, s := range in.Snakes {
		lines = append(lines, fmt.Sprintf("%d %s", s.ID, formatBody(s.Body)))
	}

	return bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
}

func formatBody(body [][2]int) string {
	var b strings.Builder
	for i, p := range body {
		if i > 0 {
			b.WriteByte(':')
		}
		fmt.Fprintf(&b, "%d,%d", p[0], p[1])
	}
	return b.String()
}
