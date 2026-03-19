package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"testing"

	"codingame/internal/engine"

	"github.com/stretchr/testify/assert"
)

func findAppleLink(s Surface, apple int) (AppleLink, bool) {
	for _, link := range s.Apples {
		if link.Apple == apple {
			return link, true
		}
	}
	return AppleLink{}, false
}

func TestMain(m *testing.M) {
	debug = false
	os.Exit(m.Run())
}

const testSeed int64 = -4514697363674281500
const testLeague = 3

func gameOpts(opts ...int64) (int64, int) {
	if len(opts) == 0 {
		return testSeed, testLeague
	}
	if len(opts) == 1 {
		return opts[0], testLeague
	}
	return opts[0], int(opts[1])
}

// testGame creates a Game initialized from engine-generated grid (Init only, no turn data).
func testGame(opts ...int64) *Game {
	eg := engine.NewGame(gameOpts(opts...))
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})

	lines := engine.SerializeGlobalInfoFor(p0, eg)
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	g := Init(s)
	(&Plan{G: g}).Init()
	return g
}

// testGameFull creates a Game with grid + first turn data (apples + snakes).
func testGameFull(opts ...int64) *Game {
	eg := engine.NewGame(gameOpts(opts...))
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})

	lines := engine.SerializeGlobalInfoFor(p0, eg)
	lines = append(lines, engine.SerializeFrameInfoFor(p0, eg)...)
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	g := Init(s)
	g.Turn(s)
	(&Plan{G: g}).Init()
	return g
}

// testTurnInput feeds turn data lines into an already-initialized Game.
// Lines format: apple count, then apple coords, then snake count, then snake bodies.
// Same format as g.Turn() expects from stdin.
func testTurnInput(g *Game, lines []string) {
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	g.Turn(s)
}

// testGameWithTurn creates a Game from seed + custom turn data, then builds Plan with apples.
func testGameWithTurn(turnLines []string, opts ...int64) (*Game, *Plan) {
	seed, league := gameOpts(opts...)
	eg := engine.NewGame(seed, league)
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})

	lines := engine.SerializeGlobalInfoFor(p0, eg)
	s := bufio.NewScanner(strings.NewReader(strings.Join(lines, "\n")))
	g := Init(s)

	testTurnInput(g, turnLines)

	p := &Plan{G: g}
	p.Init()
	return g, p
}

func testGridInput(lines []string) *Game {
	w := len(lines[0])
	h := len(lines)
	header := []string{
		"0",
		fmt.Sprintf("%d", w),
		fmt.Sprintf("%d", h),
	}
	all := append(header, lines...)
	all = append(all, "0") // snake count

	s := bufio.NewScanner(strings.NewReader(strings.Join(all, "\n")))
	g := Init(s)
	(&Plan{G: g}).Init()
	return g
}

// --- Dl, Dn ---

func TestDirDelta(t *testing.T) {
	assert.Equal(t, [2]int{0, -1}, Dl[DU])
	assert.Equal(t, [2]int{1, 0}, Dl[DR])
	assert.Equal(t, [2]int{0, 1}, Dl[DD])
	assert.Equal(t, [2]int{-1, 0}, Dl[DL])
}

func TestDirName(t *testing.T) {
	assert.Equal(t, "UP", Dn[DU])
	assert.Equal(t, "RIGHT", Dn[DR])
	assert.Equal(t, "DOWN", Dn[DD])
	assert.Equal(t, "LEFT", Dn[DL])
}

// --- Init ---

func TestInit(t *testing.T) {
	g := testGame()

	// grid dimensions: 28x15
	assert.Equal(t, 28, g.W)
	assert.Equal(t, 15, g.H)

	// snake IDs
	assert.Equal(t, 3, g.MyN)
	assert.Equal(t, 0, g.MyIDs[0])
	assert.Equal(t, 1, g.MyIDs[1])
	assert.Equal(t, 2, g.MyIDs[2])
	assert.Equal(t, 3, g.OpN)
	assert.Equal(t, 3, g.OpIDs[0])
	assert.Equal(t, 4, g.OpIDs[1])
	assert.Equal(t, 5, g.OpIDs[2])
}

