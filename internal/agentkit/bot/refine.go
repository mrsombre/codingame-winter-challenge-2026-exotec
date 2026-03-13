package bot

import (
	"time"

	"codingame/internal/agentkit/game"
)

// ---------------------------------------------------------------------------
// Zero-alloc one-turn simulation for refinePlans safety check.
// Mirrors basic/main.go simulateOneTurn but uses fixed-size scratch buffers
// (same pattern as mc.go RollState).
// ---------------------------------------------------------------------------

const MaxBirds = 8

type refBird struct {
	owner  int
	body   game.Body
	facing game.Direction
	alive  bool
}

type OneTurnOutcome struct {
	Losses  [2]int
	Deaths  [2]int
	Trapped [2]int
}

// RefScratch holds all pre-allocated buffers for SimOneTurn + RefinePlans.
// Allocate once (e.g. in State or at top-level), reuse across all calls.
type RefScratch struct {
	birds   [MaxBirds]refBird
	apples  game.BitGrid // init Bits once, Reset per call
	occ     game.BitGrid // occupied
	otherOc game.BitGrid // for occExcept inline

	toBehead  [MaxBirds]bool
	airborne  [MaxBirds]bool
	grounded  [MaxBirds]bool
	newlyGrnd [MaxBirds]int

	eaten   [MaxBirds]game.Point
	nextBuf [game.MaxBody + 1]game.Point // shared move buffer

	enemyDirs [MaxBirds]game.Direction
	ourDirs   [4]game.Direction
	candidate [4]game.Direction
}

func NewRefScratch(w, h int) RefScratch {
	return RefScratch{
		apples:  game.NewBG(w, h),
		occ:     game.NewBG(w, h),
		otherOc: game.NewBG(w, h),
	}
}

