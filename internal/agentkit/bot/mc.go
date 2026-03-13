package bot

import (
	"time"

	"codingame/internal/agentkit/game"
)

// ---------------------------------------------------------------------------
// Fixed-size bitgrid for zero-alloc rollout simulation
// ---------------------------------------------------------------------------

const RollMaxCells = 44 * 24

type RollBG [RollMaxCells/64 + 1]uint64

func rbHas(g *RollBG, p game.Point, w, h int) bool {
	if p.X < 0 || p.X >= w || p.Y < 0 || p.Y >= h {
		return false
	}
	i := p.Y*w + p.X
	return g[i/64]&(1<<uint(i%64)) != 0
}

func rbSet(g *RollBG, p game.Point, w, h int) {
	if p.X < 0 || p.X >= w || p.Y < 0 || p.Y >= h {
		return
	}
	i := p.Y*w + p.X
	g[i/64] |= 1 << uint(i%64)
}

func rbClear(g *RollBG, p game.Point, w, h int) {
	if p.X < 0 || p.X >= w || p.Y < 0 || p.Y >= h {
		return
	}
	i := p.Y*w + p.X
	g[i/64] &^= 1 << uint(i%64)
}

func rbFill(g *RollBG, pts []game.Point, w, h int) {
	*g = RollBG{}
	for _, p := range pts {
		rbSet(g, p, w, h)
	}
}

// ---------------------------------------------------------------------------
// Rollout apple distance cache
// ---------------------------------------------------------------------------

var rollAppleDist [24][44]int

func PrecomputeRollAppleDists(g *game.AGrid, sources []game.Point) {
	w, h := g.Width, g.Height
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			rollAppleDist[y][x] = 9999
		}
	}
	type qi struct{ x, y, d int }
	var queue [1100]qi
	qLen := 0
	for _, s := range sources {
		if s.X >= 0 && s.X < w && s.Y >= 0 && s.Y < h {
			rollAppleDist[s.Y][s.X] = 0
			queue[qLen] = qi{s.X, s.Y, 0}
			qLen++
		}
	}
	for i := 0; i < qLen; i++ {
		q := queue[i]
		for d := game.DirUp; d <= game.DirLeft; d++ {
			nx := q.x + game.DirDelta[d].X
			ny := q.y + game.DirDelta[d].Y
			if nx < 0 || nx >= w || ny < 0 || ny >= h || g.IsWall(game.Point{X: nx, Y: ny}) {
				continue
			}
			nd := q.d + 1
			if nd < rollAppleDist[ny][nx] {
				rollAppleDist[ny][nx] = nd
				if qLen < len(queue) {
					queue[qLen] = qi{nx, ny, nd}
					qLen++
				}
			}
		}
	}
}

// RollFloodCount does a bounded BFS flood from head through non-wall, non-occupied cells.
func RollFloodCount(g *game.AGrid, head game.Point, occ *RollBG, maxCount int) int {
	w, h := g.Width, g.Height
	if g.IsWall(head) || rbHas(occ, head, w, h) {
		return 0
	}
	var visited RollBG
	rbSet(&visited, head, w, h)
	var queue [64]game.Point
	queue[0] = head
	qLen := 1
	count := 0
	for i := 0; i < qLen && count < maxCount; i++ {
		p := queue[i]
		count++
		for d := game.DirUp; d <= game.DirLeft; d++ {
			np := game.Add(p, game.DirDelta[d])
			if g.IsWall(np) || rbHas(&visited, np, w, h) || rbHas(occ, np, w, h) {
				continue
			}
			rbSet(&visited, np, w, h)
			if qLen < len(queue) {
				queue[qLen] = np
				qLen++
			}
		}
	}
	return count
}

// ---------------------------------------------------------------------------
// Rollout simulation state
// ---------------------------------------------------------------------------

const MaxRollBots = 8

type RollBot struct {
	ID    int
	Owner int
	Alive bool
	Body  game.Body
}