func TestIsWall(t *testing.T) {
	g := testGridInput([]string{
		"..#",
		"...",
		"#..",
	})

	tests := []struct {
		x, y int
		free bool
		msg  string
	}{
		{0, 0, true, "(0,0) free"},
		{1, 0, true, "(1,0) free"},
		{2, 0, false, "(2,0) wall"},
		{1, 1, true, "(1,1) free"},
		{0, 2, false, "(0,2) wall"},
		{2, 2, true, "(2,2) free"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.free, g.Cell[g.Idx(tt.x, tt.y)], tt.msg)
	}
}

func TestNeighbors(t *testing.T) {
	// 5x4 grid; walls at (1,0) and (3,2)
	g := testGridInput([]string{
		".#...",
		".....",
		"...#.",
		".....",
	})

	tests := []struct {
		name    string
		x, y    int
		wantNb  [4]int // [UP, RIGHT, DOWN, LEFT]
		wantNbm [4]int // valid moves (no walls)
	}{
		// (0,0) corner: right neighbor is wall (1,0)
		{"top-left, wall right", 0, 0,
			[4]int{g.Idx(0, -1), g.Idx(1, 0), g.Idx(0, 1), g.Idx(-1, 0)},
			[4]int{g.Idx(0, -1), -1, g.Idx(0, 1), g.Idx(-1, 0)}},
		// (4,3) corner: all neighbors free (border is free)
		{"bottom-right, all free", 4, 3,
			[4]int{g.Idx(4, 2), g.Idx(5, 3), g.Idx(4, 4), g.Idx(3, 3)},
			[4]int{g.Idx(4, 2), g.Idx(5, 3), g.Idx(4, 4), g.Idx(3, 3)}},
		// (2,0) left neighbor is wall (1,0)
		{"wall to left", 2, 0,
			[4]int{g.Idx(2, -1), g.Idx(3, 0), g.Idx(2, 1), g.Idx(1, 0)},
			[4]int{g.Idx(2, -1), g.Idx(3, 0), g.Idx(2, 1), -1}},
		// (2,1) all neighbors free
		{"interior all free", 2, 1,
			[4]int{g.Idx(2, 0), g.Idx(3, 1), g.Idx(2, 2), g.Idx(1, 1)},
			[4]int{g.Idx(2, 0), g.Idx(3, 1), g.Idx(2, 2), g.Idx(1, 1)}},
		// (2,2) right neighbor is wall (3,2)
		{"wall to right", 2, 2,
			[4]int{g.Idx(2, 1), g.Idx(3, 2), g.Idx(2, 3), g.Idx(1, 2)},
			[4]int{g.Idx(2, 1), -1, g.Idx(2, 3), g.Idx(1, 2)}},
		// (3,1) down neighbor is wall (3,2)
		{"wall below", 3, 1,
			[4]int{g.Idx(3, 0), g.Idx(4, 1), g.Idx(3, 2), g.Idx(2, 1)},
			[4]int{g.Idx(3, 0), g.Idx(4, 1), -1, g.Idx(2, 1)}},
	}
	for _, tt := range tests {
		idx := g.Idx(tt.x, tt.y)
		assert.Equal(t, tt.wantNb, g.Nb[idx], "%s Nb", tt.name)
		assert.Equal(t, tt.wantNbm, g.Nbm[idx], "%s Nbm", tt.name)
	}
}

