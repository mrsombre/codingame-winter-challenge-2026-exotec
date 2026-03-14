package bot

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"codingame/internal/agentkit/game"
	"codingame/internal/engine"
)

const mcTestSeed = int64(-4275176072729869300)
const mcTestLeague = 3
const mcTestPlayer = 0 // we are player 0

// buildMCTestState creates a full game state from engine seed for MC perf testing.
func buildMCTestState() (*game.State, []MyBotInfo, []EnemyInfo, []game.Point) {
	eg := engine.NewGame(mcTestSeed, mcTestLeague)
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})

	me := []*engine.Player{p0, p1}[mcTestPlayer]

	// --- Parse global info (same as main() readline loop) ---
	global := engine.SerializeGlobalInfoFor(me, eg)
	var W, H int
	fmt.Sscan(global[1], &W)
	fmt.Sscan(global[2], &H)

	walls := make(map[game.Point]bool)
	for y := 0; y < H; y++ {
		row := global[3+y]
		for x, ch := range row {
			if ch == '#' {
				walls[game.Point{X: x, Y: y}] = true
			}
		}
	}
	ag := game.NewAG(W, H, walls)
	s := game.NewState(ag)

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
	frame := engine.SerializeFrameInfoFor(me, eg)
	var srcN int
	fmt.Sscan(frame[0], &srcN)
	sources := make([]game.Point, srcN)
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

		f := game.DirUp
		if len(pts) >= 2 {
			f = game.FacingPts(pts[0], pts[1])
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

func parseTestBody(s string) []game.Point {
	parts := strings.Split(s, ":")
	pts := make([]game.Point, len(parts))
	for i, p := range parts {
		fmt.Sscanf(p, "%d,%d", &pts[i].X, &pts[i].Y)
	}
	return pts
}

func TestMCPerfInfo(t *testing.T) {
	// Print raw serializer input
	eg := engine.NewGame(mcTestSeed, mcTestLeague)
	p0 := engine.NewPlayer(0)
	p1 := engine.NewPlayer(1)
	eg.Init([]*engine.Player{p0, p1})
	me := []*engine.Player{p0, p1}[mcTestPlayer]
	t.Log("=== Global Info ===")
	for _, line := range engine.SerializeGlobalInfoFor(me, eg) {
		t.Log(line)
	}
	t.Log("=== Frame Info ===")
	for _, line := range engine.SerializeFrameInfoFor(me, eg) {
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
	allOcc := game.NewBG(g.Width, g.Height)
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
		tgt, ok := NearestTarget(g, b.Body[0], sources)
		plans[i] = SearchResult{Dir: game.DirUp, Target: tgt, Ok: ok}
	}

	// --- Single rollout timing ---
	PrecomputeRollAppleDists(g, sources)

	var base RollState
	base.Grid = g
	nMy := len(mine)
	nOpp := len(enemies)
	base.BotCount = nMy + nOpp
	for i, bot := range mine {
		base.Bots[i] = RollBot{ID: bot.ID, Owner: 0, Alive: true, Body: game.NewBody(bot.Body)}
	}
	for i, enemy := range enemies {
		idx := nMy + i
		base.Bots[idx] = RollBot{ID: -1 - i, Owner: 1, Alive: true, Body: game.NewBody(enemy.Body)}
	}
	rbFill(&base.Apples, sources, g.Width, g.Height)
	base.RebuildOcc()

	targets := make([]game.Point, base.BotCount)
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
			var moves [MaxRollBots]game.Direction
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
						moves[k] = game.DirNone
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
				var moves [MaxRollBots]game.Direction
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
							moves[k] = game.DirNone
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
			fmt.Printf("  bot %d: dir=%s (was %s)\n", mine[i].ID, game.DirName[p.Dir], game.DirName[plans[i].Dir])
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
		base.Bots[i] = RollBot{ID: bot.ID, Owner: 0, Alive: true, Body: game.NewBody(bot.Body)}
	}
	for i, enemy := range enemies {
		base.Bots[nMy+i] = RollBot{ID: -1 - i, Owner: 1, Alive: true, Body: game.NewBody(enemy.Body)}
	}
	rbFill(&base.Apples, sources, g.Width, g.Height)
	base.RebuildOcc()

	targets := make([]game.Point, base.BotCount)
	hasTarget := make([]bool, base.BotCount)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var sim RollState
		sim.CopyFrom(&base)
		var moves [MaxRollBots]game.Direction
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
					moves[k] = game.DirNone
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
		base.Bots[i] = RollBot{ID: bot.ID, Owner: 0, Alive: true, Body: game.NewBody(bot.Body)}
	}
	for i, enemy := range enemies {
		base.Bots[nMy+i] = RollBot{ID: -1 - i, Owner: 1, Alive: true, Body: game.NewBody(enemy.Body)}
	}
	rbFill(&base.Apples, sources, g.Width, g.Height)
	base.RebuildOcc()

	targets := make([]game.Point, base.BotCount)
	hasTarget := make([]bool, base.BotCount)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		total := 0
		for v := 0; v < 3; v++ {
			var sim RollState
			sim.CopyFrom(&base)
			var moves [MaxRollBots]game.Direction
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
						moves[k] = game.DirNone
					}
				}
				sim.SimTurn(&moves)
			}
			total += EvalRollState(&sim, 0, targets, hasTarget)
		}
		_ = total
	}
}