type RollState struct {
	Grid     *game.AGrid
	Bots     [MaxRollBots]RollBot
	BotCount int
	Apples   RollBG
	Occ      RollBG
	Scores   [2]int
	Losses   [2]int
}

func (s *RollState) CopyFrom(src *RollState) {
	s.Grid = src.Grid
	s.BotCount = src.BotCount
	for i := 0; i < src.BotCount; i++ {
		d := &s.Bots[i]
		sr := &src.Bots[i]
		d.ID = sr.ID
		d.Owner = sr.Owner
		d.Alive = sr.Alive
		d.Body.Len = sr.Body.Len
		copy(d.Body.Parts[:sr.Body.Len], sr.Body.Parts[:sr.Body.Len])
	}
	s.Apples = src.Apples
	s.Occ = src.Occ
	s.Scores = src.Scores
	s.Losses = src.Losses
}

func (s *RollState) RebuildOcc() {
	s.Occ = RollBG{}
	w, h := s.Grid.Width, s.Grid.Height
	for i := 0; i < s.BotCount; i++ {
		b := &s.Bots[i]
		if !b.Alive {
			continue
		}
		for k := 0; k < b.Body.Len; k++ {
			rbSet(&s.Occ, b.Body.Parts[k], w, h)
		}
	}
}

func (s *RollState) SimTurn(moves *[MaxRollBots]game.Direction) {
	s.doMoves(moves)
	s.doEats()
	s.doBeheadings()
	s.doFalls()
}

func (s *RollState) doMoves(moves *[MaxRollBots]game.Direction) {
	w, h := s.Grid.Width, s.Grid.Height
	for i := 0; i < s.BotCount; i++ {
		b := &s.Bots[i]
		if !b.Alive {
			continue
		}
		dir := moves[i]
		if dir == game.DirNone {
			dir = b.Body.Facing()
		}
		if dir == game.DirNone {
			continue
		}
		newHead := game.Add(b.Body.Parts[0], game.DirDelta[dir])
		willEat := rbHas(&s.Apples, newHead, w, h)
		if !willEat && b.Body.Len > 0 {
			rbClear(&s.Occ, b.Body.Parts[b.Body.Len-1], w, h)
			b.Body.Len--
		}
		copy(b.Body.Parts[1:b.Body.Len+1], b.Body.Parts[:b.Body.Len])
		b.Body.Parts[0] = newHead
		b.Body.Len++
		rbSet(&s.Occ, newHead, w, h)
	}
}

func (s *RollState) doEats() {
	w, h := s.Grid.Width, s.Grid.Height
	for i := 0; i < s.BotCount; i++ {
		b := &s.Bots[i]
		if !b.Alive || b.Body.Len == 0 {
			continue
		}
		head := b.Body.Parts[0]
		if rbHas(&s.Apples, head, w, h) {
			rbClear(&s.Apples, head, w, h)
			s.Scores[b.Owner]++
		}
	}
}

func (s *RollState) doBeheadings() {
	g := s.Grid
	var toBehead [MaxRollBots]bool
	for i := 0; i < s.BotCount; i++ {
		b := &s.Bots[i]
		if !b.Alive || b.Body.Len == 0 {
			continue
		}
		head := b.Body.Parts[0]
		if g.IsWall(head) {
			toBehead[i] = true
			continue
		}
		for j := 0; j < s.BotCount && !toBehead[i]; j++ {
			if j == i || !s.Bots[j].Alive {
				continue
			}
			o := &s.Bots[j]
			for k := 0; k < o.Body.Len; k++ {
				if o.Body.Parts[k] == head {
					toBehead[i] = true
					break
				}
			}
		}
		if toBehead[i] {
			continue
		}
		for k := 1; k < b.Body.Len; k++ {
			if b.Body.Parts[k] == head {
				toBehead[i] = true
				break
			}
		}
	}

	w, h := g.Width, g.Height
	for i := 0; i < s.BotCount; i++ {
		if !toBehead[i] {
			continue
		}
		b := &s.Bots[i]
		if b.Body.Len <= 3 {
			for _, p := range b.Body.Slice() {
				rbClear(&s.Occ, p, w, h)
			}
			b.Alive = false
			s.Losses[b.Owner] += b.Body.Len
			game.BodyReset(&b.Body)
			continue
		}
		rbClear(&s.Occ, b.Body.Parts[0], w, h)
		copy(b.Body.Parts[:b.Body.Len-1], b.Body.Parts[1:b.Body.Len])
		b.Body.Len--
		s.Losses[b.Owner]++
	}
}

