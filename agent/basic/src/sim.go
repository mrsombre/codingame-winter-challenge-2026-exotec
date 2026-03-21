package main

import "time"

var simBuf [MaxBody + 1]Point

func hasSupport(body []Point, sources, occupied *BitGrid, eaten *Point) bool {
	for _, part := range body {
		below := Point{X: part.X, Y: part.Y + 1}
		isBody := false
		for _, bp := range body {
			if bp == below {
				isBody = true
				break
			}
		}
		if isBody {
			continue
		}
		if grid.WBelow(part) {
			return true
		}
		if occupied != nil && occupied.Has(below) {
			return true
		}
		if sources != nil && sources.Has(below) && (eaten == nil || below != *eaten) {
			return true
		}
	}
	return false
}

func simMove(body []Point, facing, dir Direction, sources, occupied *BitGrid) ([]Point, Direction, bool, bool, Point) {
	nh := Add(body[0], DirDelta[dir])
	willEat := sources != nil && sources.Has(nh)

	n := 0
	simBuf[n] = nh
	n++
	if willEat {
		copy(simBuf[n:], body)
		n += len(body)
	} else {
		copy(simBuf[n:], body[:len(body)-1])
		n += len(body) - 1
	}

	collision := grid.IsActualWall(nh) || (occupied != nil && occupied.Has(nh))
	if !collision {
		for k := 1; k < n; k++ {
			if simBuf[k] == nh {
				collision = true
				break
			}
		}
	}
	if collision {
		if n <= 3 {
			return nil, DirNone, false, willEat, nh
		}
		copy(simBuf[:], simBuf[1:n])
		n--
	}
	nb := simBuf[:n]

	var eaten *Point
	if willEat {
		eaten = &nh
	}
	for {
		if hasSupport(nb, sources, occupied, eaten) {
			break
		}
		allOut := true
		for i := range nb {
			nb[i].Y++
			if nb[i].Y < H+1 {
				allOut = false
			}
		}
		if allOut {
			return nil, DirNone, false, willEat, nh
		}
	}

	f := DirUp
	if n >= 2 {
		f = FacingPts(nb[0], nb[1])
	}
	return nb, f, true, willEat, nh
}

type refBird struct {
	owner  int
	body   fBody
	facing Direction
	alive  bool
}

type oneTurnOutcome struct {
	losses  [2]int
	deaths  [2]int
	trapped [2]int
}

type refScratch struct {
	birds     [MaxBirds]refBird
	apples    BitGrid
	occ       BitGrid
	otherOc   BitGrid
	toBehead  [MaxBirds]bool
	airborne  [MaxBirds]bool
	grounded  [MaxBirds]bool
	newlyGrnd [MaxBirds]int
	eaten     [MaxBirds]Point
	nextBuf   [MaxBody + 1]Point
	enemyDirs [MaxBirds]Direction
	ourDirs   [4]Direction
	candidate [4]Direction
}

func newRefScratch(w, h int) refScratch {
	return refScratch{
		apples:  NewBG(w, h),
		occ:     NewBG(w, h),
		otherOc: NewBG(w, h),
	}
}

func isGroundedRef(c Point, grounded []bool, birds []refBird, apples *BitGrid) bool {
	below := Point{c.X, c.Y + 1}
	if grid.WBelow(c) {
		return true
	}
	if apples.Has(below) {
		return true
	}
	for i, ok := range grounded {
		if ok && birds[i].alive && birds[i].body.contains(below) {
			return true
		}
	}
	return false
}

