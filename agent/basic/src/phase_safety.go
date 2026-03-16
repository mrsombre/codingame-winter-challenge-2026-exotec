package main

// --- Phase 5: Safety check ---

const (
	safetyRolloutHorizon = 5
	safetyRolloutMyEat   = 800
	safetyRolloutOpEat   = 600
	safetyRolloutMyDeath = 12000
	safetyRolloutOpDeath = 6000
	safetyRolloutMyBody  = 40
	safetyRolloutOpBody  = 25
	safetyRolloutTrap    = 4000
	safetyRolloutReach   = 5
)

func (d *Decision) phaseSafety() {
	g := d.G
	n := g.W * g.H
	numMy := len(d.MySnakes)

	// Build body-cell bitmap (all alive snake bodies).
	bodyCell := make([]bool, n)
	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive {
			continue
		}
		for _, c := range sn.Body {
			if c >= 0 && c < n {
				bodyCell[c] = true
			}
		}
	}

	// --- Phase A: per-snake safety scoring ---

	// safeScore[si][dir] = -1 (lethal) or reachable cell count.
	safeScore := make([][4]int, numMy)

	for si := 0; si < numMy; si++ {
		snIdx := d.MySnakes[si]
		sn := &g.Sn[snIdx]
		head := sn.Body[0]
		neck := neckOf(sn.Body)
		ownTail := sn.Body[len(sn.Body)-1]

		// OOB head: emergency — prefer directions back inside the map.
		if head >= n {
			if head < g.NCells {
				bestDir := -1
				bestPriority := -1
				for dir := 0; dir < 4; dir++ {
					nc := g.Nb[head][dir]
					if nc == -1 || nc == neck {
						continue
					}
					if nc >= 0 && nc < n && bodyCell[nc] && nc != ownTail {
						continue
					}
					priority := 0
					if nc >= 0 && nc < n {
						priority = 2 // back inside map
					} else {
						priority = 1 // still OOB but alive
					}
					if priority > bestPriority {
						bestPriority = priority
						bestDir = dir
					}
				}
				if bestDir >= 0 {
					d.AssignedDir[si] = bestDir
				}
			}
			for dir := 0; dir < 4; dir++ {
				safeScore[si][dir] = -1
			}
			continue
		}

		if head < 0 {
			for dir := 0; dir < 4; dir++ {
				safeScore[si][dir] = -1
			}
			continue
		}

		bfs := d.BFS[si]

		// Count reachable cells per first-direction (flood fill proxy).
		var reach [4]int
		if bfs != nil {
			for c := 0; c < n; c++ {
				r := bfs[c]
				if r.Dist > 0 && r.FirstDir >= 0 && r.FirstDir < 4 {
					reach[r.FirstDir]++
				}
			}
		}

		for dir := 0; dir < 4; dir++ {
			safeScore[si][dir] = -1
			nc := g.Nb[head][dir]
			if nc == -1 || nc == neck {
				continue
			}
			// Body collision (own tail retracts → safe to enter).
			if nc >= 0 && nc < n && bodyCell[nc] && nc != ownTail {
				continue
			}
			safeScore[si][dir] = reach[dir]
		}

		// Override assigned direction if lethal or trap.
		bestDir := -1
		bestScore := -1
		for dir := 0; dir < 4; dir++ {
			if safeScore[si][dir] > bestScore {
				bestScore = safeScore[si][dir]
				bestDir = dir
			}
		}

		if bestDir == -1 {
			continue
		}

		assigned := d.AssignedDir[si]
		bodyLen := len(sn.Body)

		switch {
		case safeScore[si][assigned] < 0:
			d.AssignedDir[si] = bestDir
		case safeScore[si][assigned] < bodyLen && bestScore >= bodyLen:
			d.AssignedDir[si] = bestDir
		}
	}

	// --- Phase B: deconflict — no two of my snakes target the same cell ---

	for iter := 0; iter < numMy; iter++ {
		// Build target map: next-cell → first snake claiming it.
		targets := make(map[int]int) // nc → si
		conflictSI := -1

		for si := 0; si < numMy; si++ {
			snIdx := d.MySnakes[si]
			sn := &g.Sn[snIdx]
			head := sn.Body[0]
			if head < 0 || head >= n {
				continue
			}
			nc := g.Nb[head][d.AssignedDir[si]]
			if nc < 0 {
				continue
			}

			if _, ok := targets[nc]; ok {
				conflictSI = si // later snake loses the conflict
				break
			}
			targets[nc] = si
		}

		if conflictSI < 0 {
			break // no conflicts
		}

		// Redirect conflictSI to its best non-conflicting safe direction.
		si := conflictSI
		snIdx := d.MySnakes[si]
		head := g.Sn[snIdx].Body[0]
		bestAlt := -1
		bestAltScore := -1
		for dir := 0; dir < 4; dir++ {
			if dir == d.AssignedDir[si] {
				continue
			}
			if safeScore[si][dir] < 0 {
				continue
			}
			nc := g.Nb[head][dir]
			if nc < 0 {
				continue
			}
			if _, ok := targets[nc]; ok {
				continue // another snake already targets this cell
			}
			if safeScore[si][dir] > bestAltScore {
				bestAltScore = safeScore[si][dir]
				bestAlt = dir
			}
		}
		if bestAlt >= 0 {
			d.AssignedDir[si] = bestAlt
		} else {
			break // can't resolve without cascading — stop
		}
	}

	d.refineSafetyWithRollout(safeScore)
}

