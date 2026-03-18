package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// Shared 7x7 grid for all sim move tests:
//
//	   0123456
//	0  .......
//	1  .......
//	2  .......
//	3  ..#....
//	4  .......
//	5  .......
//	6  #######
//
// Wall: (2,3). Ground: full row y=6.
func simMoveGrid() (*Game, *Sim) {
	g := testGridInput([]string{
		".......",
		".......",
		".......",
		"..#....",
		".......",
		".......",
		"#######",
	})
	s := NewSim(g)
	return g, s
}

// helper: simulateMove + copy + applyGravity in one step
func fullMove(s *Sim, body []int, d int) ([]int, bool) {
	newBody, alive := s.simulateMove(body, d)
	if !alive {
		return nil, false
	}
	cp := append([]int(nil), newBody...)
	ok := s.applyGravity(cp)
	if !ok {
		return nil, false
	}
	return cp, true
}

// --- simulateMove only (no gravity) ---

func TestSimMoveNormal(t *testing.T) {
	// Snake heading RIGHT moves RIGHT — tail drops
	//  row 5: . t n h . . .   →   . . t n H . .
	g, s := simMoveGrid()

	body := []int{g.Idx(3, 5), g.Idx(2, 5), g.Idx(1, 5)}
	got, alive := s.simulateMove(body, DR)
	got = append([]int(nil), got...)

	assert.True(t, alive)
	assert.Equal(t, 3, len(got))
	assert.Equal(t, g.Idx(4, 5), got[0], "new head at (4,5)")
	assert.Equal(t, g.Idx(3, 5), got[1], "old head becomes neck")
	assert.Equal(t, g.Idx(2, 5), got[2], "old neck, tail (1,5) dropped")
}

func TestSimMoveEatApple(t *testing.T) {
	// Snake heading LEFT eats apple — body grows, tail stays
	//  row 5: A h n t . . .
	g, s := simMoveGrid()

	apple := g.Idx(0, 5)
	s.rebuildAppleMapFrom([]int{apple})

	body := []int{g.Idx(1, 5), g.Idx(2, 5), g.Idx(3, 5)}
	got, alive := s.simulateMove(body, DL)
	got = append([]int(nil), got...)

	assert.True(t, alive)
	assert.Equal(t, 4, len(got), "body grows by 1")
	assert.Equal(t, apple, got[0], "new head on apple")
	assert.Equal(t, g.Idx(1, 5), got[1])
	assert.Equal(t, g.Idx(2, 5), got[2])
	assert.Equal(t, g.Idx(3, 5), got[3], "tail stays when eating")
}

func TestSimMoveHitWallBehead(t *testing.T) {
	// 4-seg snake heading LEFT moves LEFT into wall at (2,3) — beheaded
	//  row 3: . . # h n t b   snake (3,3)(4,3)(5,3)(6,3) → LEFT
	g, s := simMoveGrid()

	body := []int{g.Idx(3, 3), g.Idx(4, 3), g.Idx(5, 3), g.Idx(6, 3)}
	got, alive := s.simulateMove(body, DL)
	got = append([]int(nil), got...)

	assert.True(t, alive, "4-seg survives beheading")
	assert.Equal(t, 3, len(got), "lost head segment")
	assert.Equal(t, g.Idx(3, 3), got[0], "old head becomes new head")
	assert.Equal(t, g.Idx(4, 3), got[1])
	assert.Equal(t, g.Idx(5, 3), got[2])
}

func TestSimMoveHitWallDie(t *testing.T) {
	// 3-seg snake heading LEFT moves LEFT into wall at (2,3) — dies
	//  row 3: . . # h n t .   snake (3,3)(4,3)(5,3) → LEFT
	g, s := simMoveGrid()

	body := []int{g.Idx(3, 3), g.Idx(4, 3), g.Idx(5, 3)}
	_, alive := s.simulateMove(body, DL)

	assert.False(t, alive, "3-seg snake dies on wall hit")
}

func TestSimMoveHitOwnBody(t *testing.T) {
	// 5-seg U-shape: head turns DOWN into own body segment
	//
	//   col: 2 3 4
	//  row4: . h b     h=(3,4) b=(4,4)
	//  row5: t b b     t=(2,5) b=(3,5) b=(4,5)
	//
	//  body: (3,4)(4,4)(4,5)(3,5)(2,5) — move DOWN → new head (3,5) hits body[3]
	g, s := simMoveGrid()

	body := []int{g.Idx(3, 4), g.Idx(4, 4), g.Idx(4, 5), g.Idx(3, 5), g.Idx(2, 5)}
	got, alive := s.simulateMove(body, DD)
	got = append([]int(nil), got...)

	assert.True(t, alive, "5-seg survives self-hit behead")
	assert.Equal(t, 4, len(got), "beheaded: lost head")
	// returns body[1:] of the new body: old segments minus head
	assert.Equal(t, g.Idx(3, 4), got[0], "old head becomes new head")
	assert.Equal(t, g.Idx(4, 4), got[1])
	assert.Equal(t, g.Idx(4, 5), got[2])
	assert.Equal(t, g.Idx(3, 5), got[3])
}

// --- full move (simulateMove + gravity) ---