func TestSurfaces(t *testing.T) {
	g := testGame(-9093555897832026000, 3)

	assert.True(t, len(g.Surfs) > 0, "should detect surfaces")

	// SurfAt consistency: every surface cell maps back to its surface ID
	for id, s := range g.Surfs {
		for x := s.Left; x <= s.Right; x++ {
			cell := g.Idx(x, s.Y)
			assert.Equal(t, id, g.SurfAt[cell], "SurfAt(%d,%d)", x, s.Y)
		}
	}

	assert.Equal(t, 57, len(g.Surfs), "surface count")

	// spot-check surfaces of different lengths
	tests := []struct {
		id                  int
		y, left, right, len int
	}{
		{0, 0, 9, 9, 1},     // single cell above wall at (9,1)
		{29, 10, 7, 8, 2},   // 2-cell ledge above walls at y=11
		{53, 16, 12, 14, 3}, // 3-cell platform above bottom row
		{34, 11, 15, 16, 2}, // 2-cell platform above ## at y=12
		{51, 16, 0, 0, 1},   // single cell bottom-left corner
		{25, 9, 12, 12, 1},  // single cell above wall at (12,10)
	}
	for _, tt := range tests {
		s := g.Surfs[tt.id]
		assert.Equal(t, tt.y, s.Y, "S%d Y", tt.id)
		assert.Equal(t, tt.left, s.Left, "S%d Left", tt.id)
		assert.Equal(t, tt.right, s.Right, "S%d Right", tt.id)
		assert.Equal(t, tt.len, s.Len, "S%d Len", tt.id)
	}
}

// --- Turn ---

func TestTurn(t *testing.T) {
	g := testGameFull(testSeed, int64(testLeague))

	// verify initial state from engine (new 28x15 map has 30 apples)
	assert.Equal(t, 30, g.ANum)
	assert.Equal(t, 6, g.SNum)
	assert.Equal(t, 3, g.Sn[0].Len)
	assert.Equal(t, 3, g.Sn[5].Len)
	assert.Equal(t, g.Idx(20, 9), g.Ap[0])
	assert.Equal(t, g.Idx(18, 11), g.Ap[g.ANum-1])
	assert.Equal(t, 0, g.Sn[0].ID)
	assert.Equal(t, 0, g.Sn[0].Owner)
	assert.Equal(t, g.Idx(16, 10), g.Sn[0].Body[0])
	assert.Equal(t, 5, g.Sn[5].ID)
	assert.Equal(t, 1, g.Sn[5].Owner)
	assert.Equal(t, g.Idx(22, 6), g.Sn[5].Body[0])

	// simulate second turn: snake 0 grew, some apples eaten
	turn2 := strings.Join([]string{
		"2",
		"9 9",
		"18 9",
		"2",
		"0 17,10:16,10:16,11:16,12",
		"3 11,9:11,10:11,11",
	}, "\n")
	g.Turn(bufio.NewScanner(strings.NewReader(turn2)))

	// apples reduced
	assert.Equal(t, 2, g.ANum)
	assert.Equal(t, g.Idx(9, 9), g.Ap[0])
	assert.Equal(t, g.Idx(18, 9), g.Ap[1])

	// snake 0 grew from 3 to 4
	assert.Equal(t, 0, g.Sn[0].ID)
	assert.Equal(t, 0, g.Sn[0].Owner)
	assert.Equal(t, 4, g.Sn[0].Len)
	assert.Equal(t, g.Idx(17, 10), g.Sn[0].Body[0])

	// snake 3 (opponent)
	assert.Equal(t, 3, g.Sn[1].ID)
	assert.Equal(t, 1, g.Sn[1].Owner)
	assert.Equal(t, 3, g.Sn[1].Len)
	assert.Equal(t, g.Idx(11, 9), g.Sn[1].Body[0])
}

// --- ParseBody ---

func TestParseBody(t *testing.T) {
	g := testGridInput([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})

	tests := []struct {
		name    string
		input   string
		wantLen int
		wantIdx []int
	}{
		{
			"horizontal body",
			"0,0:1,0:2,0",
			3,
			[]int{g.Idx(0, 0), g.Idx(1, 0), g.Idx(2, 0)},
		},
		{
			"vertical body",
			"2,1:2,2:2,3:2,4",
			4,
			[]int{g.Idx(2, 1), g.Idx(2, 2), g.Idx(2, 3), g.Idx(2, 4)},
		},
		{
			"L-shaped body",
			"4,0:4,1:3,1",
			3,
			[]int{g.Idx(4, 0), g.Idx(4, 1), g.Idx(3, 1)},
		},
	}
	for _, tt := range tests {
		var sn Snake
		g.ParseBody(&sn, tt.input)
		assert.Equal(t, tt.wantLen, sn.Len, tt.name)
		assert.True(t, sn.Alive, tt.name+" alive")
		for i, want := range tt.wantIdx {
			assert.Equal(t, want, sn.Body[i], "%s body[%d]", tt.name, i)
		}
	}
}