func (s *RollState) solidUnder(p game.Point, ignore *RollBG) bool {
	w, h := s.Grid.Width, s.Grid.Height
	below := game.Point{p.X, p.Y + 1}
	if ignore != nil && rbHas(ignore, below, w, h) {
		return false
	}
	return s.Grid.IsWall(below) || rbHas(&s.Occ, below, w, h) || rbHas(&s.Apples, below, w, h)
}

func (s *RollState) doFalls() {
	g := s.Grid
	w, h := g.Width, g.Height

	allGrounded := true
	for i := 0; i < s.BotCount; i++ {
		b := &s.Bots[i]
		if !b.Alive {
			continue
		}
		grounded := false
		for _, p := range b.Body.Slice() {
			if g.WBelow(p) {
				grounded = true
				break
			}
		}
		if !grounded {
			allGrounded = false
			break
		}
	}
	if allGrounded {
		return
	}

	var bodySet RollBG
	somethingFell := true
	for somethingFell {
		for somethingFell {
			somethingFell = false
			for i := 0; i < s.BotCount; i++ {
				b := &s.Bots[i]
				if !b.Alive {
					continue
				}
				bodySet = RollBG{}
				for k := 0; k < b.Body.Len; k++ {
					rbSet(&bodySet, b.Body.Parts[k], w, h)
				}
				canFall := true
				for _, p := range b.Body.Slice() {
					if s.solidUnder(p, &bodySet) {
						canFall = false
						break
					}
				}
				if !canFall {
					continue
				}
				somethingFell = true
				for j, p := range b.Body.Slice() {
					rbClear(&s.Occ, p, w, h)
					b.Body.Parts[j].Y++
					rbSet(&s.Occ, b.Body.Parts[j], w, h)
				}
				allOut := true
				for _, p := range b.Body.Slice() {
					if p.Y < h+1 {
						allOut = false
						break
					}
				}
				if allOut {
					for _, p := range b.Body.Slice() {
						rbClear(&s.Occ, p, w, h)
					}
					b.Alive = false
					game.BodyReset(&b.Body)
				}
			}
		}
		somethingFell = s.doIntercoiledFalls()
	}
}

func (s *RollState) doIntercoiledFalls() bool {
	w, h := s.Grid.Width, s.Grid.Height
	fell := false
	changed := true
	var metaSet RollBG
	for changed {
		changed = false
		groups := s.getIntercoiledGroups()
		for _, group := range groups {
			metaSet = RollBG{}
			for _, idx := range group {
				for _, p := range s.Bots[idx].Body.Slice() {
					rbSet(&metaSet, p, w, h)
				}
			}
			canFall := true
			for _, idx := range group {
				b := &s.Bots[idx]
				for _, p := range b.Body.Slice() {
					if s.solidUnder(p, &metaSet) {
						canFall = false
						break
					}
				}
				if !canFall {
					break
				}
			}
			if !canFall {
				continue
			}
			changed = true
			fell = true
			for _, idx := range group {
				b := &s.Bots[idx]
				for j, p := range b.Body.Slice() {
					rbClear(&s.Occ, p, w, h)
					b.Body.Parts[j].Y++
					rbSet(&s.Occ, b.Body.Parts[j], w, h)
				}
				if b.Body.Len > 0 && b.Body.Parts[0].Y >= h {
					for _, p := range b.Body.Slice() {
						rbClear(&s.Occ, p, w, h)
					}
					b.Alive = false
					game.BodyReset(&b.Body)
				}
			}
		}
	}
	return fell
}

