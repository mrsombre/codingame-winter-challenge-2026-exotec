package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testMovePlan(layout []string, apples [][2]int) (*Game, *Plan) {
	h := len(layout)
	w := len(layout[0])
	n := w * h

	g := &Game{
		W:       w,
		H:       h,
		OobBase: n,
		NCells:  n + 2*w + 2*h,
		Cell:    make([]bool, n),
		Nb:      make([][4]int, n+2*w+2*h),
		Ap:      make([]int, 0, len(apples)),
		ANum:    len(apples),
	}
	for i := range g.Nb {
		g.Nb[i] = [4]int{-1, -1, -1, -1}
	}

	for y, row := range layout {
		for x := 0; x < w; x++ {
			if row[x] != '#' {
				g.Cell[g.Idx(x, y)] = true
			}
		}
	}

	for cell := 0; cell < g.NCells; cell++ {
		cx, cy := g.CellXY(cell)
		for d := 0; d < 4; d++ {
			nx := cx + Dl[d][0]
			ny := cy + Dl[d][1]
			ni := g.CellIdx(nx, ny)
			if ni < 0 {
				continue
			}
			if ni < g.OobBase && !g.Cell[ni] {
				continue
			}
			g.Nb[cell][d] = ni
		}
	}

	for _, ap := range apples {
		g.Ap = append(g.Ap, g.Idx(ap[0], ap[1]))
	}

	p := &Plan{g: g}
	p.Precompute()
	p.RebuildAppleMap()
	return g, p
}

func TestSimulateMove_WallKillsLengthThree(t *testing.T) {
	_, p := testMovePlan([]string{
		".#.",
		"...",
		"...",
	}, nil)

	body := []int{
		p.g.Idx(0, 0),
		p.g.Idx(0, 1),
		p.g.Idx(0, 2),
	}

	got, alive := p.simulateMove(body, DR)
	assert.False(t, alive)
	assert.Nil(t, got)
}

func TestSimulateMove_WallBeheadsLengthFour(t *testing.T) {
	g, p := testMovePlan([]string{
		".#.",
		"...",
		"...",
		"...",
	}, nil)

	body := []int{
		g.Idx(0, 0),
		g.Idx(0, 1),
		g.Idx(0, 2),
		g.Idx(0, 3),
	}

	got, alive := p.simulateMove(body, DR)
	require.True(t, alive)
	assert.Equal(t, []int{
		g.Idx(0, 0),
		g.Idx(0, 1),
		g.Idx(0, 2),
	}, append([]int(nil), got...))
}

func TestSimulateMove_EnteringOldTailIsSafe(t *testing.T) {
	g, p := testMovePlan([]string{
		"...",
		"...",
		"...",
	}, nil)

	body := []int{
		g.Idx(1, 1),
		g.Idx(1, 2),
		g.Idx(0, 2),
		g.Idx(0, 1),
	}

	got, alive := p.simulateMove(body, DL)
	require.True(t, alive)
	assert.Equal(t, []int{
		g.Idx(0, 1),
		g.Idx(1, 1),
		g.Idx(1, 2),
		g.Idx(0, 2),
	}, append([]int(nil), got...))
}

func TestSimulateMove_ApplePreservesTailBeforeBodyCollision(t *testing.T) {
	g, p := testMovePlan([]string{
		"...",
		"...",
		"...",
	}, [][2]int{{0, 1}})

	body := []int{
		g.Idx(1, 1),
		g.Idx(1, 2),
		g.Idx(0, 2),
		g.Idx(0, 1),
	}

	got, alive := p.simulateMove(body, DL)
	require.True(t, alive)
	assert.Equal(t, body, append([]int(nil), got...))
}