func TestSnakeDir(t *testing.T) {
	g := testGridInput([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})

	tests := []struct {
		name    string
		body    string
		wantDir int
	}{
		{"facing up", "1,0:1,1:1,2", DU},
		{"facing right", "2,2:1,2:0,2", DR},
		{"facing down", "3,3:3,2:3,1", DD},
		{"facing left", "0,1:1,1:2,1", DL},
	}
	for _, tt := range tests {
		var sn Snake
		g.ParseBody(&sn, tt.body)
		assert.Equal(t, tt.wantDir, sn.Dir, "%s dir", tt.name)
		assert.True(t, sn.Alive, "%s alive", tt.name)
		assert.Equal(t, 3, sn.Len, "%s len", tt.name)
	}
}

// --- Sp ---

func buildBodyOf(g *Game) []int {
	bodyOf := g.bodyOf[:g.NCells]
	for i := range bodyOf {
		bodyOf[i] = -1
	}
	for i := 0; i < g.SNum; i++ {
		for _, c := range g.Sn[i].Body {
			if c >= 0 {
				bodyOf[c] = i
			}
		}
	}
	return bodyOf
}

func TestSnakeSp(t *testing.T) {
	// 5x5 grid, wall row at bottom
	g := testGridInput([]string{
		".....",
		".....",
		".....",
		".....",
		"#####",
	})
	g.bodyOf = make([]int, g.NCells)

	// snake on ground: head at (2,3), body going right — head is on wall
	g.SNum = 2
	g.Sn[0] = Snake{ID: 0, Owner: 0, Alive: true,
		Body: []int{g.Idx(2, 3), g.Idx(3, 3), g.Idx(4, 3)}, Len: 3}
	// snake in air: head at (1,0), body going down — tail at (1,2)
	g.Sn[1] = Snake{ID: 1, Owner: 1, Alive: true,
		Body: []int{g.Idx(1, 0), g.Idx(1, 1), g.Idx(1, 2)}, Len: 3}
	g.ANum = 0

	bodyOf := buildBodyOf(g)

	// snake 0: all segments on wall row below (y=4) → Sp=0 (head)
	assert.Equal(t, 0, g.findSp(0, bodyOf), "grounded snake")
	// snake 1: no support below any segment → Sp=-1
	assert.Equal(t, -1, g.findSp(1, bodyOf), "airborne snake")

	// now put an apple below snake 1's tail (1,3)
	g.Ap = []int{g.Idx(1, 3)}
	g.ANum = 1
	assert.Equal(t, 2, g.findSp(1, bodyOf), "apple supports tail")

	// put apple below head instead (1,1) — head wins (index 0 < 2)
	g.Ap = []int{g.Idx(1, 3), g.Idx(1, 1)}
	g.ANum = 2
	assert.Equal(t, 0, g.findSp(1, bodyOf), "apple supports head")

	// clear apples, add another snake below snake 1's mid segment
	g.Ap = g.Ap[:0]
	g.ANum = 0
	g.SNum = 3
	g.Sn[2] = Snake{ID: 2, Owner: 0, Alive: true,
		Body: []int{g.Idx(0, 2), g.Idx(1, 2), g.Idx(2, 2)}, Len: 3}
	bodyOf = buildBodyOf(g)
	// snake 2 body at (1,2) is below snake 1's segment at (1,1) → Sp=1
	assert.Equal(t, 1, g.findSp(1, bodyOf), "other snake supports mid")
}

