package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// --- Point ------------------------------------------------------------------

func TestMDist(t *testing.T) {
	got := MDist(Point{X: 1, Y: 2}, Point{X: 4, Y: -3})
	assert.Equal(t, 8, got)
}

// --- BitGrid ----------------------------------------------------------------

func TestBitGrid(t *testing.T) {
	grid := NewBG(5, 4)
	p := Point{X: 2, Y: 3}

	assert.False(t, grid.Has(p), "new grid should be empty")

	grid.Set(p)
	assert.True(t, grid.Has(p), "Set() did not mark point")

	grid.Clear(p)
	assert.False(t, grid.Has(p), "Clear() did not remove point")

	grid.Set(Point{X: 0, Y: 0})
	grid.Reset()
	assert.False(t, grid.Has(Point{X: 0, Y: 0}), "Reset() did not clear bits")
}

func BenchmarkBitGridHas(b *testing.B) {
	grid := NewBG(45, 30)
	for y := 0; y < 30; y += 3 {
		for x := 0; x < 45; x += 3 {
			grid.Set(Point{X: x, Y: y})
		}
	}
	points := make([]Point, 0, 45*30)
	for y := 0; y < 30; y++ {
		for x := 0; x < 45; x++ {
			points = append(points, Point{X: x, Y: y})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range points {
			_ = grid.Has(p)
		}
	}
}

func BenchmarkBitGridSet(b *testing.B) {
	grid := NewBG(45, 30)
	points := make([]Point, 0, 45*30)
	for y := 0; y < 30; y++ {
		for x := 0; x < 45; x++ {
			points = append(points, Point{X: x, Y: y})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, p := range points {
			grid.Set(p)
		}
	}
}

func BenchmarkBitGridReset(b *testing.B) {
	grid := NewBG(45, 30)
	for y := 0; y < 30; y += 2 {
		for x := 0; x < 45; x += 2 {
			grid.Set(Point{X: x, Y: y})
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		grid.Reset()
	}
}

// --- AGrid ------------------------------------------------------------------

func TestAGridWBelow(t *testing.T) {
	grid := NewAG(4, 4, map[Point]bool{
		{X: 1, Y: 1}: true,
		{X: 2, Y: 2}: true,
		{X: 1, Y: 3}: true,
	})

	assert.True(t, grid.WBelow(Point{X: 1, Y: 2}))
	assert.False(t, grid.WBelow(Point{X: 0, Y: 1}))
}

// --- State ------------------------------------------------------------------

func TestStateVMoves(t *testing.T) {
	grid := NewAG(4, 4, map[Point]bool{
		{X: 1, Y: 1}: true,
		{X: 2, Y: 2}: true,
		{X: 1, Y: 3}: true,
	})
	state := NewState(grid)

	moves := state.VMoves(Point{X: 1, Y: 2}, DirUp)
	want := []Direction{DirLeft}
	assert.Equal(t, want, moves)
}