// SimOneTurn simulates one game turn with zero heap allocations.
// mine bodies and enemy bodies are copied into scratch birds.
func SimOneTurn(s *game.State, sc *RefScratch,
	mine []MyBotInfo, enemies []EnemyInfo,
	ourDirs []game.Direction, enemyDirs []game.Direction,
	sources []game.Point) OneTurnOutcome {

	g := s.Grid
	nMine := len(mine)
	nEnemy := len(enemies)
	nBirds := nMine + nEnemy

	// --- init birds ---
	for i, mb := range mine {
		b := &sc.birds[i]
		b.owner = 0
		b.body.Set(mb.Body)
		b.facing = b.body.Facing()
		b.alive = true
	}
	for i, enemy := range enemies {
		b := &sc.birds[nMine+i]
		b.owner = 1
		b.body.Set(enemy.Body)
		b.facing = enemy.Facing
		b.alive = true
	}

	// --- apples bitgrid ---
	game.FillBG(&sc.apples, sources)

	// --- move ---
	for i := 0; i < nBirds; i++ {
		b := &sc.birds[i]
		if !b.alive || b.body.Len == 0 {
			continue
		}
		var dir game.Direction
		if i < nMine {
			dir = ourDirs[i]
		} else {
			dir = enemyDirs[i-nMine]
		}
		if dir == game.DirNone {
			dir = b.facing
		}
		if dir == game.DirNone {
			continue
		}

		head := b.body.Parts[0]
		newHead := game.Add(head, game.DirDelta[dir])
		willEat := sc.apples.Has(newHead)

		n := 0
		sc.nextBuf[n] = newHead
		n++
		if willEat {
			copy(sc.nextBuf[n:], b.body.Parts[:b.body.Len])
			n += b.body.Len
		} else {
			copy(sc.nextBuf[n:], b.body.Parts[:b.body.Len-1])
			n += b.body.Len - 1
		}
		b.body.Len = n
		copy(b.body.Parts[:n], sc.nextBuf[:n])
	}

	// --- eat apples ---
	nEaten := 0
	for i := 0; i < nBirds; i++ {
		b := &sc.birds[i]
		if b.alive && b.body.Len > 0 && sc.apples.Has(b.body.Parts[0]) {
			sc.eaten[nEaten] = b.body.Parts[0]
			nEaten++
		}
	}
	for k := 0; k < nEaten; k++ {
		sc.apples.Clear(sc.eaten[k])
	}

	// --- beheadings ---
	var outcome OneTurnOutcome
	for i := 0; i < nBirds; i++ {
		sc.toBehead[i] = false
	}
	for i := 0; i < nBirds; i++ {
		bi := &sc.birds[i]
		if !bi.alive || bi.body.Len == 0 {
			continue
		}
		head := bi.body.Parts[0]
		if g.IsWall(head) {
			sc.toBehead[i] = true
			continue
		}
		for j := 0; j < nBirds; j++ {
			bj := &sc.birds[j]
			if !bj.alive || bj.body.Len == 0 {
				continue
			}
			if !bj.body.Contains(head) {
				continue
			}
			if i != j {
				sc.toBehead[i] = true
				break
			}
			for k := 1; k < bj.body.Len; k++ {
				if bj.body.Parts[k] == head {
					sc.toBehead[i] = true
					break
				}
			}
			if sc.toBehead[i] {
				break
			}
		}
	}
	for i := 0; i < nBirds; i++ {
		if !sc.toBehead[i] {
			continue
		}
		b := &sc.birds[i]
		if b.body.Len <= 3 {
			outcome.Losses[b.owner] += b.body.Len
			outcome.Deaths[b.owner]++
			b.alive = false
			continue
		}
		outcome.Losses[b.owner]++
		copy(b.body.Parts[:b.body.Len-1], b.body.Parts[1:b.body.Len])
		b.body.Len--
	}

	// --- gravity ---
	for i := 0; i < nBirds; i++ {
		sc.airborne[i] = sc.birds[i].alive
		sc.grounded[i] = false
	}
	somethingFell := true
	for somethingFell {
		somethingFell = false
		somethingGotGrounded := true
		for somethingGotGrounded {
			somethingGotGrounded = false
			nNewlyGrnd := 0
			for i := 0; i < nBirds; i++ {
				if !sc.airborne[i] {
					continue
				}
				isGrnd := false
				bi := &sc.birds[i]
				for k := 0; k < bi.body.Len; k++ {
					if isGroundedRef(bi.body.Parts[k], sc.grounded[:nBirds], sc.birds[:nBirds], &sc.apples, g) {
						isGrnd = true
						break
					}
				}
				if isGrnd {
					sc.newlyGrnd[nNewlyGrnd] = i
					nNewlyGrnd++
				}
			}
			if nNewlyGrnd > 0 {
				somethingGotGrounded = true
				for k := 0; k < nNewlyGrnd; k++ {
					idx := sc.newlyGrnd[k]
					sc.grounded[idx] = true
					sc.airborne[idx] = false
				}
			}
		}

		for i := 0; i < nBirds; i++ {
			if !sc.airborne[i] {
				continue
			}
			somethingFell = true
			bi := &sc.birds[i]
			for j := 0; j < bi.body.Len; j++ {
				bi.body.Parts[j].Y++
			}
			allOut := true
			for j := 0; j < bi.body.Len; j++ {
				if bi.body.Parts[j].Y < g.Height+1 {
					allOut = false
					break
				}
			}
			if allOut {
				outcome.Deaths[bi.owner]++
				bi.alive = false
				sc.airborne[i] = false
			}
		}
	}

	// --- update facing ---
	for i := 0; i < nBirds; i++ {
		if sc.birds[i].alive {
			sc.birds[i].facing = sc.birds[i].body.Facing()
		}
	}

	// --- trapped check (our bots only) ---
	sc.occ.Reset()
	for i := 0; i < nBirds; i++ {
		if !sc.birds[i].alive {
			continue
		}
		bi := &sc.birds[i]
		for k := 0; k < bi.body.Len; k++ {
			sc.occ.Set(bi.body.Parts[k])
		}
	}
	for i := 0; i < nMine; i++ {
		bi := &sc.birds[i]
		if !bi.alive || bi.body.Len == 0 {
			continue
		}
		// inline occExcept into sc.otherOc
		copy(sc.otherOc.Bits, sc.occ.Bits)
		for k := 0; k < bi.body.Len; k++ {
			sc.otherOc.Clear(bi.body.Parts[k])
		}
		hasEscape := false
		body := bi.body.Slice()
		for _, dir := range s.VMoves(body[0], bi.facing) {
			_, _, alive, _, _ := s.SimMove(body, bi.facing, dir, &sc.apples, &sc.otherOc)
			if alive {
				hasEscape = true
				break
			}
		}
		if !hasEscape {
			outcome.Trapped[0]++
		}
	}

	return outcome
}

