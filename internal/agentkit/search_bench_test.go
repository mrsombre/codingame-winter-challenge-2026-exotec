package agentkit

import (
	"testing"
	"time"
)

// --- Fixtures ---------------------------------------------------------------

// Realistic 32×17 map (seed1001) with bodies, enemies, apples.
func benchSearchState() (State, []Point, []Point, []Point, []EnemyInfo) {
	state := stateFromLayout(seed1001Layout, seed1001Apples)

	// My body: grounded, mid-map, length 5.
	myBody := []Point{
		{X: 14, Y: 14}, {X: 14, Y: 13}, {X: 13, Y: 13},
		{X: 12, Y: 13}, {X: 12, Y: 14},
	}

	// Enemy body: grounded, left side, length 4.
	enemyBody := []Point{
		{X: 5, Y: 14}, {X: 4, Y: 14}, {X: 3, Y: 14}, {X: 3, Y: 15},
	}
	enemies := []EnemyInfo{
		{Head: enemyBody[0], Facing: DirRight, BodyLen: len(enemyBody), Body: enemyBody},
	}

	return state, myBody, enemyBody, seed1001Apples, enemies
}

func benchOcc(state *State, myBody, enemyBody []Point) BitGrid {
	g := state.Grid
	occ := NewBG(g.Width, g.Height)
	for _, p := range myBody {
		occ.Set(p)
	}
	for _, p := range enemyBody {
		occ.Set(p)
	}
	return occ
}

// --- HasSupport -------------------------------------------------------------

func BenchmarkHasSupport_Grounded(b *testing.B) {
	state, myBody, _, _, _ := benchSearchState()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HasSupport(state.Grid, myBody, nil, nil, nil)
	}
}

func BenchmarkHasSupport_Floating(b *testing.B) {
	state, _, _, _, _ := benchSearchState()
	floating := []Point{{X: 10, Y: 2}, {X: 10, Y: 1}, {X: 10, Y: 0}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HasSupport(state.Grid, floating, nil, nil, nil)
	}
}

// --- SimMove ----------------------------------------------------------------

func BenchmarkSimMove_NoEat(b *testing.B) {
	state, myBody, enemyBody, _, _ := benchSearchState()
	occ := benchOcc(&state, myBody, enemyBody)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.SimMove(myBody, DirRight, DirUp, nil, &occ)
	}
}

func BenchmarkSimMove_WithEat(b *testing.B) {
	state, _, _, _, _ := benchSearchState()
	// Body head next to apple at (2,7).
	body := []Point{{X: 3, Y: 7}, {X: 3, Y: 8}, {X: 3, Y: 9}}
	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, seed1001Apples)
	srcBG.Set(Point{X: 2, Y: 7})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.SimMove(body, DirDown, DirLeft, &srcBG, nil)
	}
}

func BenchmarkSimMove_Collision(b *testing.B) {
	state, _, _, _, _ := benchSearchState()
	// Body tries to move into a wall.
	body := []Point{{X: 0, Y: 15}, {X: 1, Y: 15}, {X: 2, Y: 15}, {X: 3, Y: 15}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.SimMove(body, DirRight, DirLeft, nil, nil)
	}
}

// --- StateHash --------------------------------------------------------------

func BenchmarkStateHash_Short(b *testing.B) {
	body := []Point{{X: 10, Y: 10}, {X: 10, Y: 11}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StateHash(DirUp, body)
	}
}

func BenchmarkStateHash_Long(b *testing.B) {
	body := make([]Point, 20)
	for i := range body {
		body[i] = Point{X: i, Y: 14}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		StateHash(DirRight, body)
	}
}

// --- FloodDist --------------------------------------------------------------

func BenchmarkFloodDist(b *testing.B) {
	state, myBody, enemyBody, _, _ := benchSearchState()
	occ := benchOcc(&state, myBody, enemyBody)
	otherOcc := OccExcept(&occ, myBody)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.FloodDist(myBody[0], &otherOcc)
	}
}

// --- CalcDirInfo ------------------------------------------------------------

func BenchmarkCalcDirInfo(b *testing.B) {
	state, myBody, enemyBody, _, _ := benchSearchState()
	occ := benchOcc(&state, myBody, enemyBody)
	otherOcc := OccExcept(&occ, myBody)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.CalcDirInfo(myBody, DirRight, &otherOcc)
	}
}