func copySafetySnakes(src []Snake) []Snake {
	dst := make([]Snake, len(src))
	for i := range src {
		dst[i] = src[i]
		if src[i].Body != nil {
			dst[i].Body = append([]int(nil), src[i].Body...)
		}
	}
	return dst
}

func copyAppleCells(src []int) []int {
	return append([]int(nil), src...)
}

func removeEatenApples(apples []int, eaten []int) []int {
	if len(eaten) == 0 || len(apples) == 0 {
		return apples
	}
	rm := make(map[int]bool, len(eaten))
	for _, cell := range eaten {
		rm[cell] = true
	}
	dst := apples[:0]
	for _, cell := range apples {
		if !rm[cell] {
			dst = append(dst, cell)
		}
	}
	return dst
}

func countOwnerAlive(snakes []Snake, owner int) int {
	total := 0
	for i := range snakes {
		if snakes[i].Alive && snakes[i].Owner == owner && len(snakes[i].Body) > 0 {
			total++
		}
	}
	return total
}

func totalOwnerBody(snakes []Snake, owner int) int {
	total := 0
	for i := range snakes {
		if snakes[i].Alive && snakes[i].Owner == owner {
			total += len(snakes[i].Body)
		}
	}
	return total
}

func candidateDirs(assigned int, scores [4]int) []int {
	var out []int
	seen := [4]bool{}
	add := func(dir int) {
		if dir < 0 || dir >= 4 || seen[dir] {
			return
		}
		seen[dir] = true
		out = append(out, dir)
	}

	add(assigned)
	for len(out) < 3 {
		bestDir := -1
		bestScore := -1 << 30
		for dir := 0; dir < 4; dir++ {
			if seen[dir] {
				continue
			}
			score := scores[dir]
			if score > bestScore {
				bestScore = score
				bestDir = dir
			}
		}
		if bestDir < 0 {
			break
		}
		add(bestDir)
	}
	if len(out) == 0 {
		add(DU)
	}
	return out
}

func applePresent(apples []int, cell int) bool {
	for _, ap := range apples {
		if ap == cell {
			return true
		}
	}
	return false
}

