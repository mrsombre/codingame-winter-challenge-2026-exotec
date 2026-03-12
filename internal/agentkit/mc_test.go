package agentkit

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"codingame/internal/engine"
)

const mcTestSeed = int64(-4275176072729869300)
const mcTestLeague = 3
const mcTestPlayer = 0 // we are player 0

// buildMCTestState creates a full game state from engine seed for MC perf testing.
// Uses engine serializer to produce exact same input as the arena bot receives,
// then parses it the same way main() does.
func buildMCTestState() (*State, []MyBotInfo, []EnemyInfo, []Point) {
	game := engine.NewGame(mcTestSeed, mcTestLeague)
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	game.Init([]*engine.Player{p0, p1})

	me := []*engine.Player{p0, p1}[mcTestPlayer]

	// --- Parse global info (same as main() readline loop) ---
	global := engine.SerializeGlobalInfoFor(me, game)
	// global[0] = playerIndex, global[1] = width, global[2] = height
	var W, H int
	fmt.Sscan(global[1], &W)
	fmt.Sscan(global[2], &H)

	walls := make(map[Point]bool)
	for y := 0; y < H; y++ {
		row := global[3+y]
		for x, ch := range row {
			if ch == '#' {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}
	ag := NewAG(W, H, walls)
	s := NewState(ag)

	// My bot IDs
	gridEnd := 3 + H
	var botsPerPlayer int
	fmt.Sscan(global[gridEnd], &botsPerPlayer)
	myBots := make(map[int]bool)
	for i := 0; i < botsPerPlayer; i++ {
		var id int
		fmt.Sscan(global[gridEnd+1+i], &id)
		myBots[id] = true
	}

	// --- Parse frame info (same as main() turn loop) ---
	frame := engine.SerializeFrameInfoFor(me, game)
	var srcN int
	fmt.Sscan(frame[0], &srcN)
	sources := make([]Point, srcN)
	for i := range sources {
		fmt.Sscan(frame[1+i], &sources[i].X, &sources[i].Y)
		s.Apples.Set(sources[i])
	}

	botStart := 1 + srcN
	var botN int
	fmt.Sscan(frame[botStart], &botN)

	var mine []MyBotInfo
	var enemies []EnemyInfo

	for i := 0; i < botN; i++ {
		line := frame[botStart+1+i]
		sp := strings.IndexByte(line, ' ')
		var id int
		fmt.Sscan(line[:sp], &id)
		pts := parseTestBody(line[sp+1:])

		f := DirUp
		if len(pts) >= 2 {
			f = FacingPts(pts[0], pts[1])
		}
		if myBots[id] {
			mine = append(mine, MyBotInfo{ID: id, Body: pts})
		} else {
			enemies = append(enemies, EnemyInfo{
				Head:    pts[0],
				Facing:  f,
				BodyLen: len(pts),
				Body:    pts,
			})
		}
	}

	return &s, mine, enemies, sources
}

func parseTestBody(s string) []Point {
	parts := strings.Split(s, ":")
	pts := make([]Point, len(parts))
	for i, p := range parts {
		fmt.Sscanf(p, "%d,%d", &pts[i].X, &pts[i].Y)
	}
	return pts
}

func TestMCPerfInfo(t *testing.T) {
	// Print raw serializer input
	game := engine.NewGame(mcTestSeed, mcTestLeague)
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	game.Init([]*engine.Player{p0, p1})
	me := []*engine.Player{p0, p1}[mcTestPlayer]
	t.Log("=== Global Info ===")
	for _, line := range engine.SerializeGlobalInfoFor(me, game) {
		t.Log(line)
	}
	t.Log("=== Frame Info ===")
	for _, line := range engine.SerializeFrameInfoFor(me, game) {
		t.Log(line)
	}

	s, mine, enemies, sources := buildMCTestState()
	g := s.Grid

	fmt.Printf("map: %dx%d, apples: %d, my bots: %d, enemy bots: %d\n",
		g.Width, g.Height, len(sources), len(mine), len(enemies))
	for i, b := range mine {
		fmt.Printf("  mine[%d] id=%d body=%v\n", i, b.ID, b.Body)
	}
	for i, e := range enemies {
		fmt.Printf("  enemy[%d] head=%v bodyLen=%d\n", i, e.Head, e.BodyLen)
	}

	// Build allOcc
	allOcc := NewBG(g.Width, g.Height)
	for _, b := range mine {
		for _, p := range b.Body {
			allOcc.Set(p)
		}
	}
	for _, e := range enemies {
		for _, p := range e.Body {
			allOcc.Set(p)
		}
	}

	// Dummy greedy plans (all DirUp)
	plans := make([]SearchResult, len(mine))
	for i, b := range mine {
		t, ok := NearestTarget(g, b.Body[0], sources)
		plans[i] = SearchResult{Dir: DirUp, Target: t, Ok: ok}
	}

	// --- Single rollout timing ---
	PrecomputeRollAppleDists(g, sources)

	var base RollState
	base.Grid = g
	nMy := len(mine)
	nOpp := len(enemies)
	base.BotCount = nMy + nOpp
	for i, bot := range mine {
		base.Bots[i] = RollBot{ID: bot.ID, Owner: 0, Alive: true, Body: NewBody(bot.Body)}
	}
	for i, enemy := range enemies {
		idx := nMy + i
		base.Bots[idx] = RollBot{ID: -1 - i, Owner: 1, Alive: true, Body: NewBody(enemy.Body)}
	}
	rbFill(&base.Apples, sources, g.Width, g.Height)
	base.RebuildOcc()

	targets := make([]Point, base.BotCount)
	hasTarget := make([]bool, base.BotCount)
	for i, b := range mine {
		if tt, ok := NearestTarget(g, b.Body[0], sources); ok {
			targets[i] = tt
			hasTarget[i] = true
		}
	}
	for i, e := range enemies {
		if tt, ok := NearestTarget(g, e.Head, sources); ok {
			targets[nMy+i] = tt
			hasTarget[nMy+i] = true
		}
	}

	// Single rollout (MCRolloutDepth turns, 1 variant)
	{
		n := 0
		start := time.Now()
		for time.Since(start) < 45*time.Millisecond {
			var sim RollState
			sim.CopyFrom(&base)
			var moves [MaxRollBots]Direction
			// First move: greedy policy
			for k := 0; k < sim.BotCount; k++ {
				if sim.Bots[k].Alive {
					moves[k] = RollPolicyDir(s, &sim, k, n)
				}
			}
			sim.SimTurn(&moves)
			// Remaining turns
			for step := 1; step < MCRolloutDepth; step++ {
				for k := 0; k < sim.BotCount; k++ {
					if sim.Bots[k].Alive {
						moves[k] = RollPolicyDir(s, &sim, k, step+n*7)
					} else {
						moves[k] = DirNone
					}
				}
				sim.SimTurn(&moves)
			}
			EvalRollState(&sim, 0, targets, hasTarget)
			n++
		}
		elapsed := time.Since(start)
		usPerRoll := float64(elapsed.Microseconds()) / float64(n)
		fmt.Printf("\nsingle rollout (depth %d, %d bots): %d in 45ms = %.1f µs/rollout\n",
			MCRolloutDepth, base.BotCount, n, usPerRoll)
	}

	// MC eval (3 variants averaged)
	{
		numVariants := 3
		n := 0
		start := time.Now()
		for time.Since(start) < 45*time.Millisecond {
			total := 0
			for v := 0; v < numVariants; v++ {
				var sim RollState
				sim.CopyFrom(&base)
				var moves [MaxRollBots]Direction
				for k := 0; k < sim.BotCount; k++ {
					if sim.Bots[k].Alive {
						moves[k] = RollPolicyDir(s, &sim, k, v)
					}
				}
				sim.SimTurn(&moves)
				for step := 1; step < MCRolloutDepth; step++ {
					for k := 0; k < sim.BotCount; k++ {
						if sim.Bots[k].Alive {
							moves[k] = RollPolicyDir(s, &sim, k, step+v*7)
						} else {
							moves[k] = DirNone
						}
					}
					sim.SimTurn(&moves)
				}
				total += EvalRollState(&sim, 0, targets, hasTarget)
			}
			_ = total
			n++
		}
		elapsed := time.Since(start)
		usPerEval := float64(elapsed.Microseconds()) / float64(n)
		fmt.Printf("MC eval (%d variants, depth %d): %d evals in 45ms = %.1f µs/eval\n",
			numVariants, MCRolloutDepth, n, usPerEval)
	}

	// Full MCRefine call
	{
		start := time.Now()
		plansCopy := make([]SearchResult, len(plans))
		copy(plansCopy, plans)
		deadline := start.Add(45 * time.Millisecond)
		MCRefine(s, mine, enemies, sources, plansCopy, &allOcc, deadline)
		elapsed := time.Since(start)
		fmt.Printf("\nMCRefine total: %v\n", elapsed)
		for i, p := range plansCopy {
			fmt.Printf("  bot %d: dir=%s (was %s)\n", mine[i].ID, DirName[p.Dir], DirName[plans[i].Dir])
		}
	}
}

func BenchmarkSingleRollout(b *testing.B) {
	s, mine, enemies, sources := buildMCTestState()
	g := s.Grid
	PrecomputeRollAppleDists(g, sources)

	var base RollState
	base.Grid = g
	nMy := len(mine)
	base.BotCount = nMy + len(enemies)
	for i, bot := range mine {
		base.Bots[i] = RollBot{ID: bot.ID, Owner: 0, Alive: true, Body: NewBody(bot.Body)}
	}
	for i, enemy := range enemies {
		base.Bots[nMy+i] = RollBot{ID: -1 - i, Owner: 1, Alive: true, Body: NewBody(enemy.Body)}
	}
	rbFill(&base.Apples, sources, g.Width, g.Height)
	base.RebuildOcc()

	targets := make([]Point, base.BotCount)
	hasTarget := make([]bool, base.BotCount)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var sim RollState
		sim.CopyFrom(&base)
		var moves [MaxRollBots]Direction
		for k := 0; k < sim.BotCount; k++ {
			if sim.Bots[k].Alive {
				moves[k] = RollPolicyDir(s, &sim, k, i)
			}
		}
		sim.SimTurn(&moves)
		for step := 1; step < MCRolloutDepth; step++ {
			for k := 0; k < sim.BotCount; k++ {
				if sim.Bots[k].Alive {
					moves[k] = RollPolicyDir(s, &sim, k, step+i*7)
				} else {
					moves[k] = DirNone
				}
			}
			sim.SimTurn(&moves)
		}
		EvalRollState(&sim, 0, targets, hasTarget)
	}
}

func BenchmarkMCEval3Variants(b *testing.B) {
	s, mine, enemies, sources := buildMCTestState()
	g := s.Grid
	PrecomputeRollAppleDists(g, sources)

	var base RollState
	base.Grid = g
	nMy := len(mine)
	base.BotCount = nMy + len(enemies)
	for i, bot := range mine {
		base.Bots[i] = RollBot{ID: bot.ID, Owner: 0, Alive: true, Body: NewBody(bot.Body)}
	}
	for i, enemy := range enemies {
		base.Bots[nMy+i] = RollBot{ID: -1 - i, Owner: 1, Alive: true, Body: NewBody(enemy.Body)}
	}
	rbFill(&base.Apples, sources, g.Width, g.Height)
	base.RebuildOcc()

	targets := make([]Point, base.BotCount)
	hasTarget := make([]bool, base.BotCount)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := 0
		for v := 0; v < 3; v++ {
			var sim RollState
			sim.CopyFrom(&base)
			var moves [MaxRollBots]Direction
			for k := 0; k < sim.BotCount; k++ {
				if sim.Bots[k].Alive {
					moves[k] = RollPolicyDir(s, &sim, k, v)
				}
			}
			sim.SimTurn(&moves)
			for step := 1; step < MCRolloutDepth; step++ {
				for k := 0; k < sim.BotCount; k++ {
					if sim.Bots[k].Alive {
						moves[k] = RollPolicyDir(s, &sim, k, step+v*7)
					} else {
						moves[k] = DirNone
					}
				}
				sim.SimTurn(&moves)
			}
			total += EvalRollState(&sim, 0, targets, hasTarget)
		}
		_ = total
	}
}