func isGroundedRef(c game.Point, grounded []bool, birds []refBird, apples *game.BitGrid, g *game.AGrid) bool {
	below := game.Point{X: c.X, Y: c.Y + 1}
	if g.WBelow(c) {
		return true
	}
	if apples.Has(below) {
		return true
	}
	for i, ok := range grounded {
		if ok && birds[i].alive && birds[i].body.Contains(below) {
			return true
		}
	}
	return false
}

// OutcomeRisk scores an outcome (higher = worse for us).
func OutcomeRisk(o OneTurnOutcome) int {
	return o.Deaths[0]*100000 + o.Trapped[0]*5000 + o.Losses[0]*100 - o.Deaths[1]*20 - o.Losses[1]
}

// WorstCasePlanRisk enumerates all enemy direction combos and returns max risk.
func WorstCasePlanRisk(s *game.State, sc *RefScratch,
	mine []MyBotInfo, enemies []EnemyInfo,
	sources []game.Point, ourDirs []game.Direction) int {

	if len(enemies) == 0 {
		o := SimOneTurn(s, sc, mine, enemies, ourDirs, nil, sources)
		return OutcomeRisk(o)
	}

	worst := -1
	var walk func(idx int)
	walk = func(idx int) {
		if idx == len(enemies) {
			o := SimOneTurn(s, sc, mine, enemies, ourDirs, sc.enemyDirs[:len(enemies)], sources)
			risk := OutcomeRisk(o)
			if risk > worst {
				worst = risk
			}
			return
		}
		dirs, nd := game.ValidDirs(enemies[idx].Facing)
		for di := 0; di < nd; di++ {
			sc.enemyDirs[idx] = dirs[di]
			walk(idx + 1)
		}
	}
	walk(0)
	return worst
}

// BotPlan holds a bot's current plan for RefinePlans.
type BotPlan struct {
	ID     int
	Body   []game.Point
	Facing game.Direction
	Dir    game.Direction
	Target game.Point
	Reason string
	Ok     bool
}

// RefinePlans tries alternative first-move directions to minimise worst-case risk.
// Updates plans in place. Zero heap allocations in the hot loop.
func RefinePlans(s *game.State, sc *RefScratch,
	mine []MyBotInfo, enemies []EnemyInfo,
	sources []game.Point, plans []BotPlan, deadline time.Time) {

	if len(mine) == 0 || len(enemies) == 0 || time.Until(deadline) < 8*time.Millisecond {
		return
	}

	combos := 1
	for _, enemy := range enemies {
		_, nd := game.ValidDirs(enemy.Facing)
		combos *= nd
		if combos > 128 {
			return
		}
	}

	nPlans := len(plans)
	for i := 0; i < nPlans; i++ {
		sc.ourDirs[i] = plans[i].Dir
	}

	bestRisk := WorstCasePlanRisk(s, sc, mine, enemies, sources, sc.ourDirs[:nPlans])
	for i := 0; i < nPlans; i++ {
		if time.Until(deadline) < 4*time.Millisecond {
			break
		}
		currentDir := sc.ourDirs[i]
		dirs, nd := game.ValidDirs(plans[i].Facing)
		for _, dir := range dirs[:nd] {
			if dir == currentDir {
				continue
			}
			// copy ourDirs into candidate (fixed array, no alloc)
			sc.candidate = sc.ourDirs
			sc.candidate[i] = dir
			risk := WorstCasePlanRisk(s, sc, mine, enemies, sources, sc.candidate[:nPlans])
			if risk < bestRisk {
				bestRisk = risk
				sc.ourDirs = sc.candidate
				plans[i].Dir = dir
				plans[i].Target = game.Add(plans[i].Body[0], game.DirDelta[dir])
				plans[i].Reason = "safety"
				plans[i].Ok = true
			}
		}
	}
}