// rolloutChooseDirCheap picks a direction toward target using Manhattan distance.
// No BFS — uses only neighbor lookups. Falls back to nearest apple if target is eaten.
func (d *Decision) rolloutChooseDirCheap(sn Snake, apples []int, target int) int {
	g := d.G
	if !sn.Alive || len(sn.Body) == 0 {
		return DU
	}
	head := sn.Body[0]
	if head < 0 || head >= g.NCells {
		return DU
	}
	neck := neckOf(sn.Body)
	n := g.W * g.H

	// OOB head: prefer directions back inside the map.
	if head >= n {
		bestDir := -1
		bestPriority := -1
		for dir := 0; dir < 4; dir++ {
			nc := g.Nb[head][dir]
			if nc == -1 || nc == neck {
				continue
			}
			priority := 0
			if nc >= 0 && nc < n {
				priority = 2
			} else {
				priority = 1
			}
			if priority > bestPriority {
				bestPriority = priority
				bestDir = dir
			}
		}
		if bestDir >= 0 {
			return bestDir
		}
		return DU
	}

	// Resolve target: use assigned if still available, else nearest apple.
	if target < 0 || !applePresent(apples, target) {
		target = -1
		bestDist := MaxCells
		for _, ap := range apples {
			if md := g.Manhattan(head, ap); md < bestDist {
				bestDist = md
				target = ap
			}
		}
	}

	bestDir := -1
	bestScore := -1 << 30
	for dir := 0; dir < 4; dir++ {
		nc := g.Nb[head][dir]
		if nc == -1 || nc == neck {
			continue
		}
		if nc >= 0 && nc < n && !g.Cell[nc] {
			continue // wall
		}

		score := 0
		if nc >= 0 && nc < n {
			if target >= 0 {
				score = -g.Manhattan(nc, target) * 100
			}
			score += 50 // prefer in-map over OOB
			_, ny := g.XY(nc)
			score += g.H - 1 - ny // height bonus
		}

		if score > bestScore {
			bestScore = score
			bestDir = dir
		}
	}
	if bestDir >= 0 {
		return bestDir
	}
	return DU
}

// quickReach does a simple 4-neighbor flood fill from head, counting
// reachable free cells up to limit. No gravity — just wall avoidance.
// Cheap proxy for trap detection in rollout scoring.
func quickReach(g *Game, head int, limit int) int {
	n := g.W * g.H
	if head < 0 || head >= n {
		return 0
	}
	if limit > n {
		limit = n
	}
	visited := make([]bool, n)
	visited[head] = true
	queue := make([]int, 1, limit)
	queue[0] = head
	count := 0
	for qi := 0; qi < len(queue) && count < limit; qi++ {
		c := queue[qi]
		count++
		for dir := 0; dir < 4; dir++ {
			nc := g.Nb[c][dir]
			if nc < 0 || nc >= n || visited[nc] || !g.Cell[nc] {
				continue
			}
			visited[nc] = true
			queue = append(queue, nc)
		}
	}
	return count
}

