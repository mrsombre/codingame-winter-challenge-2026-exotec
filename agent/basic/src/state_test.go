package src

import (
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
		G:  stateGrid,
		ID: 0,
		MyIDs: [MaxPerSide]int{0, 2, 4},
		MyN:   3,
		OppIDs: [MaxPerSide]int{1, 3, 5},
		OppN:   3,
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
	st.AppleN = 3
	st.SetApple(0, 0, 0)
	st.SetApple(1, 5, 3)
	st.SetApple(2, 19, 10)

	assert.Equal(t, st.G.Idx(0, 0), st.Apples[0])
	assert.Equal(t, st.G.Idx(5, 3), st.Apples[1])
	assert.Equal(t, st.G.Idx(19, 10), st.Apples[2])
}

func TestSetSnake(t *testing.T) {
	st := testState()
	st.SnakeN = 2

	st.SetSnake(0, 0, "0,0:1,0:2,0")
	st.SetSnake(1, 1, "4,0:3,0")

	// my snake
	s0 := &st.Snakes[0]
	assert.Equal(t, 0, s0.ID)
	assert.Equal(t, 0, s0.Owner)
	assert.True(t, s0.Alive)
	assert.Equal(t, 3, s0.Len)
	assert.Equal(t, st.G.Idx(0, 0), s0.Head())
	assert.Equal(t, st.G.Idx(1, 0), s0.Body[1])
	assert.Equal(t, st.G.Idx(2, 0), s0.Body[2])

	// enemy snake
	s1 := &st.Snakes[1]
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
		var body [MaxBody]int
		n := ParseBody(tt.input, &body, g)
		assert.Equal(t, tt.wantLen, n, tt.name)
		for i, want := range tt.wantIdx {
			assert.Equal(t, want, body[i], "%s body[%d]", tt.name, i)
		}
	}
}

func TestSnakeHead(t *testing.T) {
	s := Snake{Body: [MaxBody]int{42, 10, 5}, Len: 3}
	assert.Equal(t, 42, s.Head())
}