func (s *RollState) getIntercoiledGroups() [][]int {
	var groups [][]int
	var visited [MaxRollBots]bool
	for i := 0; i < s.BotCount; i++ {
		if visited[i] || !s.Bots[i].Alive {
			continue
		}
		var group []int
		var queue [MaxRollBots]int
		queue[0] = i
		qLen := 1
		visited[i] = true
		for qi := 0; qi < qLen; qi++ {
			cur := queue[qi]
			group = append(group, cur)
			ba := &s.Bots[cur]
			for j := 0; j < s.BotCount; j++ {
				if visited[j] || !s.Bots[j].Alive {
					continue
				}
				bb := &s.Bots[j]
				touching := false
				for _, p := range ba.Body.Slice() {
					for dir := game.DirUp; dir <= game.DirLeft; dir++ {
						np := game.Add(p, game.DirDelta[dir])
						for _, op := range bb.Body.Slice() {
							if op == np {
								touching = true
								break
							}
						}
						if touching {
							break
						}
					}
					if touching {
						break
					}
				}
				if touching {
					visited[j] = true
					queue[qLen] = j
					qLen++
				}
			}
		}
		if len(group) > 1 {
			groups = append(groups, group)
		}
	}
	return groups
}

// ---------------------------------------------------------------------------
// Rollout policy & evaluation
// ---------------------------------------------------------------------------

// SanitizeRollDir ensures dir is legal; falls back to first legal move.
func SanitizeRollDir(s *game.State, body game.Body, dir game.Direction) game.Direction {
	head, ok := game.BodyHead(&body)
	if !ok {
		return game.DirNone
	}
	legal := s.VMoves(head, body.Facing())
	if len(legal) == 0 {
		return dir
	}
	for _, cand := range legal {
		if cand == dir {
			return dir
		}
	}
	return legal[0]
}

// RollPolicyDir picks a greedy move for botIdx in the rollout sim.
func RollPolicyDir(s *game.State, sim *RollState, botIdx int, variant int) game.Direction {
	b := &sim.Bots[botIdx]
	if !b.Alive || b.Body.Len == 0 {
		return game.DirNone
	}
	head := b.Body.Parts[0]
	facing := b.Body.Facing()
	legal := s.VMoves(head, facing)
	if len(legal) == 0 {
		return facing
	}

	w, h := s.Grid.Width, s.Grid.Height
	bestDir := legal[0]
	bestScore := -1_000_000
	for _, dir := range legal {
		np := game.Add(head, game.DirDelta[dir])
		score := 0
		if s.Grid.IsWall(np) {
			score = -50000
		} else {
			if rbHas(&sim.Apples, np, w, h) {
				score += 10000
			}
			if rbHas(&sim.Occ, np, w, h) {
				isTail := b.Body.Len > 0 && b.Body.Parts[b.Body.Len-1] == np && !rbHas(&sim.Apples, head, w, h)
				if !isTail {
					score -= 5000
				}
			}
			if np.X >= 0 && np.X < w && np.Y >= 0 && np.Y < h {
				score -= rollAppleDist[np.Y][np.X] * 10
			}
			flood := RollFloodCount(s.Grid, np, &sim.Occ, 32)
			if flood <= 2 {
				score -= 3000
			} else if flood < b.Body.Len {
				score -= 500
			}
			score += minInt(flood, b.Body.Len*3) * 15
			if s.Grid.WBelow(np) {
				score += 30
			}
			for i := 0; i < sim.BotCount; i++ {
				if i == botIdx || !sim.Bots[i].Alive {
					continue
				}
				other := &sim.Bots[i]
				dist := game.MDist(np, other.Body.Parts[0])
				if other.Owner == b.Owner {
					if dist == 0 {
						score -= 900
					}
				} else if dist == 1 && b.Body.Len <= other.Body.Len {
					score -= 200
				}
			}
			score += ((int(dir) + variant*3) % 5) * 2
		}
		if score > bestScore {
			bestScore = score
			bestDir = dir
		}
	}
	return bestDir
}