// --- CalcEnemyDist ----------------------------------------------------------

func BenchmarkCalcEnemyDist(b *testing.B) {
	state, myBody, enemyBody, _, enemies := benchSearchState()
	occ := benchOcc(&state, myBody, enemyBody)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.CalcEnemyDist(enemies, &occ)
	}
}

// --- FiltSrc ----------------------------------------------------------------

func BenchmarkFiltSrc(b *testing.B) {
	state, myBody, enemyBody, sources, enemies := benchSearchState()
	occ := benchOcc(&state, myBody, enemyBody)
	otherOcc := OccExcept(&occ, myBody)
	_, myDists := state.FloodDist(myBody[0], &otherOcc)
	enemyDists := state.CalcEnemyDist(enemies, &occ)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.FiltSrc(sources, myDists, enemyDists)
	}
}

// --- InstantEat -------------------------------------------------------------

func BenchmarkInstantEat(b *testing.B) {
	state, myBody, _, sources, _ := benchSearchState()
	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, sources)
	occ := NewBG(state.Grid.Width, state.Grid.Height)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.InstantEat(myBody, DirRight, sources, &srcBG, &occ)
	}
}

// --- SupReachMulti ----------------------------------------------------------

func BenchmarkSupReachMulti(b *testing.B) {
	state, myBody, _, _, _ := benchSearchState()
	terrain := state.Terr
	initRun := terrain.BodyInitRun(myBody)
	bodyLen := len(myBody)
	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, seed1001Apples)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.SupReachMulti(myBody[0], initRun, bodyLen, seed1001Apples, &srcBG)
	}
}

// --- SupPathBFS -------------------------------------------------------------

func BenchmarkSupPathBFS_Near(b *testing.B) {
	state, myBody, _, _, _ := benchSearchState()
	terrain := state.Terr
	initRun := terrain.BodyInitRun(myBody)
	target := Point{X: 11, Y: 6}
	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, seed1001Apples)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.SupPathBFS(myBody[0], initRun, target, &srcBG)
	}
}

func BenchmarkSupPathBFS_Far(b *testing.B) {
	state, myBody, _, _, _ := benchSearchState()
	terrain := state.Terr
	initRun := terrain.BodyInitRun(myBody)
	target := Point{X: 4, Y: 1}
	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, seed1001Apples)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		terrain.SupPathBFS(myBody[0], initRun, target, &srcBG)
	}
}

// --- PathBFS ----------------------------------------------------------------

func BenchmarkPathBFS_Depth8(b *testing.B) {
	state, myBody, enemyBody, sources, enemies := benchSearchState()
	occ := benchOcc(&state, myBody, enemyBody)
	otherOcc := OccExcept(&occ, myBody)
	dirInfo := state.CalcDirInfo(myBody, DirRight, &otherOcc)
	enemyDists := state.CalcEnemyDist(enemies, &occ)
	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, sources)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		deadline := time.Now().Add(100 * time.Millisecond)
		state.PathBFS(myBody, DirRight, sources, 8, dirInfo, enemyDists, &srcBG, &otherOcc, deadline)
	}
}

func BenchmarkPathBFS_Depth12(b *testing.B) {
	state, myBody, enemyBody, sources, enemies := benchSearchState()
	occ := benchOcc(&state, myBody, enemyBody)
	otherOcc := OccExcept(&occ, myBody)
	dirInfo := state.CalcDirInfo(myBody, DirRight, &otherOcc)
	enemyDists := state.CalcEnemyDist(enemies, &occ)
	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, sources)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		deadline := time.Now().Add(100 * time.Millisecond)
		state.PathBFS(myBody, DirRight, sources, 12, dirInfo, enemyDists, &srcBG, &otherOcc, deadline)
	}
}

// --- BestAction -------------------------------------------------------------

