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

// --- NewSupportTerrain (init + precompute) ----------------------------------

func BenchmarkNewSupportTerrain_Seed18(b *testing.B) {
	walls := layoutWalls(seed18Layout)
	grid := NewArenaGrid(len(seed18Layout[0]), len(seed18Layout), walls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSupportTerrain(grid)
	}
}

func BenchmarkNewSupportTerrain_Seed1001(b *testing.B) {
	walls := layoutWalls(seed1001Layout)
	grid := NewArenaGrid(len(seed1001Layout[0]), len(seed1001Layout), walls)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewSupportTerrain(grid)
	}
}

// --- MinSoloLengthFromComponentToTarget (O(1) lookup) -----------------------

func BenchmarkMinSoloLength_Seed18(b *testing.B) {
	terrain := supportTerrainFromLayout(seed18Layout)
	body := []Point{{X: 2, Y: 1}, {X: 2, Y: 2}, {X: 2, Y: 3}}
	target := Point{X: 7, Y: 0}
	comp := terrain.AnchorComponent(body)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.MinSoloLengthFromComponentToTarget(comp, target)
	}
}

func BenchmarkMinSoloLength_Seed1001(b *testing.B) {
	terrain := supportTerrainFromLayout(seed1001Layout)
	body := []Point{{X: 14, Y: 13}, {X: 14, Y: 14}, {X: 14, Y: 15}}
	target := Point{X: 11, Y: 6}
	comp := terrain.AnchorComponent(body)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.MinSoloLengthFromComponentToTarget(comp, target)
	}
}

// --- MinSoloLengthFromBodyToTarget (anchor + lookup) ------------------------

func BenchmarkMinSoloLengthFromBody_Seed1001(b *testing.B) {
	terrain := supportTerrainFromLayout(seed1001Layout)
	body := []Point{{X: 14, Y: 13}, {X: 14, Y: 14}, {X: 14, Y: 15}}
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.MinSoloLengthFromBodyToTarget(body, target)
	}
}

// --- ApproachNodeIDs --------------------------------------------------------

func BenchmarkApproachNodeIDs_ColdCache(b *testing.B) {
	walls := layoutWalls(seed1001Layout)
	grid := NewArenaGrid(len(seed1001Layout[0]), len(seed1001Layout), walls)
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t := NewSupportTerrain(grid)
		t.ApproachNodeIDs(target)
	}
}

func BenchmarkApproachNodeIDs_WarmCache(b *testing.B) {
	terrain := supportTerrainFromLayout(seed1001Layout)
	target := Point{X: 11, Y: 6}
	terrain.ApproachNodeIDs(target) // warm
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.ApproachNodeIDs(target)
	}
}

// --- minImmediateLengthFromSupportToTarget (per-call BFS) -------------------

func BenchmarkMinImmediateLength_NearSupport(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terrain
	support := Point{X: 10, Y: 8}
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.minImmediateLengthFromSupportToTarget(support, target, &state.Apples)
	}
}

func BenchmarkMinImmediateLength_FarSupport(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terrain
	support := Point{X: 12, Y: 10}
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.minImmediateLengthFromSupportToTarget(support, target, &state.Apples)
	}
}

// --- targetApproaches (scan + many BFS) -------------------------------------

func BenchmarkTargetApproaches_TopApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terrain
	target := Point{X: 4, Y: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.targetApproaches(&state.Apples, target)
	}
}

func BenchmarkTargetApproaches_MidApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terrain
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.targetApproaches(&state.Apples, target)
	}
}

func BenchmarkTargetApproaches_BottomApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terrain
	target := Point{X: 3, Y: 12}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.targetApproaches(&state.Apples, target)
	}
}

// --- closestSupports (targetApproaches + dedup) -----------------------------

func BenchmarkClosestSupports_TopApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terrain
	target := Point{X: 4, Y: 1}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.closestSupports(&state.Apples, target)
	}
}

func BenchmarkClosestSupports_MidApple(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	terrain := state.Terrain
	target := Point{X: 11, Y: 6}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.closestSupports(&state.Apples, target)
	}
}

// --- RebuildAppleSupports (full per-turn cost) ------------------------------

func BenchmarkRebuildAppleSupports_Seed1001(b *testing.B) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.RebuildAppleSupports()
	}
}

func BenchmarkRebuildAppleSupports_Seed18(b *testing.B) {
	apples := []Point{
		{X: 7, Y: 0}, {X: 10, Y: 0},
		{X: 2, Y: 4}, {X: 15, Y: 4},
		{X: 5, Y: 5}, {X: 12, Y: 5},
	}
	state := stateFromLayout(seed18Layout, apples)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.RebuildAppleSupports()
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
