package agentkit

import "testing"

func TestBodySetAndSlice(t *testing.T) {
	body := NewBody([]Point{
		{X: 3, Y: 1},
		{X: 3, Y: 2},
		{X: 3, Y: 3},
	})

	if body.Len != 3 {
		t.Fatalf("Len = %d, want 3", body.Len)
	}

	got := body.Slice()
	want := []Point{
		{X: 3, Y: 1},
		{X: 3, Y: 2},
		{X: 3, Y: 3},
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Slice()[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestBodyHeadTailFacingContains(t *testing.T) {
	body := NewBody([]Point{
		{X: 5, Y: 4},
		{X: 5, Y: 5},
		{X: 5, Y: 6},
	})

	head, ok := body.Head()
	if !ok || head != (Point{X: 5, Y: 4}) {
		t.Fatalf("Head() = %+v, %v", head, ok)
	}

	tail, ok := body.Tail()
	if !ok || tail != (Point{X: 5, Y: 6}) {
		t.Fatalf("Tail() = %+v, %v", tail, ok)
	}

	if got := body.Facing(); got != DirUp {
		t.Fatalf("Facing() = %v, want %v", got, DirUp)
	}

	if !body.Contains(Point{X: 5, Y: 5}) {
		t.Fatalf("Contains() = false, want true")
	}
	if body.Contains(Point{X: 6, Y: 5}) {
		t.Fatalf("Contains() = true, want false")
	}
}

func TestBodyCopyAndReset(t *testing.T) {
	src := NewBody([]Point{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
	})

	var dst Body
	dst.Copy(src)

	if dst.Len != src.Len {
		t.Fatalf("Len = %d, want %d", dst.Len, src.Len)
	}
	if dst.Slice()[1] != src.Slice()[1] {
		t.Fatalf("copied body mismatch")
	}

	src.Parts[0] = Point{X: 9, Y: 9}
	if dst.Slice()[0] == src.Slice()[0] {
		t.Fatalf("Copy() should copy values, not alias source")
	}

	dst.Reset()
	if dst.Len != 0 {
		t.Fatalf("Len after Reset() = %d, want 0", dst.Len)
	}
	if _, ok := dst.Head(); ok {
		t.Fatalf("Head() on empty body should be missing")
	}
	if got := dst.Facing(); got != DirNone {
		t.Fatalf("Facing() on empty body = %v, want %v", got, DirNone)
	}
}
