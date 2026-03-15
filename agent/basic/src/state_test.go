package main

import (
	"bufio"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

var stateGrid = NewGrid(20, 11, []string{
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
	"....................",
})

func testState() *State {
	st := &State{
		G:     stateGrid,
		ID:    0,
		MyIDs: [MaxPSn]int{0, 2, 4},
		MyN:   3,
		OpIDs: [MaxPSn]int{1, 3, 5},
		OpN:   3,
	}
	return st
}

func TestIsMyID(t *testing.T) {
	st := testState()
	tests := []struct {
		id   int
		want bool
	}{
		{0, true},
		{2, true},
		{4, true},
		{1, false},
		{3, false},
		{5, false},
		{99, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, st.IsMyID(tt.id), "IsMyID(%d)", tt.id)
	}
}

func TestSetApple(t *testing.T) {
	st := testState()
	st.ANum = 3
	st.SetApple(0, 0, 0)
	st.SetApple(1, 5, 3)
	st.SetApple(2, 19, 10)

	assert.Equal(t, st.G.Idx(0, 0), st.Ap[0])
	assert.Equal(t, st.G.Idx(5, 3), st.Ap[1])
	assert.Equal(t, st.G.Idx(19, 10), st.Ap[2])
}

func TestSetSnake(t *testing.T) {
	st := testState()
	st.SNum = 2

	st.SetSnake(0, 0, "0,0:1,0:2,0")
	st.SetSnake(1, 1, "4,0:3,0")

	// my snake
	s0 := &st.Sn[0]
	assert.Equal(t, 0, s0.ID)
	assert.Equal(t, 0, s0.Owner)
	assert.True(t, s0.Alive)
	assert.Equal(t, 3, s0.Len)
	assert.Equal(t, st.G.Idx(0, 0), s0.Head())
	assert.Equal(t, st.G.Idx(1, 0), s0.Body[1])
	assert.Equal(t, st.G.Idx(2, 0), s0.Body[2])

	// enemy snake
	s1 := &st.Sn[1]
	assert.Equal(t, 1, s1.ID)
	assert.Equal(t, 1, s1.Owner)
	assert.True(t, s1.Alive)
	assert.Equal(t, 2, s1.Len)
	assert.Equal(t, st.G.Idx(4, 0), s1.Head())
}

func TestParseBody(t *testing.T) {
	g := stateGrid
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
		var body [MaxSeg]int
		n := ParseBody(tt.input, &body, g)
		assert.Equal(t, tt.wantLen, n, tt.name)
		for i, want := range tt.wantIdx {
			assert.Equal(t, want, body[i], "%s body[%d]", tt.name, i)
		}
	}
}

func TestSnakeHead(t *testing.T) {
	s := Snake{Body: [MaxSeg]int{42, 10, 5}, Len: 3}
	assert.Equal(t, 42, s.Head())
}

func TestReadInit(t *testing.T) {
	// player id=0, 20x11 grid, 3 snakes per side
	lines := []string{
		"0",
		"20", "11",
	}
	for i := 0; i < 11; i++ {
		lines = append(lines, "....................")
	}
	lines = append(lines, "3", "0", "2", "4", "1", "3", "5")

	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	st := &State{}
	SFI(s, st)

	want := testState()
	assert.Equal(t, want.ID, st.ID)
	assert.Equal(t, want.MyN, st.MyN)
	assert.Equal(t, want.MyIDs, st.MyIDs)
	assert.Equal(t, want.OpN, st.OpN)
	assert.Equal(t, want.OpIDs, st.OpIDs)
	assert.Equal(t, want.G.W, st.G.W)
	assert.Equal(t, want.G.H, st.G.H)
}

func TestReadTurn(t *testing.T) {
	st := testState()

	lines := []string{
		"2",
		"5 3",
		"19 10",
		"2",
		"0 0,0:1,0:2,0",
		"1 4,0:3,0",
	}
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	RT(s, st)

	// apples
	assert.Equal(t, 2, st.ANum)
	assert.Equal(t, st.G.Idx(5, 3), st.Ap[0])
	assert.Equal(t, st.G.Idx(19, 10), st.Ap[1])

	// snakes
	assert.Equal(t, 2, st.SNum)
	s0 := &st.Sn[0]
	assert.Equal(t, 0, s0.ID)
	assert.Equal(t, 0, s0.Owner)
	assert.Equal(t, 3, s0.Len)
	assert.Equal(t, st.G.Idx(0, 0), s0.Head())

	s1 := &st.Sn[1]
	assert.Equal(t, 1, s1.ID)
	assert.Equal(t, 1, s1.Owner)
	assert.Equal(t, 2, s1.Len)
	assert.Equal(t, st.G.Idx(4, 0), s1.Head())
}

func TestReadTurnStaleData(t *testing.T) {
	st := testState()

	// turn 1: 3 apples, 4 snakes
	turn1 := []string{
		"3",
		"0 0", "1 1", "2 2",
		"4",
		"0 0,0:1,0:2,0",
		"1 4,0:3,0",
		"2 10,0:11,0",
		"3 15,0:16,0",
	}
	RT(stateScanner(turn1...), st)
	assert.Equal(t, 3, st.ANum)
	assert.Equal(t, 4, st.SNum)
	assert.True(t, st.Sn[2].Alive)
	assert.True(t, st.Sn[3].Alive)

	// turn 2: 1 apple eaten, 2 snakes died
	turn2 := []string{
		"1",
		"0 0",
		"2",
		"0 0,0:1,0:2,0",
		"1 4,0:3,0",
	}
	RT(stateScanner(turn2...), st)

	assert.Equal(t, 1, st.ANum)
	assert.Equal(t, 2, st.SNum)
	// stale slots must be clean
	assert.Equal(t, 0, st.Ap[1], "stale apple[1]")
	assert.Equal(t, 0, st.Ap[2], "stale apple[2]")
	assert.False(t, st.Sn[2].Alive, "stale snake[2]")
	assert.False(t, st.Sn[3].Alive, "stale snake[3]")
}

// helpers

func stateScanner(lines ...string) *bufio.Scanner {
	return bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
}

