package agentkit

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

func TestAppleDist(t *testing.T) {
	grid := NewAG(5, 4, map[Point]bool{
		{X: 2, Y: 1}: true,
	})
	apples := NewBG(5, 4)
	apples.Set(Point{X: 4, Y: 0})
	apples.Set(Point{X: 0, Y: 3})

	field := grid.AppleDist(&apples)

	tests := map[Point]int{
		{X: 4, Y: 0}: 0,
		{X: 3, Y: 0}: 1,
		{X: 4, Y: 3}: 3,
		{X: 1, Y: 1}: 3,
	}
	for p, want := range tests {
		assert.Equalf(t, want, field.At(p), "distance at %+v", p)
	}

	assert.Equal(t, Unreachable, field.At(Point{X: 2, Y: 1}))
}

func TestFlood(t *testing.T) {
	grid := NewAG(5, 4, map[Point]bool{
		{X: 1, Y: 1}: true,
		{X: 2, Y: 1}: true,
	})
	occ := NewBG(5, 4)
	occ.Set(Point{X: 0, Y: 0})
	occ.Set(Point{X: 3, Y: 0})

	assert.Equal(t, 17, grid.Flood(Point{X: 0, Y: 0}, &occ, 100))
	assert.Equal(t, 5, grid.Flood(Point{X: 0, Y: 0}, &occ, 5))
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

func TestStateAppleDist(t *testing.T) {
	grid := NewAG(5, 4, map[Point]bool{
		{X: 2, Y: 1}: true,
	})
	state := NewState(grid)
	state.Apples.Set(Point{X: 4, Y: 0})
	state.Apples.Set(Point{X: 0, Y: 3})

	field := state.AppleDist()

	assert.Zero(t, field.At(Point{X: 4, Y: 0}))
	assert.Equal(t, 1, field.At(Point{X: 3, Y: 0}))
}

// --- Benchmarks -------------------------------------------------------------

func BenchmarkAppleDist(b *testing.B) {
	grid := NewAG(45, 30, map[Point]bool{
		{X: 8, Y: 5}:   true,
		{X: 8, Y: 6}:   true,
		{X: 8, Y: 7}:   true,
		{X: 24, Y: 14}: true,
		{X: 24, Y: 15}: true,
		{X: 24, Y: 16}: true,
	})
	apples := NewBG(45, 30)
	apples.Set(Point{X: 3, Y: 3})
	apples.Set(Point{X: 20, Y: 12})
	apples.Set(Point{X: 39, Y: 26})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		field := grid.AppleDist(&apples)
		if field.At(Point{X: 20, Y: 12}) != 0 {
			b.Fatal("bad distance field")
		}
	}
}

func BenchmarkFlood(b *testing.B) {
	grid := NewAG(45, 30, map[Point]bool{
		{X: 8, Y: 5}:   true,
		{X: 8, Y: 6}:   true,
		{X: 8, Y: 7}:   true,
		{X: 24, Y: 14}: true,
		{X: 24, Y: 15}: true,
		{X: 24, Y: 16}: true,
	})
	occ := NewBG(45, 30)
	occ.Set(Point{X: 10, Y: 10})
	occ.Set(Point{X: 11, Y: 10})
	occ.Set(Point{X: 12, Y: 10})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := grid.Flood(Point{X: 10, Y: 10}, &occ, 200); got == 0 {
			b.Fatal("unexpected zero flood count")
		}
	}
}
