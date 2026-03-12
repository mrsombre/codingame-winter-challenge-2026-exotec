package agentkit

import "testing"

// --- Point ------------------------------------------------------------------

func TestManhattanDistance(t *testing.T) {
	got := ManhattanDistance(Point{X: 1, Y: 2}, Point{X: 4, Y: -3})
	if got != 8 {
		t.Fatalf("ManhattanDistance() = %d, want 8", got)
	}
}

// --- BitGrid ----------------------------------------------------------------

func TestBitGrid(t *testing.T) {
	grid := NewBitGrid(5, 4)
	p := Point{X: 2, Y: 3}

	if grid.Has(p) {
		t.Fatalf("new grid should be empty")
	}

	grid.Set(p)
	if !grid.Has(p) {
		t.Fatalf("Set() did not mark point")
	}

	grid.Clear(p)
	if grid.Has(p) {
		t.Fatalf("Clear() did not remove point")
	}

	grid.Set(Point{X: 0, Y: 0})
	grid.Reset()
	if grid.Has(Point{X: 0, Y: 0}) {
		t.Fatalf("Reset() did not clear bits")
	}
}

func BenchmarkBitGridHas(b *testing.B) {
	grid := NewBitGrid(45, 30)
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
	grid := NewBitGrid(45, 30)
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
	grid := NewBitGrid(45, 30)
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

// --- ArenaGrid --------------------------------------------------------------

func TestArenaGridWallBelow(t *testing.T) {
	grid := NewArenaGrid(4, 4, map[Point]bool{
		{X: 1, Y: 1}: true,
		{X: 2, Y: 2}: true,
		{X: 1, Y: 3}: true,
	})

	if !grid.WallBelow(Point{X: 1, Y: 2}) {
		t.Fatalf("WallBelow() = false, want true")
	}
	if grid.WallBelow(Point{X: 0, Y: 1}) {
		t.Fatalf("WallBelow() = true, want false")
	}
}

func TestAppleDistanceField(t *testing.T) {
	grid := NewArenaGrid(5, 4, map[Point]bool{
		{X: 2, Y: 1}: true,
	})
	apples := NewBitGrid(5, 4)
	apples.Set(Point{X: 4, Y: 0})
	apples.Set(Point{X: 0, Y: 3})

	field := grid.AppleDistanceField(&apples)

	tests := map[Point]int{
		{X: 4, Y: 0}: 0,
		{X: 3, Y: 0}: 1,
		{X: 4, Y: 3}: 3,
		{X: 1, Y: 1}: 3,
	}
	for p, want := range tests {
		if got := field.At(p); got != want {
			t.Fatalf("distance at %+v = %d, want %d", p, got, want)
		}
	}

	if got := field.At(Point{X: 2, Y: 1}); got != UnreachableDistance {
		t.Fatalf("wall distance = %d, want %d", got, UnreachableDistance)
	}
}

func TestFloodCount(t *testing.T) {
	grid := NewArenaGrid(5, 4, map[Point]bool{
		{X: 1, Y: 1}: true,
		{X: 2, Y: 1}: true,
	})
	occ := NewBitGrid(5, 4)
	occ.Set(Point{X: 0, Y: 0})
	occ.Set(Point{X: 3, Y: 0})

	if got := grid.FloodCount(Point{X: 0, Y: 0}, &occ, 100); got != 17 {
		t.Fatalf("FloodCount() = %d, want 17", got)
	}
	if got := grid.FloodCount(Point{X: 0, Y: 0}, &occ, 5); got != 5 {
		t.Fatalf("FloodCount() with cap = %d, want 5", got)
	}
}

// --- State ------------------------------------------------------------------

func TestStateValidMoves(t *testing.T) {
	grid := NewArenaGrid(4, 4, map[Point]bool{
		{X: 1, Y: 1}: true,
		{X: 2, Y: 2}: true,
		{X: 1, Y: 3}: true,
	})
	state := NewState(grid)

	moves := state.ValidMoves(Point{X: 1, Y: 2}, DirUp)
	want := []Direction{DirLeft}
	if len(moves) != len(want) {
		t.Fatalf("len(moves) = %d, want %d", len(moves), len(want))
	}
	for i := range want {
		if moves[i] != want[i] {
			t.Fatalf("moves[%d] = %v, want %v", i, moves[i], want[i])
		}
	}
}

func TestStateAppleDistanceField(t *testing.T) {
	grid := NewArenaGrid(5, 4, map[Point]bool{
		{X: 2, Y: 1}: true,
	})
	state := NewState(grid)
	state.Apples.Set(Point{X: 4, Y: 0})
	state.Apples.Set(Point{X: 0, Y: 3})

	field := state.AppleDistanceField()

	if got := field.At(Point{X: 4, Y: 0}); got != 0 {
		t.Fatalf("distance at apple = %d, want 0", got)
	}
	if got := field.At(Point{X: 3, Y: 0}); got != 1 {
		t.Fatalf("distance at neighbor = %d, want 1", got)
	}
}

// --- Benchmarks -------------------------------------------------------------

func BenchmarkAppleDistanceField(b *testing.B) {
	grid := NewArenaGrid(45, 30, map[Point]bool{
		{X: 8, Y: 5}:   true,
		{X: 8, Y: 6}:   true,
		{X: 8, Y: 7}:   true,
		{X: 24, Y: 14}: true,
		{X: 24, Y: 15}: true,
		{X: 24, Y: 16}: true,
	})
	apples := NewBitGrid(45, 30)
	apples.Set(Point{X: 3, Y: 3})
	apples.Set(Point{X: 20, Y: 12})
	apples.Set(Point{X: 39, Y: 26})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		field := grid.AppleDistanceField(&apples)
		if field.At(Point{X: 20, Y: 12}) != 0 {
			b.Fatal("bad distance field")
		}
	}
}

func BenchmarkFloodCount(b *testing.B) {
	grid := NewArenaGrid(45, 30, map[Point]bool{
		{X: 8, Y: 5}:   true,
		{X: 8, Y: 6}:   true,
		{X: 8, Y: 7}:   true,
		{X: 24, Y: 14}: true,
		{X: 24, Y: 15}: true,
		{X: 24, Y: 16}: true,
	})
	occ := NewBitGrid(45, 30)
	occ.Set(Point{X: 10, Y: 10})
	occ.Set(Point{X: 11, Y: 10})
	occ.Set(Point{X: 12, Y: 10})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := grid.FloodCount(Point{X: 10, Y: 10}, &occ, 200); got == 0 {
			b.Fatal("unexpected zero flood count")
		}
	}
}