// ---------------------------------------------------------------------------
// Collision avoidance analysis: seed=-468706172918629800
// Map: 20x11, walls at row 9 (partial) and row 10 (full)
// Two head-on collision deaths:
//   Turn 13: our bot 2 (len 3) vs enemy bot 4 (len 6) at (19,7) — bot 2 dies
//   Turn 16: our bot 0 (len 3) vs enemy bot 5 (len 3) at (7,7) — both die
// ---------------------------------------------------------------------------

func buildCollisionGrid() *AGrid {
	mapRows := []string{
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....................",
		"....#..#....#..#....",
		"####################",
	}
	walls := make(map[Point]bool)
	for y, row := range mapRows {
		for x, ch := range row {
			if ch == '#' {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}
	return NewAG(20, 11, walls)
}

var collisionSources = []Point{
	{11, 0}, {8, 0}, {16, 1}, {3, 1}, {4, 3}, {15, 3},
}

func TestMCCollisionAvoidance(t *testing.T) {
	g := buildCollisionGrid()

	// ================================================================
	// Turn 13: Bot 2 (ours, len 3) vs Bot 4 (enemy, len 6) at (19,7)
	// Our bot 2 at (19,8) facing RIGHT chose UP → (19,7)
	// Enemy bot 4 at (18,7) facing RIGHT chose RIGHT → (19,7)
	// ================================================================
	t13Mine := []MyBotInfo{
		{ID: 0, Body: []Point{{4, 8}, {3, 8}, {3, 9}}},
		{ID: 1, Body: []Point{{13, 6}, {12, 6}, {11, 6}, {10, 6}, {10, 7}, {10, 8}, {10, 9}}},
		{ID: 2, Body: []Point{{19, 8}, {18, 8}, {18, 9}}},
	}
	t13Enemies := []EnemyInfo{
		{Head: Point{18, 7}, Facing: DirRight, BodyLen: 6,
			Body: []Point{{18, 7}, {17, 7}, {16, 7}, {15, 7}, {15, 8}, {14, 8}}},
		{Head: Point{3, 7}, Facing: DirRight, BodyLen: 3,
			Body: []Point{{3, 7}, {2, 7}, {1, 7}}},
	}

	// Verify simulation catches head-on collision with actual enemy moves
	t.Run("Turn13_SimConfirm", func(t *testing.T) {
		var sim RollState
		sim.Grid = g
		sim.BotCount = 5
		sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: NewBody(t13Mine[0].Body)}
		sim.Bots[1] = RollBot{ID: 1, Owner: 0, Alive: true, Body: NewBody(t13Mine[1].Body)}
		sim.Bots[2] = RollBot{ID: 2, Owner: 0, Alive: true, Body: NewBody(t13Mine[2].Body)}
		sim.Bots[3] = RollBot{ID: -1, Owner: 1, Alive: true, Body: NewBody(t13Enemies[0].Body)}
		sim.Bots[4] = RollBot{ID: -2, Owner: 1, Alive: true, Body: NewBody(t13Enemies[1].Body)}
		rbFill(&sim.Apples, collisionSources, g.Width, g.Height)
		sim.RebuildOcc()

		var moves [MaxRollBots]Direction
		moves[0] = DirRight // bot 0
		moves[1] = DirRight // bot 1
		moves[2] = DirUp    // bot 2 → (19,7) COLLISION
		moves[3] = DirRight // enemy bot 4 → (19,7) COLLISION
		moves[4] = DirRight // enemy bot 5
		sim.SimTurn(&moves)

		if sim.Bots[2].Alive {
			t.Fatal("expected bot 2 dead from head-on collision at (19,7)")
		}
		if !sim.Bots[3].Alive {
			t.Fatal("enemy bot 4 (len 6) should survive the collision")
		}
		t.Log("confirmed: bot 2 (len 3) dies vs bot 4 (len 6) at (19,7)")
	})

	// Show what RollPolicyDir predicts for enemy bot 4 across variants
	t.Run("Turn13_EnemyPrediction", func(t *testing.T) {
		s := NewState(g)
		PrecomputeRollAppleDists(g, collisionSources)

		var sim RollState
		sim.Grid = g
		sim.BotCount = 5
		sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: NewBody(t13Mine[0].Body)}
		sim.Bots[1] = RollBot{ID: 1, Owner: 0, Alive: true, Body: NewBody(t13Mine[1].Body)}
		sim.Bots[2] = RollBot{ID: 2, Owner: 0, Alive: true, Body: NewBody(t13Mine[2].Body)}
		sim.Bots[3] = RollBot{ID: -1, Owner: 1, Alive: true, Body: NewBody(t13Enemies[0].Body)}
		sim.Bots[4] = RollBot{ID: -2, Owner: 1, Alive: true, Body: NewBody(t13Enemies[1].Body)}
		rbFill(&sim.Apples, collisionSources, g.Width, g.Height)
		sim.RebuildOcc()

		rightPredicted := 0
		for v := 0; v < 5; v++ {
			d := RollPolicyDir(&s, &sim, 3, v) // enemy bot 4 (idx 3)
			t.Logf("variant %d: enemy bot 4 predicted=%s (actual was RIGHT)", v, DirName[d])
			if d == DirRight {
				rightPredicted++
			}
		}
		t.Logf("RIGHT predicted in %d/5 variants (need >0 for MCRefine to detect collision)", rightPredicted)
	})

	// Run MCRefine — does it override bot 2's UP?
	t.Run("Turn13_MCRefine", func(t *testing.T) {
		s := NewState(g)
		plans := []SearchResult{
			{Dir: DirRight, Target: Point{4, 3}, Ok: true},
			{Dir: DirRight, Target: Point{15, 3}, Ok: true},
			{Dir: DirUp, Target: Point{15, 3}, Ok: true}, // BAD: leads to collision
		}

		allOcc := NewBG(g.Width, g.Height)
		for _, b := range t13Mine {
			for _, p := range b.Body {
				allOcc.Set(p)
			}
		}
		for _, e := range t13Enemies {
			for _, p := range e.Body {
				allOcc.Set(p)
			}
		}

		origPlans := make([]SearchResult, len(plans))
		copy(origPlans, plans)
		deadline := time.Now().Add(45 * time.Millisecond)
		MCRefine(&s, t13Mine, t13Enemies, collisionSources, plans, &allOcc, deadline)

		for i, p := range plans {
			changed := ""
			if p.Dir != origPlans[i].Dir {
				changed = " CHANGED"
			}
			t.Logf("bot %d: %s -> %s%s", t13Mine[i].ID, DirName[origPlans[i].Dir], DirName[p.Dir], changed)
		}
		if plans[2].Dir == DirUp {
			t.Error("MCRefine did NOT prevent collision: bot 2 still goes UP toward (19,7)")
		} else {
			t.Logf("MCRefine prevented collision: bot 2 changed to %s", DirName[plans[2].Dir])
		}
	})

	// ================================================================
	// Turn 16: Bot 0 (ours, len 3) vs Bot 5 (enemy, len 3) at (7,7)
	// Our bot 0 at (7,8) facing RIGHT chose UP → (7,7)
	// Enemy bot 5 at (6,7) facing RIGHT chose RIGHT → (7,7)
	// ================================================================
	t16Mine := []MyBotInfo{
		{ID: 0, Body: []Point{{7, 8}, {6, 8}, {5, 8}}},
		{ID: 1, Body: []Point{{15, 7}, {14, 7}, {14, 8}, {13, 8}, {12, 8}, {11, 8}, {10, 8}}},
	}
	t16Enemies := []EnemyInfo{
		{Head: Point{20, 9}, Facing: DirRight, BodyLen: 5,
			Body: []Point{{20, 9}, {19, 9}, {18, 9}, {17, 9}, {16, 9}}},
		{Head: Point{6, 7}, Facing: DirRight, BodyLen: 3,
			Body: []Point{{6, 7}, {5, 7}, {4, 7}}},
	}

	// Verify simulation catches mutual kill
	t.Run("Turn16_SimConfirm", func(t *testing.T) {
		var sim RollState
		sim.Grid = g
		sim.BotCount = 4
		sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: NewBody(t16Mine[0].Body)}
		sim.Bots[1] = RollBot{ID: 1, Owner: 0, Alive: true, Body: NewBody(t16Mine[1].Body)}
		sim.Bots[2] = RollBot{ID: -1, Owner: 1, Alive: true, Body: NewBody(t16Enemies[0].Body)}
		sim.Bots[3] = RollBot{ID: -2, Owner: 1, Alive: true, Body: NewBody(t16Enemies[1].Body)}
		rbFill(&sim.Apples, collisionSources, g.Width, g.Height)
		sim.RebuildOcc()

		var moves [MaxRollBots]Direction
		moves[0] = DirUp    // bot 0 → (7,7) COLLISION
		moves[1] = DirUp    // bot 1
		moves[2] = DirRight // enemy bot 4 (off-screen)
		moves[3] = DirRight // enemy bot 5 → (7,7) COLLISION
		sim.SimTurn(&moves)

		if sim.Bots[0].Alive {
			t.Fatal("expected bot 0 dead from head-on collision at (7,7)")
		}
		if sim.Bots[3].Alive {
			t.Fatal("expected enemy bot 5 dead too (equal len 3)")
		}
		t.Log("confirmed: bot 0 and bot 5 both die (equal len 3) at (7,7)")
	})

	// Show what RollPolicyDir predicts for enemy bot 5
	t.Run("Turn16_EnemyPrediction", func(t *testing.T) {
		s := NewState(g)
		PrecomputeRollAppleDists(g, collisionSources)

		var sim RollState
		sim.Grid = g
		sim.BotCount = 4
		sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: NewBody(t16Mine[0].Body)}
		sim.Bots[1] = RollBot{ID: 1, Owner: 0, Alive: true, Body: NewBody(t16Mine[1].Body)}
		sim.Bots[2] = RollBot{ID: -1, Owner: 1, Alive: true, Body: NewBody(t16Enemies[0].Body)}
		sim.Bots[3] = RollBot{ID: -2, Owner: 1, Alive: true, Body: NewBody(t16Enemies[1].Body)}
		rbFill(&sim.Apples, collisionSources, g.Width, g.Height)
		sim.RebuildOcc()

		rightPredicted := 0
		for v := 0; v < 5; v++ {
			d := RollPolicyDir(&s, &sim, 3, v) // enemy bot 5 (idx 3)
			t.Logf("variant %d: enemy bot 5 predicted=%s (actual was RIGHT)", v, DirName[d])
			if d == DirRight {
				rightPredicted++
			}
		}
		t.Logf("RIGHT predicted in %d/5 variants (need >0 for MCRefine to detect collision)", rightPredicted)
	})

	// Run MCRefine — does it override bot 0's UP?
	t.Run("Turn16_MCRefine", func(t *testing.T) {
		s := NewState(g)
		plans := []SearchResult{
			{Dir: DirUp, Target: Point{4, 3}, Ok: true}, // BAD: leads to collision
			{Dir: DirUp, Target: Point{15, 3}, Ok: true},
		}

		allOcc := NewBG(g.Width, g.Height)
		for _, b := range t16Mine {
			for _, p := range b.Body {
				allOcc.Set(p)
			}
		}
		for _, e := range t16Enemies {
			for _, p := range e.Body {
				allOcc.Set(p)
			}
		}

		origPlans := make([]SearchResult, len(plans))
		copy(origPlans, plans)
		deadline := time.Now().Add(45 * time.Millisecond)
		MCRefine(&s, t16Mine, t16Enemies, collisionSources, plans, &allOcc, deadline)

		for i, p := range plans {
			changed := ""
			if p.Dir != origPlans[i].Dir {
				changed = " CHANGED"
			}
			t.Logf("bot %d: %s -> %s%s", t16Mine[i].ID, DirName[origPlans[i].Dir], DirName[p.Dir], changed)
		}
		if plans[0].Dir == DirUp {
			t.Error("MCRefine did NOT prevent collision: bot 0 still goes UP toward (7,7)")
		} else {
			t.Logf("MCRefine prevented collision: bot 0 changed to %s", DirName[plans[0].Dir])
		}
	})
}
