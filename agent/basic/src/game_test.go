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
	return Init(s)
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
	return g
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

	// BFS link invariants
	for _, s := range g.Surfs {
		for _, l := range s.Links {
			// Path length matches Len
			assert.Equal(t, l.Len, len(l.Path)-1,
				"S%d→S%d Len vs Path length", s.ID, l.To)
			// Path starts at an edge cell of the source surface
			p0x, _ := g.XY(l.Path[0])
			assert.True(t, p0x == s.Left || p0x == s.Right,
				"S%d→S%d Path[0] should be edge cell", s.ID, l.To)
			// Path ends at landing cell
			assert.Equal(t, l.Landing, l.Path[len(l.Path)-1],
				"S%d→S%d Path[-1] should be Landing", s.ID, l.To)
			// Landing cell belongs to target surface
			assert.Equal(t, l.To, g.SurfAt[l.Landing],
				"S%d→S%d Landing SurfAt mismatch", s.ID, l.To)
		}
	}

	// Verify links exist (non-zero count)
	linkCount := 0
	for _, s := range g.Surfs {
		linkCount += len(s.Links)
	}
	assert.True(t, linkCount > 0, "should have some links")
}

// --- Apple Surfaces ---

func TestAppleSurfaces(t *testing.T) {
	// Grid layout (7x6):
	//   .......   y=0
	//   .......   y=1  <- cell above apple at (3,2) → apple surface
	//   ...A...   y=2  <- apple at (3,2); cell above wall at (1,3) is (1,2) → no apple surface (already solid)
	//   .#.....   y=3  <- wall at (1,3)
	//   ..A.A..   y=4  <- apples at (2,4) and (4,4); two separate apple surfaces at y=3
	//   #######   y=5  <- all walls → solid surface at y=4 x=0..6
	g := testGridInput([]string{
		".......",
		".......",
		".......",
		".#.....",
		".......",
		"#######",
	})

	// Before apples: solid surfaces only.
	solidCount := len(g.Surfs)
	// (1,2) above wall at (1,3) → solid surface; (0..6,4) above wall row → solid surface
	assert.True(t, solidCount > 0)
	for _, s := range g.Surfs {
		assert.Equal(t, SurfSolid, s.Type)
	}

	// Inject apples manually and init.
	g.Ap = []int{g.Idx(3, 2), g.Idx(2, 4), g.Idx(4, 4)}
	g.ANum = 3

	g.InitAppleSurfaces()

	// Count by type.
	var solid, apple, none int
	for _, s := range g.Surfs {
		switch s.Type {
		case SurfSolid:
			solid++
		case SurfApple:
			apple++
		case SurfNone:
			none++
		}
	}
	assert.Equal(t, solidCount, solid, "solid count unchanged")
	assert.Equal(t, 0, none, "no stale surfaces on first call")
	assert.Equal(t, 3, apple)

	// Apple at (3,2): cell above is (3,1), free → apple surface.
	sid31 := g.SurfAt[g.Idx(3, 1)]
	assert.True(t, sid31 >= 0, "apple surface at (3,1)")
	assert.Equal(t, SurfApple, g.Surfs[sid31].Type)
	assert.Equal(t, 1, g.Surfs[sid31].Len)

	// Apple at (2,4) and (4,4): separate surfaces.
	sid23 := g.SurfAt[g.Idx(2, 3)]
	assert.True(t, sid23 >= 0, "apple surface at (2,3)")
	assert.Equal(t, SurfApple, g.Surfs[sid23].Type)

	sid43 := g.SurfAt[g.Idx(4, 3)]
	assert.True(t, sid43 >= 0, "apple surface at (4,3)")
	assert.Equal(t, SurfApple, g.Surfs[sid43].Type)
	assert.NotEqual(t, sid23, sid43, "two apples → two separate surfaces")

	// Apple surfaces have BFS links.
	assert.True(t, len(g.Surfs[sid31].Links) > 0, "apple surface should have links")

	// --- Second turn: apple at (3,2) eaten, others remain ---
	g.Ap = []int{g.Idx(2, 4), g.Idx(4, 4)}
	g.ANum = 2

	g.UpdateAppleSurfaces()

	// (3,1) surface becomes SurfNone (links kept, type marks it invalid).
	assert.Equal(t, SurfNone, g.Surfs[sid31].Type, "eaten apple → SurfNone")

	// (2,3) and (4,3) still apple.
	assert.Equal(t, SurfApple, g.Surfs[sid23].Type)
	assert.Equal(t, SurfApple, g.Surfs[sid43].Type)

	// Count none.
	var none2 int
	for _, s := range g.Surfs {
		if s.Type == SurfNone {
			none2++
		}
	}
	assert.Equal(t, 1, none2, "1 eaten apple surface")

	// --- Wall above apple: no apple surface created ---
	// (1,2) is already solid. Tested via InitAppleSurfaces — solid not overwritten.
	sid12 := g.SurfAt[g.Idx(1, 2)]
	assert.True(t, sid12 >= 0)
	assert.Equal(t, SurfSolid, g.Surfs[sid12].Type)
}

func TestAppleSurfacesStacked(t *testing.T) {
	g := testGridInput([]string{
		".......",
		".......",
		".......",
		".......",
		".......",
		"#######",
	})

	// Two stacked apples: (3,3) and (3,2) → surfaces at (3,2) and (3,1).
	g.Ap = []int{g.Idx(3, 3), g.Idx(3, 2)}
	g.ANum = 2

	g.InitAppleSurfaces()

	sid32 := g.SurfAt[g.Idx(3, 2)]
	assert.True(t, sid32 >= 0, "apple surface at (3,2)")
	assert.Equal(t, SurfApple, g.Surfs[sid32].Type)

	sid31 := g.SurfAt[g.Idx(3, 1)]
	assert.True(t, sid31 >= 0, "apple surface at (3,1)")
	assert.Equal(t, SurfApple, g.Surfs[sid31].Type)

	assert.NotEqual(t, sid32, sid31, "stacked apples → two separate surfaces")
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
			"single cell",
			"3,2",
			1,
			[]int{g.Idx(3, 2)},
		},
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
		body := g.ParseBody(tt.input)
		assert.Equal(t, tt.wantLen, len(body), tt.name)
		for i, want := range tt.wantIdx {
			assert.Equal(t, want, body[i], "%s body[%d]", tt.name, i)
		}
	}
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