func TestSimFullMoveGravityFall(t *testing.T) {
	// Vertical snake in air moves RIGHT — head shifts, then everything falls
	//
	//  snake: (0,1)(0,2)(0,3) heading UP, move RIGHT
	//  after move: (1,1)(0,1)(0,2) — no segment grounded
	//  falls until grounded on wall row y=6
	//  result: (1,4)(0,4)(0,5)
	g, s := simMoveGrid()

	body := []int{g.Idx(0, 1), g.Idx(0, 2), g.Idx(0, 3)}
	got, alive := fullMove(s, body, DR)

	assert.True(t, alive)
	assert.Equal(t, 3, len(got))
	// tail should be at y=5 (above wall at y=6)
	_, tailY := g.XY(got[2])
	assert.Equal(t, 5, tailY, "tail on ground")
	// head should have fallen from y=1 to y=4
	hx, hy := g.XY(got[0])
	assert.Equal(t, 1, hx, "head x shifted right")
	assert.Equal(t, 4, hy, "head fell to y=4")
}

func TestSimFullMoveDiagonalFall(t *testing.T) {
	// Vertical snake with tail on ground moves LEFT — diagonal fall
	//
	//  snake: (4,3)(4,4)(4,5) heading UP, move LEFT
	//  after move: (3,3)(4,3)(4,4) — no segment at y=5, falls 1 row
	//  result: (3,4)(4,4)(4,5) — grounded via (4,5) above wall
	g, s := simMoveGrid()

	body := []int{g.Idx(4, 3), g.Idx(4, 4), g.Idx(4, 5)}
	got, alive := fullMove(s, body, DL)

	assert.True(t, alive)
	assert.Equal(t, 3, len(got))

	hx, hy := g.XY(got[0])
	assert.Equal(t, 3, hx, "head moved left")
	assert.Equal(t, 4, hy, "head fell diagonally: x-1, y+1")

	_, tailY := g.XY(got[2])
	assert.Equal(t, 5, tailY, "tail back on ground")
}

func TestSimFullMoveUPNoop(t *testing.T) {
	// Vertical snake with tail on ground moves UP — falls back to same position
	//
	//  snake: (4,3)(4,4)(4,5) heading UP, move UP
	//  after move: (4,2)(4,3)(4,4) — lost tail anchor at y=5, falls
	//  result: (4,3)(4,4)(4,5) — identical to start
	g, s := simMoveGrid()

	body := []int{g.Idx(4, 3), g.Idx(4, 4), g.Idx(4, 5)}
	got, alive := fullMove(s, body, DU)

	assert.True(t, alive)
	assert.Equal(t, 3, len(got))
	assert.Equal(t, body, got, "UP is a noop: snake returns to same position")
}

func TestSimFullMoveClockwiseRotation(t *testing.T) {
	// 4-seg L-shape on ground rotates clockwise:
	// head moves DOWN into tail cell — tail vacates, no self-hit
	//
	//  before:          after:
	//  row4: . . . b H    row4: . . . b b
	//  row5: . . . b t    row5: . . . t H
	//
	//  body: (4,4)(3,4)(3,5)(4,5) dir=DR, move DD
	//  new head (4,5) = old tail cell, tail drops
	//  result: (4,5)(4,4)(3,4)(3,5) — rotated 90° CW
	g, s := simMoveGrid()

	body := []int{g.Idx(4, 4), g.Idx(3, 4), g.Idx(3, 5), g.Idx(4, 5)}
	got, alive := fullMove(s, body, DD)

	assert.True(t, alive)
	assert.Equal(t, 4, len(got))
	assert.Equal(t, g.Idx(4, 5), got[0], "head at old tail position")
	assert.Equal(t, g.Idx(4, 4), got[1])
	assert.Equal(t, g.Idx(3, 4), got[2])
	assert.Equal(t, g.Idx(3, 5), got[3])
}

func TestSimFullMoveWalkOnSurface(t *testing.T) {
	// Snake heading RIGHT on ground moves RIGHT — no fall
	//  row 5: . . t n h . .   →   . . . t n H .
	g, s := simMoveGrid()

	body := []int{g.Idx(4, 5), g.Idx(3, 5), g.Idx(2, 5)}
	got, alive := fullMove(s, body, DR)

	assert.True(t, alive)
	assert.Equal(t, []int{g.Idx(5, 5), g.Idx(4, 5), g.Idx(3, 5)}, got,
		"simple walk right on ground")
}

func TestSimFullMoveEatAndStayGrounded(t *testing.T) {
	// Snake heading LEFT on ground eats apple — grows, stays on ground
	//  row 5: A h n t . . .
	g, s := simMoveGrid()

	apple := g.Idx(0, 5)
	s.rebuildAppleMapFrom([]int{apple})

	body := []int{g.Idx(1, 5), g.Idx(2, 5), g.Idx(3, 5)}
	got, alive := fullMove(s, body, DL)

	assert.True(t, alive)
	assert.Equal(t, 4, len(got), "grew by eating")
	assert.Equal(t, apple, got[0], "head on apple cell")
	for i, c := range got {
		_, y := g.XY(c)
		assert.Equal(t, 5, y, "segment %d on ground", i)
	}
}
