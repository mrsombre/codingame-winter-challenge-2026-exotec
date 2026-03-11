package agentkit

import "testing"

func TestManhattanDistance(t *testing.T) {
	got := ManhattanDistance(Point{X: 1, Y: 2}, Point{X: 4, Y: -3})
	if got != 8 {
		t.Fatalf("ManhattanDistance() = %d, want 8", got)
	}
}

func TestFloodFillWithDist(t *testing.T) {
	grid := NewGrid(5, 4, map[Point]bool{
		{X: 1, Y: 1}: true,
		{X: 2, Y: 1}: true,
		{X: 3, Y: 1}: true,
	})
	blocked := map[Point]bool{
		{X: 1, Y: 2}: true,
	}

	count, dists := grid.FloodFillWithDist(Point{X: 0, Y: 0}, blocked)

	if count != 16 {
		t.Fatalf("reachable count = %d, want 16", count)
	}

	tests := map[Point]int{
		{X: 0, Y: 0}: 0,
		{X: 4, Y: 0}: 4,
		{X: 4, Y: 2}: 6,
		{X: 4, Y: 3}: 7,
	}
	for p, want := range tests {
		if got := dists[p]; got != want {
			t.Fatalf("distance to %+v = %d, want %d", p, got, want)
		}
	}

	if _, ok := dists[Point{X: 1, Y: 1}]; ok {
		t.Fatalf("wall cell should not be reachable")
	}
	if _, ok := dists[Point{X: 1, Y: 2}]; ok {
		t.Fatalf("blocked cell should not be reachable")
	}
}

func TestFloodFillWithDistBlockedStart(t *testing.T) {
	grid := NewGrid(3, 3, nil)

	count, dists := grid.FloodFillWithDist(Point{X: 1, Y: 1}, map[Point]bool{
		{X: 1, Y: 1}: true,
	})

	if count != 0 {
		t.Fatalf("reachable count = %d, want 0", count)
	}
	if len(dists) != 0 {
		t.Fatalf("len(dists) = %d, want 0", len(dists))
	}
}

func BenchmarkFloodFillWithDist(b *testing.B) {
	walls := map[Point]bool{}
	for x := 3; x < 41; x += 6 {
		for y := 2; y < 22; y++ {
			if y == 10 || y == 11 {
				continue
			}
			walls[Point{X: x, Y: y}] = true
		}
	}
	grid := NewGrid(44, 24, walls)
	blocked := map[Point]bool{
		{X: 5, Y: 5}:   true,
		{X: 6, Y: 5}:   true,
		{X: 7, Y: 5}:   true,
		{X: 20, Y: 14}: true,
		{X: 21, Y: 14}: true,
		{X: 22, Y: 14}: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		count, dists := grid.FloodFillWithDist(Point{X: 0, Y: 0}, blocked)
		if count == 0 || len(dists) == 0 {
			b.Fatal("unexpected empty flood fill result")
		}
	}
}