func simulateOneTurn(sc *refScratch, mine []botEntry, enemies []enemyInfo, ourDirs, enemyDirs []Direction, sources []Point) oneTurnOutcome {
	nMine := len(mine)
	nEnemy := len(enemies)
	nBirds := nMine + nEnemy

	// --- init birds ---
	for i, bot := range mine {
		b := &sc.birds[i]
		b.owner = 0
		b.body.set(bot.body)
		b.facing = b.body.facing()
		b.alive = true
	}
	for i, enemy := range enemies {
		b := &sc.birds[nMine+i]
		b.owner = 1
		b.body.set(enemy.body)
		b.facing = enemy.facing
		b.alive = true
	}

	// --- apples ---
	fillBG(&sc.apples, sources)

	// --- move ---
	for i := 0; i < nBirds; i++ {
		b := &sc.birds[i]
		if !b.alive || b.body.len == 0 {
			continue
		}
		var dir Direction
		if i < nMine {
			dir = ourDirs[i]
		} else {
			dir = enemyDirs[i-nMine]
		}
		if dir == DirNone {
			dir = b.facing
		}
		if dir == DirNone {
			continue
		}

		head := b.body.parts[0]
		newHead := Add(head, DirDelta[dir])
		willEat := sc.apples.Has(newHead)

		n := 0
		sc.nextBuf[n] = newHead
		n++
		if willEat {
			copy(sc.nextBuf[n:], b.body.parts[:b.body.len])
			n += b.body.len
		} else {
			copy(sc.nextBuf[n:], b.body.parts[:b.body.len-1])
			n += b.body.len - 1
		}
		b.body.len = n
		copy(b.body.parts[:n], sc.nextBuf[:n])
	}

	// --- eat apples ---
	nEaten := 0
	for i := 0; i < nBirds; i++ {
		b := &sc.birds[i]
		if b.alive && b.body.len > 0 && sc.apples.Has(b.body.parts[0]) {
			sc.eaten[nEaten] = b.body.parts[0]
			nEaten++
		}
	}
	for k := 0; k < nEaten; k++ {
		sc.apples.Clear(sc.eaten[k])
	}

	// --- beheadings ---
	var outcome oneTurnOutcome
	for i := 0; i < nBirds; i++ {
		sc.toBehead[i] = false
	}
	for i := 0; i < nBirds; i++ {
		bi := &sc.birds[i]
		if !bi.alive || bi.body.len == 0 {
			continue
		}
		head := bi.body.parts[0]
		if grid.IsActualWall(head) {
			sc.toBehead[i] = true
			continue
		}
		for j := 0; j < nBirds; j++ {
			bj := &sc.birds[j]
			if !bj.alive || bj.body.len == 0 {
				continue
			}
			if !bj.body.contains(head) {
				continue
			}
			if i != j {
				sc.toBehead[i] = true
				break
			}
			for k := 1; k < bj.body.len; k++ {
				if bj.body.parts[k] == head {
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
		if b.body.len <= 3 {
			outcome.losses[b.owner] += b.body.len
			outcome.deaths[b.owner]++
			b.alive = false
			continue
		}
		outcome.losses[b.owner]++
		copy(b.body.parts[:b.body.len-1], b.body.parts[1:b.body.len])
		b.body.len--
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
				bi := &sc.birds[i]
				isGrnd := false
				for k := 0; k < bi.body.len; k++ {
					if isGroundedRef(bi.body.parts[k], sc.grounded[:nBirds], sc.birds[:nBirds], &sc.apples) {
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
			for j := 0; j < bi.body.len; j++ {
				bi.body.parts[j].Y++
			}
			allOut := true
			for j := 0; j < bi.body.len; j++ {
				if bi.body.parts[j].Y < H+1 {
					allOut = false
					break
				}
			}
			if allOut {
				outcome.deaths[bi.owner]++
				bi.alive = false
				sc.airborne[i] = false
			}
		}
	}

	// --- update facing ---
	for i := 0; i < nBirds; i++ {
		if sc.birds[i].alive {
			sc.birds[i].facing = sc.birds[i].body.facing()
		}
	}

	// --- trapped check (our bots only) ---
	sc.occ.Reset()
	for i := 0; i < nBirds; i++ {
		if !sc.birds[i].alive {
			continue
		}
		bi := &sc.birds[i]
		for k := 0; k < bi.body.len; k++ {
			sc.occ.Set(bi.body.parts[k])
		}
	}
	for i := 0; i < nMine; i++ {
		bi := &sc.birds[i]
		if !bi.alive || bi.body.len == 0 {
			continue
		}
		copy(sc.otherOc.Bits, sc.occ.Bits)
		for k := 0; k < bi.body.len; k++ {
			sc.otherOc.Clear(bi.body.parts[k])
		}
		hasEscape := false
		body := bi.body.slice()
		for _, dir := range state.VMoves(body[0], bi.facing) {
			_, _, alive, _, _ := simMove(body, bi.facing, dir, &sc.apples, &sc.otherOc)
			if alive {
				hasEscape = true
				break
			}
		}
		if !hasEscape {
			outcome.trapped[0]++
		}
	}

	return outcome
}

func outcomeRisk(outcome oneTurnOutcome) int {
	return outcome.deaths[0]*100000 + outcome.trapped[0]*5000 + outcome.losses[0]*100 - outcome.deaths[1]*20 - outcome.losses[1]
}

func worstCasePlanRisk(sc *refScratch, mine []botEntry, enemies []enemyInfo, sources []Point, ourDirs []Direction) int {
	if len(enemies) == 0 {
		return outcomeRisk(simulateOneTurn(sc, mine, nil, ourDirs, nil, sources))
	}

	worst := -1
	var walk func(idx int)
	walk = func(idx int) {
		if idx == len(enemies) {
			risk := outcomeRisk(simulateOneTurn(sc, mine, enemies, ourDirs, sc.enemyDirs[:len(enemies)], sources))
			if risk > worst {
				worst = risk
			}
			return
		}
		dirs, nd := validDirs(enemies[idx].facing)
		for di := 0; di < nd; di++ {
			sc.enemyDirs[idx] = dirs[di]
			walk(idx + 1)
		}
	}
	walk(0)
	return worst
}

func refinePlansWithOneTurnSafety(sc *refScratch, mine []botEntry, enemies []enemyInfo, sources []Point, plans []botPlan, deadline time.Time) {
	if len(mine) == 0 || len(enemies) == 0 || time.Until(deadline) < 8*time.Millisecond {
		return
	}

	combos := 1
	for _, enemy := range enemies {
		_, nd := validDirs(enemy.facing)
		combos *= nd
		if combos > 128 {
			return
		}
	}

	nPlans := len(plans)
	for i := 0; i < nPlans; i++ {
		sc.ourDirs[i] = plans[i].dir
	}

	bestRisk := worstCasePlanRisk(sc, mine, enemies, sources, sc.ourDirs[:nPlans])
	for i := 0; i < nPlans; i++ {
		if time.Until(deadline) < 4*time.Millisecond {
			break
		}
		currentDir := sc.ourDirs[i]
		dirs, nd := validDirs(plans[i].facing)
		for _, dir := range dirs[:nd] {
			if dir == currentDir {
				continue
			}
			sc.candidate = sc.ourDirs
			sc.candidate[i] = dir
			risk := worstCasePlanRisk(sc, mine, enemies, sources, sc.candidate[:nPlans])
			if risk < bestRisk {
				bestRisk = risk
				sc.ourDirs = sc.candidate
				plans[i].dir = dir
				plans[i].target = Add(plans[i].body[0], DirDelta[dir])
				plans[i].reason = "safety"
				plans[i].ok = true
			}
		}
	}
}

// isHeadLockedWorstCase checks whether, after our bot moves in dir,
// there exists ANY combination of enemy moves that leaves our bot
// with zero valid follow-up moves (head locked).
func isHeadLockedWorstCase(body []Point, facing, dir Direction, enemies []enemyInfo, otherOcc, srcBG *BitGrid) bool {
	head := body[0]
	newHead := Add(head, DirDelta[dir])
	nearbyEnemies := make([]enemyInfo, 0, len(enemies))
	for _, e := range enemies {
		if MDist(newHead, e.head) <= 3 {
			nearbyEnemies = append(nearbyEnemies, e)
		}
	}
	if len(nearbyEnemies) == 0 {
		return false
	}

	nb, nf, alive, _, _ := simMove(body, facing, dir, srcBG, otherOcc)
	if !alive {
		return true
	}

	baseOcc := NewBG(W, H)
	copy(baseOcc.Bits, otherOcc.Bits)
	for _, p := range nb[1:] {
		baseOcc.Set(p)
	}

	combos := 1
	for _, e := range nearbyEnemies {
		_, nd := validDirs(e.facing)
		combos *= nd
		if combos > 27 {
			return false
		}
	}

	finalHead := nb[0]
	eDirs := make([]Direction, len(nearbyEnemies))
	testOcc := NewBG(W, H)
	locked := false

	var walk func(idx int)
	walk = func(idx int) {
		if locked {
			return
		}
		if idx == len(nearbyEnemies) {
			copy(testOcc.Bits, baseOcc.Bits)
			for ei, e := range nearbyEnemies {
				nh := Add(e.head, DirDelta[eDirs[ei]])
				if !grid.IsActualWall(nh) {
					testOcc.Set(nh)
				}
			}
			for _, d := range state.VMoves(finalHead, nf) {
				nh := Add(finalHead, DirDelta[d])
				if !grid.IsActualWall(nh) && !testOcc.Has(nh) {
					return
				}
			}
			locked = true
			return
		}
		dirs, nd := validDirs(nearbyEnemies[idx].facing)
		for di := 0; di < nd; di++ {
			eDirs[idx] = dirs[di]
			walk(idx + 1)
		}
	}
	walk(0)
	return locked
}