// EvalRollState scores a rollout state from myPlayer's perspective.
func EvalRollState(sim *RollState, myPlayer int, targets []game.Point, hasTarget []bool) int {
	w, h := sim.Grid.Width, sim.Grid.Height
	score := (sim.Scores[myPlayer] - sim.Scores[1-myPlayer]) * 1500
	score -= (sim.Losses[myPlayer] - sim.Losses[1-myPlayer]) * 800

	for i := 0; i < sim.BotCount; i++ {
		b := &sim.Bots[i]
		sign := 1
		if b.Owner != myPlayer {
			sign = -1
		}
		if !b.Alive || b.Body.Len == 0 {
			score -= sign * 10000
			continue
		}
		head := b.Body.Parts[0]
		score += sign * b.Body.Len * 300
		if head.X >= 0 && head.X < w && head.Y >= 0 && head.Y < h {
			ad := rollAppleDist[head.Y][head.X]
			if ad > 100 {
				ad = 100
			}
			score -= sign * ad * 20
		}
		if hasTarget[i] {
			score -= sign * game.MDist(head, targets[i]) * 15
		}
		flood := RollFloodCount(sim.Grid, head, &sim.Occ, 32)
		if flood <= 1 {
			score -= sign * 4000
		} else if flood < b.Body.Len {
			score -= sign * 800
		}
		score += sign * minInt(flood, b.Body.Len*3) * 12
	}
	return score
}