func buildCollisionGrid() *game.AGrid {
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
	walls := make(map[game.Point]bool)
	for y, row := range mapRows {
		for x, ch := range row {
			if ch == '#' {
				walls[game.Point{X: x, Y: y}] = true
			}
		}
	}
	return game.NewAG(20, 11, walls)
}

var collisionSources = []game.Point{
	{X: 11, Y: 0}, {X: 8, Y: 0}, {X: 16, Y: 1}, {X: 3, Y: 1}, {X: 4, Y: 3}, {X: 15, Y: 3},
}

func TestMCCollisionAvoidance(t *testing.T) {
	g := buildCollisionGrid()

	t13Mine := []MyBotInfo{
		{ID: 0, Body: []game.Point{{X: 4, Y: 8}, {X: 3, Y: 8}, {X: 3, Y: 9}}},
		{ID: 1, Body: []game.Point{{X: 13, Y: 6}, {X: 12, Y: 6}, {X: 11, Y: 6}, {X: 10, Y: 6}, {X: 10, Y: 7}, {X: 10, Y: 8}, {X: 10, Y: 9}}},
		{ID: 2, Body: []game.Point{{X: 19, Y: 8}, {X: 18, Y: 8}, {X: 18, Y: 9}}},
	}
	t13Enemies := []EnemyInfo{
		{Head: game.Point{X: 18, Y: 7}, Facing: game.DirRight, BodyLen: 6,
			Body: []game.Point{{X: 18, Y: 7}, {X: 17, Y: 7}, {X: 16, Y: 7}, {X: 15, Y: 7}, {X: 15, Y: 8}, {X: 14, Y: 8}}},
		{Head: game.Point{X: 3, Y: 7}, Facing: game.DirRight, BodyLen: 3,
			Body: []game.Point{{X: 3, Y: 7}, {X: 2, Y: 7}, {X: 1, Y: 7}}},
	}

	t.Run("Turn13_SimConfirm", func(t *testing.T) {
		var sim RollState
		sim.Grid = g
		sim.BotCount = 5
		sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: game.NewBody(t13Mine[0].Body)}
		sim.Bots[1] = RollBot{ID: 1, Owner: 0, Alive: true, Body: game.NewBody(t13Mine[1].Body)}
		sim.Bots[2] = RollBot{ID: 2, Owner: 0, Alive: true, Body: game.NewBody(t13Mine[2].Body)}
		sim.Bots[3] = RollBot{ID: -1, Owner: 1, Alive: true, Body: game.NewBody(t13Enemies[0].Body)}
		sim.Bots[4] = RollBot{ID: -2, Owner: 1, Alive: true, Body: game.NewBody(t13Enemies[1].Body)}
		rbFill(&sim.Apples, collisionSources, g.Width, g.Height)
		sim.RebuildOcc()

		var moves [MaxRollBots]game.Direction
		moves[0] = game.DirRight
		moves[1] = game.DirRight
		moves[2] = game.DirUp
		moves[3] = game.DirRight
		moves[4] = game.DirRight
		sim.SimTurn(&moves)

		if sim.Bots[2].Alive {
			t.Fatal("expected bot 2 dead from head-on collision at (19,7)")
		}
		if !sim.Bots[3].Alive {
			t.Fatal("enemy bot 4 (len 6) should survive the collision")
		}
		t.Log("confirmed: bot 2 (len 3) dies vs bot 4 (len 6) at (19,7)")
	})

	t.Run("Turn13_EnemyPrediction", func(t *testing.T) {
		s := game.NewState(g)
		PrecomputeRollAppleDists(g, collisionSources)

		var sim RollState
		sim.Grid = g
		sim.BotCount = 5
		sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: game.NewBody(t13Mine[0].Body)}
		sim.Bots[1] = RollBot{ID: 1, Owner: 0, Alive: true, Body: game.NewBody(t13Mine[1].Body)}
		sim.Bots[2] = RollBot{ID: 2, Owner: 0, Alive: true, Body: game.NewBody(t13Mine[2].Body)}
		sim.Bots[3] = RollBot{ID: -1, Owner: 1, Alive: true, Body: game.NewBody(t13Enemies[0].Body)}
		sim.Bots[4] = RollBot{ID: -2, Owner: 1, Alive: true, Body: game.NewBody(t13Enemies[1].Body)}
		rbFill(&sim.Apples, collisionSources, g.Width, g.Height)
		sim.RebuildOcc()

		rightPredicted := 0
		for v := 0; v < 5; v++ {
			d := RollPolicyDir(&s, &sim, 3, v)
			t.Logf("variant %d: enemy bot 4 predicted=%s (actual was RIGHT)", v, game.DirName[d])
			if d == game.DirRight {
				rightPredicted++
			}
		}
		t.Logf("RIGHT predicted in %d/5 variants (need >0 for MCRefine to detect collision)", rightPredicted)
	})

	t.Run("Turn13_MCRefine", func(t *testing.T) {
		s := game.NewState(g)
		plans := []SearchResult{
			{Dir: game.DirRight, Target: game.Point{X: 4, Y: 3}, Ok: true},
			{Dir: game.DirRight, Target: game.Point{X: 15, Y: 3}, Ok: true},
			{Dir: game.DirUp, Target: game.Point{X: 15, Y: 3}, Ok: true},
		}

		allOcc := game.NewBG(g.Width, g.Height)
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
			t.Logf("bot %d: %s -> %s%s", t13Mine[i].ID, game.DirName[origPlans[i].Dir], game.DirName[p.Dir], changed)
		}
		if plans[2].Dir == game.DirUp {
			t.Error("MCRefine did NOT prevent collision: bot 2 still goes UP toward (19,7)")
		} else {
			t.Logf("MCRefine prevented collision: bot 2 changed to %s", game.DirName[plans[2].Dir])
		}
	})

	t16Mine := []MyBotInfo{
		{ID: 0, Body: []game.Point{{X: 7, Y: 8}, {X: 6, Y: 8}, {X: 5, Y: 8}}},
		{ID: 1, Body: []game.Point{{X: 15, Y: 7}, {X: 14, Y: 7}, {X: 14, Y: 8}, {X: 13, Y: 8}, {X: 12, Y: 8}, {X: 11, Y: 8}, {X: 10, Y: 8}}},
	}
	t16Enemies := []EnemyInfo{
		{Head: game.Point{X: 20, Y: 9}, Facing: game.DirRight, BodyLen: 5,
			Body: []game.Point{{X: 20, Y: 9}, {X: 19, Y: 9}, {X: 18, Y: 9}, {X: 17, Y: 9}, {X: 16, Y: 9}}},
		{Head: game.Point{X: 6, Y: 7}, Facing: game.DirRight, BodyLen: 3,
			Body: []game.Point{{X: 6, Y: 7}, {X: 5, Y: 7}, {X: 4, Y: 7}}},
	}

	t.Run("Turn16_SimConfirm", func(t *testing.T) {
		var sim RollState
		sim.Grid = g
		sim.BotCount = 4
		sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: game.NewBody(t16Mine[0].Body)}
		sim.Bots[1] = RollBot{ID: 1, Owner: 0, Alive: true, Body: game.NewBody(t16Mine[1].Body)}
		sim.Bots[2] = RollBot{ID: -1, Owner: 1, Alive: true, Body: game.NewBody(t16Enemies[0].Body)}
		sim.Bots[3] = RollBot{ID: -2, Owner: 1, Alive: true, Body: game.NewBody(t16Enemies[1].Body)}
		rbFill(&sim.Apples, collisionSources, g.Width, g.Height)
		sim.RebuildOcc()

		var moves [MaxRollBots]game.Direction
		moves[0] = game.DirUp
		moves[1] = game.DirUp
		moves[2] = game.DirRight
		moves[3] = game.DirRight
		sim.SimTurn(&moves)

		if sim.Bots[0].Alive {
			t.Fatal("expected bot 0 dead from head-on collision at (7,7)")
		}
		if sim.Bots[3].Alive {
			t.Fatal("expected enemy bot 5 dead too (equal len 3)")
		}
		t.Log("confirmed: bot 0 and bot 5 both die (equal len 3) at (7,7)")
	})

	t.Run("Turn16_EnemyPrediction", func(t *testing.T) {
		s := game.NewState(g)
		PrecomputeRollAppleDists(g, collisionSources)

		var sim RollState
		sim.Grid = g
		sim.BotCount = 4
		sim.Bots[0] = RollBot{ID: 0, Owner: 0, Alive: true, Body: game.NewBody(t16Mine[0].Body)}
		sim.Bots[1] = RollBot{ID: 1, Owner: 0, Alive: true, Body: game.NewBody(t16Mine[1].Body)}
		sim.Bots[2] = RollBot{ID: -1, Owner: 1, Alive: true, Body: game.NewBody(t16Enemies[0].Body)}
		sim.Bots[3] = RollBot{ID: -2, Owner: 1, Alive: true, Body: game.NewBody(t16Enemies[1].Body)}
		rbFill(&sim.Apples, collisionSources, g.Width, g.Height)
		sim.RebuildOcc()

		rightPredicted := 0
		for v := 0; v < 5; v++ {
			d := RollPolicyDir(&s, &sim, 3, v)
			t.Logf("variant %d: enemy bot 5 predicted=%s (actual was RIGHT)", v, game.DirName[d])
			if d == game.DirRight {
				rightPredicted++
			}
		}
		t.Logf("RIGHT predicted in %d/5 variants (need >0 for MCRefine to detect collision)", rightPredicted)
	})

	t.Run("Turn16_MCRefine", func(t *testing.T) {
		s := game.NewState(g)
		plans := []SearchResult{
			{Dir: game.DirUp, Target: game.Point{X: 4, Y: 3}, Ok: true},
			{Dir: game.DirUp, Target: game.Point{X: 15, Y: 3}, Ok: true},
		}

		allOcc := game.NewBG(g.Width, g.Height)
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
			t.Logf("bot %d: %s -> %s%s", t16Mine[i].ID, game.DirName[origPlans[i].Dir], game.DirName[p.Dir], changed)
		}
		if plans[0].Dir == game.DirUp {
			t.Error("MCRefine did NOT prevent collision: bot 0 still goes UP toward (7,7)")
		} else {
			t.Logf("MCRefine prevented collision: bot 0 changed to %s", game.DirName[plans[0].Dir])
		}
	})
}
