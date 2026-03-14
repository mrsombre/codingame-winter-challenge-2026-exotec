package src

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// 5x4 test grid:
//
//	.....   row 0
//	.#.#.   row 1
//	.....   row 2
//	#####   row 3
var testRows = []string{
	".....",
	".#.#.",
	".....",
	"#####",
}

const testW, testH = 5, 4

func testGrid() *Grid {
	return NewGrid(testW, testH, testRows)
}

func TestIdx(t *testing.T) {
	g := testGrid()
	tests := []struct {
		x, y int
		want int
	}{
		{0, 0, 0},
		{4, 0, 4},
		{0, 1, 5},
		{2, 1, 7},
		{4, 3, 19},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, g.Idx(tt.x, tt.y), "Idx(%d,%d)", tt.x, tt.y)
	}
}

func TestXY(t *testing.T) {
	g := testGrid()
	tests := []struct {
		idx   int
		wantX int
		wantY int
	}{
		{0, 0, 0},
		{4, 4, 0},
		{5, 0, 1},
		{7, 2, 1},
		{19, 4, 3},
	}
	for _, tt := range tests {
		x, y := g.XY(tt.idx)
		assert.Equal(t, tt.wantX, x, "XY(%d) x", tt.idx)
		assert.Equal(t, tt.wantY, y, "XY(%d) y", tt.idx)
	}
}

func TestIdxXYRoundtrip(t *testing.T) {
	g := testGrid()
	for y := 0; y < testH; y++ {
		for x := 0; x < testW; x++ {
			rx, ry := g.XY(g.Idx(x, y))
			assert.Equal(t, x, rx)
			assert.Equal(t, y, ry)
		}
	}
}

func TestIsWall(t *testing.T) {
	g := testGrid()
	tests := []struct {
		x, y int
		want bool
	}{
		{0, 0, false},
		{1, 1, true},  // '#'
		{3, 1, true},  // '#'
		{2, 1, false}, // '.' between walls
		{0, 3, true},  // bottom row all walls
		{4, 3, true},
		{4, 2, false},
	}
	for _, tt := range tests {
		idx := g.Idx(tt.x, tt.y)
		assert.Equal(t, tt.want, g.IsWall(idx), "IsWall(%d,%d)", tt.x, tt.y)
	}
}

func TestIsWallAllFree(t *testing.T) {
	g := NewGrid(3, 2, []string{"...", "..."})
	for i := 0; i < 6; i++ {
		assert.False(t, g.IsWall(i), "idx %d", i)
	}
}

func TestIsWallAllBlocked(t *testing.T) {
	g := NewGrid(3, 2, []string{"###", "###"})
	for i := 0; i < 6; i++ {
		assert.True(t, g.IsWall(i), "idx %d", i)
	}
}

func TestNeighborsCenter(t *testing.T) {
	// 3x3 all open
	g := NewGrid(3, 3, []string{"...", "...", "..."})
	// center cell (1,1) idx=4
	nb := g.Nbs(4)
	assert.Equal(t, 1, nb[DU]) // (1,0)
	assert.Equal(t, 5, nb[DR]) // (2,1)
	assert.Equal(t, 7, nb[DD]) // (1,2)
	assert.Equal(t, 3, nb[DL]) // (0,1)
}

func TestNeighborsCorners(t *testing.T) {
	g := NewGrid(3, 3, []string{"...", "...", "..."})
	tests := []struct {
		name string
		x, y int
		want [4]int // DU, DR, DD, DL
	}{
		{"top-left", 0, 0, [4]int{-1, 1, 3, -1}},
		{"top-right", 2, 0, [4]int{-1, -1, 5, 1}},
		{"bot-left", 0, 2, [4]int{3, 7, -1, -1}},
		{"bot-right", 2, 2, [4]int{5, -1, -1, 7}},
	}
	for _, tt := range tests {
		nb := g.Nbs(g.Idx(tt.x, tt.y))
		assert.Equal(t, tt.want, *nb, tt.name)
	}
}

func TestNeighborsBlockedByWall(t *testing.T) {
	// .#.
	// ...
	g := NewGrid(3, 2, []string{".#.", "..."})
	// (0,0) right neighbor is wall
	nb := g.Nbs(g.Idx(0, 0))
	assert.Equal(t, -1, nb[DR], "right blocked by wall")
	assert.Equal(t, 3, nb[DD], "down open")

	// (2,0) left neighbor is wall
	nb = g.Nbs(g.Idx(2, 0))
	assert.Equal(t, -1, nb[DL], "left blocked by wall")

	// wall cell itself has all -1
	nb = g.Nbs(g.Idx(1, 0))
	assert.Equal(t, [4]int{-1, -1, -1, -1}, *nb)
}

func TestNeighborsWallCellAllBlocked(t *testing.T) {
	g := testGrid()
	// bottom row is all walls
	for x := 0; x < testW; x++ {
		nb := g.Nbs(g.Idx(x, 3))
		assert.Equal(t, [4]int{-1, -1, -1, -1}, *nb, "wall cell (%d,3)", x)
	}
}

func TestNewGridDimensions(t *testing.T) {
	tests := []struct {
		w, h int
		rows []string
	}{
		{1, 1, []string{"."}},
		{2, 2, []string{"..", ".."}},
		{MaxW, 1, []string{makeRow(MaxW, '.')}},
		{1, MaxH, makeRows(1, MaxH, '.')},
	}
	for _, tt := range tests {
		g := NewGrid(tt.w, tt.h, tt.rows)
		assert.Equal(t, tt.w, g.W)
		assert.Equal(t, tt.h, g.H)
	}
}

func TestDirDelta(t *testing.T) {
	assert.Equal(t, [2]int{0, -1}, DirDelta[DU])
	assert.Equal(t, [2]int{1, 0}, DirDelta[DR])
	assert.Equal(t, [2]int{0, 1}, DirDelta[DD])
	assert.Equal(t, [2]int{-1, 0}, DirDelta[DL])
}

func TestDirName(t *testing.T) {
	assert.Equal(t, "UP", DirName[DU])
	assert.Equal(t, "RIGHT", DirName[DR])
	assert.Equal(t, "DOWN", DirName[DD])
	assert.Equal(t, "LEFT", DirName[DL])
}

// helpers

func makeRow(w int, ch byte) string {
	b := make([]byte, w)
	for i := range b {
		b[i] = ch
	}
	return string(b)
}

func makeRows(w, h int, ch byte) []string {
	row := makeRow(w, ch)
	rows := make([]string, h)
	for i := range rows {
		rows[i] = row
	}
	return rows
}
