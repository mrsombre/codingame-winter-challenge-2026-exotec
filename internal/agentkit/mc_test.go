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