func (d *Decision) scoreRollout(firstDirs []int) int {
	g := d.G
	savedApples := copyAppleCells(g.Ap[:g.ANum])
	savedANum := g.ANum
	defer func() {
		g.Ap = savedApples
		g.ANum = savedANum
		d.P.RebuildAppleMap()
	}()

	// Pre-compute targets: my snakes from assignment, enemy from pre-computed BFS.
	targets := make([]int, g.SNum)
	for i := range targets {
		targets[i] = -1
	}
	for si, snIdx := range d.MySnakes {
		targets[snIdx] = d.Assigned[si]
	}
	for oi, snIdx := range d.OpSnakes {
		bfs := d.OpBFS[oi]
		if bfs == nil {
			continue
		}
		bestDist := MaxCells
		for _, ap := range savedApples {
			if bfs[ap].Dist >= 0 && bfs[ap].Dist < bestDist {
				bestDist = bfs[ap].Dist
				targets[snIdx] = ap
			}
		}
	}

	mySlot := make([]int, g.SNum)
	for i := range mySlot {
		mySlot[i] = -1
	}
	for si, snIdx := range d.MySnakes {
		mySlot[snIdx] = si
	}

	snakes := copySafetySnakes(g.Sn[:g.SNum])
	apples := copyAppleCells(savedApples)
	score := 0

	for turn := 0; turn < safetyRolloutHorizon; turn++ {
		if countOwnerAlive(snakes, 0) == 0 || len(apples) == 0 {
			break
		}

		startMyAlive := countOwnerAlive(snakes, 0)
		startOpAlive := countOwnerAlive(snakes, 1)

		g.Ap = apples
		g.ANum = len(apples)
		d.P.RebuildAppleMap()

		dirs := make([]int, g.SNum)
		for i := 0; i < g.SNum; i++ {
			if !snakes[i].Alive || len(snakes[i].Body) == 0 {
				continue
			}
			if snakes[i].Owner == 0 && turn == 0 && mySlot[i] >= 0 {
				dirs[i] = firstDirs[mySlot[i]]
			} else {
				dirs[i] = d.rolloutChooseDirCheap(snakes[i], apples, targets[i])
			}
		}

		myEats := 0
		opEats := 0
		for i := 0; i < g.SNum; i++ {
			if !snakes[i].Alive || len(snakes[i].Body) == 0 {
				continue
			}
			newBody, alive := d.P.simulateMove(snakes[i].Body, dirs[i])
			if !alive {
				snakes[i].Alive = false
				snakes[i].Body = nil
				snakes[i].Len = 0
				continue
			}
			snakes[i].Body = append([]int(nil), newBody...)
			snakes[i].Len = len(snakes[i].Body)
			head := snakes[i].Body[0]
			if head >= 0 && head < g.OobBase && d.P.isApple(head) {
				if snakes[i].Owner == 0 {
					myEats++
				} else {
					opEats++
				}
			}
		}

		eaten := d.P.resolveMove(snakes)
		apples = removeEatenApples(apples, eaten)

		endMyAlive := countOwnerAlive(snakes, 0)
		endOpAlive := countOwnerAlive(snakes, 1)

		score += myEats * safetyRolloutMyEat
		score -= opEats * safetyRolloutOpEat
		score -= (startMyAlive - endMyAlive) * safetyRolloutMyDeath
		score += (startOpAlive - endOpAlive) * safetyRolloutOpDeath
		score += totalOwnerBody(snakes, 0) * safetyRolloutMyBody
		score -= totalOwnerBody(snakes, 1) * safetyRolloutOpBody
	}

	for i := 0; i < g.SNum; i++ {
		if !snakes[i].Alive || snakes[i].Owner != 0 || len(snakes[i].Body) == 0 {
			continue
		}
		bodyLen := len(snakes[i].Body)
		head := snakes[i].Body[0]
		reach := quickReach(g, head, bodyLen*3)
		if reach < bodyLen {
			score -= safetyRolloutTrap
		} else {
			score += reach * safetyRolloutReach
		}
	}

	return score
}

func (d *Decision) refineSafetyWithRollout(safeScore [][4]int) {
	numMy := len(d.MySnakes)
	if numMy == 0 {
		return
	}

	candidates := make([][]int, numMy)
	for si := 0; si < numMy; si++ {
		candidates[si] = candidateDirs(d.AssignedDir[si], safeScore[si])
	}

	bestDirs := append([]int(nil), d.AssignedDir...)
	bestScore := d.scoreRollout(bestDirs)

	cur := make([]int, numMy)
	var dfs func(int)
	dfs = func(si int) {
		if si == numMy {
			score := d.scoreRollout(cur)
			if score > bestScore {
				bestScore = score
				copy(bestDirs, cur)
			}
			return
		}
		for _, dir := range candidates[si] {
			cur[si] = dir
			dfs(si + 1)
		}
	}
	dfs(0)

	copy(d.AssignedDir, bestDirs)
}
