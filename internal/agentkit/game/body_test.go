package game

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBodySetAndSlice(t *testing.T) {
	body := NewBody([]Point{
		{X: 3, Y: 1},
		{X: 3, Y: 2},
		{X: 3, Y: 3},
	})

	assert.Equal(t, 3, body.Len)

	got := body.Slice()
	want := []Point{
		{X: 3, Y: 1},
		{X: 3, Y: 2},
		{X: 3, Y: 3},
	}
	assert.Equal(t, want, got)
}

func TestBodyHeadTailFacingContains(t *testing.T) {
	body := NewBody([]Point{
		{X: 5, Y: 4},
		{X: 5, Y: 5},
		{X: 5, Y: 6},
	})

	head, ok := BodyHead(&body)
	require.True(t, ok)
	assert.Equal(t, Point{X: 5, Y: 4}, head)

	tail, ok := BodyTail(&body)
	require.True(t, ok)
	assert.Equal(t, Point{X: 5, Y: 6}, tail)

	assert.Equal(t, DirUp, body.Facing())

	assert.True(t, body.Contains(Point{X: 5, Y: 5}))
	assert.False(t, body.Contains(Point{X: 6, Y: 5}))
}

func TestBodyCopyAndReset(t *testing.T) {
	src := NewBody([]Point{
		{X: 0, Y: 0},
		{X: 1, Y: 0},
	})

	var dst Body
	BodyCopy(&dst, &src)

	assert.Equal(t, src.Len, dst.Len)
	assert.Equal(t, src.Slice()[1], dst.Slice()[1])

	src.Parts[0] = Point{X: 9, Y: 9}
	assert.NotEqual(t, src.Slice()[0], dst.Slice()[0], "Copy() should copy values, not alias source")

	BodyReset(&dst)
	assert.Zero(t, dst.Len)
	_, ok := BodyHead(&dst)
	assert.False(t, ok, "Head() on empty body should be missing")
	assert.Equal(t, DirNone, dst.Facing())
}