func BenchmarkBestAction(b *testing.B) {
	state, myBody, enemyBody, sources, enemies := benchSearchState()
	occ := benchOcc(&state, myBody, enemyBody)
	otherOcc := OccExcept(&occ, myBody)
	dirInfo := state.CalcDirInfo(myBody, DirRight, &otherOcc)
	enemyDists := state.CalcEnemyDist(enemies, &occ)
	srcBG := NewBG(state.Grid.Width, state.Grid.Height)
	FillBG(&srcBG, sources)
	danger := NewBG(state.Grid.Width, state.Grid.Height)
	for _, e := range enemies {
		for _, d := range LegalDirs(e.Facing) {
			danger.Set(Add(e.Head, DirDelta[d]))
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		state.BestAction(myBody, DirRight, sources, dirInfo, enemies, enemyDists, &srcBG, &otherOcc, &danger)
	}
}

// --- RollFloodCount ---------------------------------------------------------

func BenchmarkRollFloodCount(b *testing.B) {
	state, myBody, enemyBody, sources, _ := benchSearchState()
	g := state.Grid
	PrecomputeRollAppleDists(g, sources)

	var occ RollBG
	rbFill(&occ, myBody, g.Width, g.Height)
	rbFill(&occ, enemyBody, g.Width, g.Height)
	head := myBody[0]
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RollFloodCount(g, head, &occ, 50)
	}
}

// --- RollPolicyDir ----------------------------------------------------------

func BenchmarkRollPolicyDir(b *testing.B) {
	state, myBody, enemyBody, sources, _ := benchSearchState()
	g := state.Grid
	PrecomputeRollAppleDists(g, sources)

	var sim RollState
	sim.Grid = g
	sim.BotCount = 2
	sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: NewBody(myBody)}
	sim.Bots[1] = RollBot{ID: 1, Owner: 1, Alive: true, Body: NewBody(enemyBody)}
	rbFill(&sim.Apples, sources, g.Width, g.Height)
	sim.RebuildOcc()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RollPolicyDir(&state, &sim, 0, i)
	}
}

// --- EvalRollState ----------------------------------------------------------

func BenchmarkEvalRollState(b *testing.B) {
	state, myBody, enemyBody, sources, _ := benchSearchState()
	g := state.Grid
	PrecomputeRollAppleDists(g, sources)

	var sim RollState
	sim.Grid = g
	sim.BotCount = 2
	sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: NewBody(myBody)}
	sim.Bots[1] = RollBot{ID: 1, Owner: 1, Alive: true, Body: NewBody(enemyBody)}
	rbFill(&sim.Apples, sources, g.Width, g.Height)
	sim.RebuildOcc()

	targets := []Point{sources[0], sources[1]}
	hasTarget := []bool{true, true}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		EvalRollState(&sim, 0, targets, hasTarget)
	}
}

// --- Full per-turn pipeline (opponent main loop equivalent) -----------------

func BenchmarkFullTurnPipeline(b *testing.B) {
	state, myBody, enemyBody, sources, enemies := benchSearchState()
	g := state.Grid

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 1. Build occupied grid
		allOcc := NewBG(g.Width, g.Height)
		for _, p := range myBody {
			allOcc.Set(p)
		}
		for _, p := range enemyBody {
			allOcc.Set(p)
		}

		// 2. Enemy danger zone
		eDanger := NewBG(g.Width, g.Height)
		for _, e := range enemies {
			for _, d := range LegalDirs(e.Facing) {
				eDanger.Set(Add(e.Head, DirDelta[d]))
			}
		}

		// 3. Enemy distances
		enemyDists := state.CalcEnemyDist(enemies, &allOcc)

		// 4. Per-bot planning
		otherOcc := OccExcept(&allOcc, myBody)
		facing := DirRight
		dirInfo := state.CalcDirInfo(myBody, facing, &otherOcc)
		_, myDists := state.FloodDist(myBody[0], &otherOcc)

		// 5. Source filtering + BFS search
		srcBG := NewBG(g.Width, g.Height)
		FillBG(&srcBG, sources)
		competitive := state.FiltSrc(sources, myDists, enemyDists)
		FillBG(&srcBG, competitive)

		plan := state.InstantEat(myBody, facing, competitive, &srcBG, &otherOcc)
		if !plan.Ok {
			deadline := time.Now().Add(45 * time.Millisecond)
			plan = state.PathBFS(myBody, facing, competitive, 8, dirInfo, enemyDists, &srcBG, &otherOcc, deadline)
		}
		if !plan.Ok {
			FillBG(&srcBG, sources)
			state.BestAction(myBody, facing, sources, dirInfo, enemies, enemyDists, &srcBG, &otherOcc, &eDanger)
		}
	}
}