// --- XY ---

func TestIdxXYRoundtrip(t *testing.T) {
	g := testGridInput([]string{
		"....",
		"....",
		"....",
	})

	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			rx, ry := g.XY(g.Idx(x, y))
			assert.Equal(t, x, rx)
			assert.Equal(t, y, ry)
		}
	}
}

// --- IsInGrid ---

func TestIsInGrid(t *testing.T) {
	g := testGridInput([]string{
		"...",
		"...",
	})

	tests := []struct {
		name string
		cell int
		want bool
	}{
		{"in grid", g.Idx(1, 0), true},
		{"corner", g.Idx(0, 0), true},
		{"border OOB", g.Idx(-1, 0), false},
		{"below grid", g.Idx(0, 2), false},
		{"negative", -1, false},
		{"past end", g.NCells, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, g.IsInGrid(tt.cell), tt.name)
	}
}

// --- IsMy ---

func TestIsMy(t *testing.T) {
	g := testGridInput([]string{
		"...",
		"...",
	})
	g.MyN = 2
	g.MyIDs = [MaxPSn]int{0, 1}
	g.OpN = 2
	g.OpIDs = [MaxPSn]int{2, 3}

	tests := []struct {
		id   int
		want bool
	}{
		{0, true},
		{1, true},
		{2, false},
		{3, false},
		{7, false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, g.IsMy(tt.id), "IsMy(%d)", tt.id)
	}
}

// --- DirFromTo ---

func TestDirFromTo(t *testing.T) {
	g := testGridInput([]string{
		".....",
		".....",
		".....",
		".....",
		".....",
	})

	tests := []struct {
		name           string
		x1, y1, x2, y2 int
		want           int
	}{
		// same cell
		{"same cell", 2, 2, 2, 2, -1},
		// adjacent
		{"adj up", 2, 2, 2, 1, DU},
		{"adj right", 2, 2, 3, 2, DR},
		{"adj down", 2, 2, 2, 3, DD},
		{"adj left", 2, 2, 1, 2, DL},
		// distant, one axis dominant
		{"far up", 1, 4, 1, 0, DU},
		{"far right", 0, 2, 4, 2, DR},
		{"far down", 3, 0, 3, 4, DD},
		{"far left", 4, 1, 0, 1, DL},
		// diagonal, dy > dx → vertical wins
		{"diag dy>dx up", 2, 3, 1, 0, DU},
		{"diag dy>dx down", 1, 0, 2, 3, DD},
		// diagonal, dx > dy → horizontal wins
		{"diag dx>dy right", 0, 1, 3, 2, DR},
		{"diag dx>dy left", 3, 2, 0, 1, DL},
		// tie → vertical wins
		{"tie up-right", 2, 2, 3, 1, DU},
		{"tie down-left", 2, 2, 1, 3, DD},
	}
	for _, tt := range tests {
		a := g.Idx(tt.x1, tt.y1)
		b := g.Idx(tt.x2, tt.y2)
		assert.Equal(t, tt.want, g.DirFromTo(a, b), tt.name)
	}
}

// --- Manhattan ---

func TestManhattan(t *testing.T) {
	g := testGridInput([]string{
		".....",
		".....",
		".....",
		".....",
	})

	tests := []struct {
		name     string
		x1, y1   int
		x2, y2   int
		wantDist int
	}{
		{"same cell", 2, 1, 2, 1, 0},
		{"horizontal", 0, 0, 4, 0, 4},
		{"vertical", 1, 0, 1, 3, 3},
		{"diagonal", 0, 0, 4, 3, 7},
		{"adjacent", 2, 2, 3, 2, 1},
	}
	for _, tt := range tests {
		a := g.Idx(tt.x1, tt.y1)
		b := g.Idx(tt.x2, tt.y2)
		assert.Equal(t, tt.wantDist, g.Manhattan(a, b), tt.name)
		assert.Equal(t, tt.wantDist, g.Manhattan(b, a), tt.name+" reversed")
	}
}