// NearestTarget returns the closest source by SrcScore.
func NearestTarget(g *game.AGrid, head game.Point, sources []game.Point) (game.Point, bool) {
	if len(sources) == 0 {
		return game.Point{}, false
	}
	best := sources[0]
	bestDist := game.SrcScore(g, head, best)
	for _, s := range sources[1:] {
		if d := game.SrcScore(g, head, s); d < bestDist {
			bestDist = d
			best = s
		}
	}
	return best, true
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ---------------------------------------------------------------------------
// Monte Carlo rollout refinement
// ---------------------------------------------------------------------------

const MCRolloutDepth = 5

// MyBotInfo holds per-bot data for MC refinement.
type MyBotInfo struct {
	ID   int
	Body []game.Point
}

// MCRefine evaluates candidate first-move combinations via averaged
// greedy rollouts and updates plans with the best combination found.
func MCRefine(s *game.State, mine []MyBotInfo, enemies []EnemyInfo, sources []game.Point,
	plans []SearchResult, allOcc *game.BitGrid, deadline time.Time) {
	cutoff := deadline.Add(-2 * time.Millisecond)
	if len(mine) == 0 || time.Now().After(cutoff) {
		return
	}

	g := s.Grid
	PrecomputeRollAppleDists(g, sources)

	// Build base rollout state
	var base RollState
	base.Grid = g
	nMy := len(mine)
	nOpp := len(enemies)
	base.BotCount = nMy + nOpp
	myIdx := make([]int, nMy)
	oppIdx := make([]int, nOpp)
	targets := make([]game.Point, base.BotCount)
	hasTarget := make([]bool, base.BotCount)

	for i, mb := range mine {
		base.Bots[i] = RollBot{
			ID: mb.ID, Owner: 0, Alive: true,
			Body: game.NewBody(mb.Body),
		}
		myIdx[i] = i
		if plans[i].Ok {
			targets[i] = plans[i].Target
			hasTarget[i] = true
		} else if t, ok := NearestTarget(g, mb.Body[0], sources); ok {
			targets[i] = t
			hasTarget[i] = true
		}
	}
	for i, enemy := range enemies {
		idx := nMy + i
		base.Bots[idx] = RollBot{
			ID: -1 - i, Owner: 1, Alive: true,
			Body: game.NewBody(enemy.Body),
		}
		oppIdx[i] = idx
		if t, ok := NearestTarget(g, enemy.Head, sources); ok {
			targets[idx] = t
			hasTarget[idx] = true
		}
	}
	rbFill(&base.Apples, sources, g.Width, g.Height)
	base.RebuildOcc()

	// Precompute enemy threat zones: cells any enemy head could reach next turn.
	type eThreat struct {
		cell    game.Point
		bodyLen int
	}
	var enemyThreats []eThreat
	for _, enemy := range enemies {
		facing := enemy.Facing
		if facing == game.DirNone {
			facing = game.DirUp
		}
		for _, d := range s.VMoves(enemy.Head, facing) {
			np := game.Add(enemy.Head, game.DirDelta[d])
			if !g.IsWall(np) {
				enemyThreats = append(enemyThreats, eThreat{cell: np, bodyLen: enemy.BodyLen})
			}
		}
	}

	// Candidate first moves per bot: plan direction + up to 2 alternatives
	const maxCands = 3
	type candSet [maxCands]game.Direction
	perBot := make([]candSet, nMy)
	perBotN := make([]int, nMy)
	for i, mb := range mine {
		facing := game.DirUp
		if len(mb.Body) >= 2 {
			facing = game.FacingPts(mb.Body[0], mb.Body[1])
		}
		legal := s.VMoves(mb.Body[0], facing)
		perBot[i][0] = plans[i].Dir
		k := 1
		for _, d := range legal {
			if d != plans[i].Dir && k < maxCands {
				perBot[i][k] = d
				k++
			}
		}
		perBotN[i] = k
	}

	// Adaptive variant count based on remaining time
	numVariants := 3
	remaining := time.Until(cutoff)
	if remaining > 25*time.Millisecond {
		numVariants = 5
	} else if remaining < 8*time.Millisecond {
		numVariants = 2
	}

	// Evaluate a candidate first-move set by averaging rollout variants
	const maxTeam = 4
	eval := func(firstMoves [maxTeam]game.Direction) int {
		// Contested-cell penalty
		contestPen := 0
		for k := 0; k < nMy; k++ {
			mb := &base.Bots[myIdx[k]]
			if !mb.Alive {
				continue
			}
			dir := SanitizeRollDir(s, mb.Body, firstMoves[k])
			dst := game.Add(mb.Body.Parts[0], game.DirDelta[dir])
			for _, th := range enemyThreats {
				if dst == th.cell {
					if mb.Body.Len <= 3 {
						contestPen += 5000
					} else if th.bodyLen > 3 {
						contestPen += 1000
					}
				}
			}
		}

		total := -contestPen * numVariants
		for v := 0; v < numVariants; v++ {
			var sim RollState
			sim.CopyFrom(&base)

			var moves [MaxRollBots]game.Direction
			for k := 0; k < sim.BotCount; k++ {
				moves[k] = game.DirNone
			}
			for k := 0; k < nMy; k++ {
				moves[myIdx[k]] = SanitizeRollDir(s, sim.Bots[myIdx[k]].Body, firstMoves[k])
			}
			for k := 0; k < nOpp; k++ {
				moves[oppIdx[k]] = RollPolicyDir(s, &sim, oppIdx[k], v)
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
		return total
	}

	// Evaluate baseline (current greedy plans)
	var baseMoves [maxTeam]game.Direction
	for i := range mine {
		baseMoves[i] = plans[i].Dir
	}
	bestScore := eval(baseMoves)
	bestMoves := baseMoves

	// Enumerate all candidate combinations recursively
	var tryMoves [maxTeam]game.Direction
	timedOut := false
	var enumerate func(bot int)
	enumerate = func(bot int) {
		if timedOut {
			return
		}
		if bot == nMy {
			same := true
			for k := 0; k < nMy; k++ {
				if tryMoves[k] != baseMoves[k] {
					same = false
					break
				}
			}
			if same {
				return
			}
			if time.Now().After(cutoff) {
				timedOut = true
				return
			}
			score := eval(tryMoves)
			if score > bestScore {
				bestScore = score
				bestMoves = tryMoves
			}
			return
		}
		for c := 0; c < perBotN[bot]; c++ {
			tryMoves[bot] = perBot[bot][c]
			enumerate(bot + 1)
			if timedOut {
				return
			}
		}
	}
	enumerate(0)

	// Apply best moves
	for i := range mine {
		plans[i].Dir = bestMoves[i]
	}
}
