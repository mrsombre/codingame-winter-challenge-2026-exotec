package agentkit

import "testing"

// --- Fixtures ---------------------------------------------------------------

var seed18Layout = []string{
	"..................",
	"...##........##...",
	"...#....##....#...",
	"...#..........#...",
	"..#............#..",
	"..................",
	".##............##.",
	"...#..........#...",
	"##....######....##",
	"##################",
}

var seed1001Layout = []string{
	".#............................#.",
	".#............................#.",
	"..#..........................#..",
	"...........#........#...........",
	"...........##......##...........",
	"................................",
	".##....#................#....##.",
	".......#......#..#......#.......",
	".........##....##....##.........",
	"#......##..............##......#",
	".##.........##....##.........##.",
	"........##..#......#..##........",
	".#.....####.#......#.####.....#.",
	".#....####..#..##..#..####....#.",
	".#..######..#..##..#..######..#.",
	"#############..##..#############",
	"################################",
}

var seed1001Apples = []Point{
	{X: 4, Y: 1}, {X: 27, Y: 1}, {X: 1, Y: 3}, {X: 30, Y: 3},
	{X: 2, Y: 7}, {X: 29, Y: 7}, {X: 3, Y: 7}, {X: 28, Y: 7},
	{X: 8, Y: 1}, {X: 23, Y: 1}, {X: 5, Y: 5}, {X: 26, Y: 5},
	{X: 11, Y: 6}, {X: 20, Y: 6}, {X: 2, Y: 8}, {X: 29, Y: 8},
	{X: 3, Y: 12}, {X: 28, Y: 12},
}

// --- NewSTerrain (init + precompute) ----------------------------------------

func BenchmarkNewSTerrain_Seed18(b *testing.B) {
	walls := layoutWalls(seed18Layout)
	grid := NewAG(len(seed18Layout[0]), len(seed18Layout), walls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSTerrain(grid)
	}
}

func BenchmarkNewSTerrain_Seed1001(b *testing.B) {
	walls := layoutWalls(seed1001Layout)
	grid := NewAG(len(seed1001Layout[0]), len(seed1001Layout), walls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSTerrain(grid)
	}
}

// --- MinSoloLen (O(1) lookup) -----------------------------------------------

func BenchmarkMinSoloLen_Seed18(b *testing.B) {
	terrain := sTerrainFromLayout(seed18Layout)
	body := []Point{{X: 2, Y: 1}, {X: 2, Y: 2}, {X: 2, Y: 3}}
	target := Point{X: 7, Y: 0}
	comp := terrain.AnchorComp(body)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.MinSoloLen(comp, target)
	}
}

func BenchmarkMinSoloLen_Seed1001(b *testing.B) {
	terrain := sTerrainFromLayout(seed1001Layout)
	body := []Point{{X: 14, Y: 13}, {X: 14, Y: 14}, {X: 14, Y: 15}}
	target := Point{X: 11, Y: 6}
	comp := terrain.AnchorComp(body)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.MinSoloLen(comp, target)
	}
}

// --- MinBodyLen (anchor + lookup) -------------------------------------------

func BenchmarkMinBodyLen_Seed1001(b *testing.B) {
	terrain := sTerrainFromLayout(seed1001Layout)
	body := []Point{{X: 14, Y: 13}, {X: 14, Y: 14}, {X: 14, Y: 15}}
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.MinBodyLen(body, target)
	}
}

// --- ApprNodes --------------------------------------------------------------

func BenchmarkApprNodes_ColdCache(b *testing.B) {
	walls := layoutWalls(seed1001Layout)
	grid := NewAG(len(seed1001Layout[0]), len(seed1001Layout), walls)
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := NewSTerrain(grid)
		t.ApprNodes(target)
	}
}

func BenchmarkApprNodes_WarmCache(b *testing.B) {
	terrain := sTerrainFromLayout(seed1001Layout)
	target := Point{X: 11, Y: 6}
	terrain.ApprNodes(target) // warm
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.ApprNodes(target)
	}
}

// --- minImmLen (per-call BFS) -----------------------------------------------

func BenchmarkMinImmLen_NearSupport(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terr
	support := Point{X: 10, Y: 8}
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.MinImmLen(support, target, &state.Apples)
	}
}

func BenchmarkMinImmLen_FarSupport(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terr
	support := Point{X: 12, Y: 10}
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.MinImmLen(support, target, &state.Apples)
	}
}

// --- tAppr (scan + many BFS) ------------------------------------------------

func BenchmarkTAppr_TopApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terr
	target := Point{X: 4, Y: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.TAppr(&state.Apples, target)
	}
}

func BenchmarkTAppr_MidApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terr
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.TAppr(&state.Apples, target)
	}
}

func BenchmarkTAppr_BottomApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terr
	target := Point{X: 3, Y: 12}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.TAppr(&state.Apples, target)
	}
}

// --- closest (tAppr + dedup) ------------------------------------------------

func BenchmarkClosest_TopApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terr
	target := Point{X: 4, Y: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.Closest(&state.Apples, target)
	}
}

func BenchmarkClosest_MidApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terr
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.Closest(&state.Apples, target)
	}
}

// --- RebuildSup (full per-turn cost) ----------------------------------------

func BenchmarkRebuildSup_Seed1001(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.RebuildSup()
	}
}

func BenchmarkRebuildSup_Seed18(b *testing.B) {
	apples := []Point{
		{X: 7, Y: 0}, {X: 10, Y: 0},
		{X: 2, Y: 4}, {X: 15, Y: 4},
		{X: 5, Y: 5}, {X: 12, Y: 5},
	}
	state := stateFromLayout(seed18Layout, apples)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.RebuildSup()
	}
}

// --- helpers ----------------------------------------------------------------

func layoutWalls(layout []string) map[Point]bool {
	walls := make(map[Point]bool)
	for y, row := range layout {
		for x, ch := range row {
			if ch == '#' {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}
	return walls
}
