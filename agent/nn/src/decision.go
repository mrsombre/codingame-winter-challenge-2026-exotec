package main

import (
	"fmt"
	"os"
	"strings"
)

type Candidate struct {
	Legal       bool
	Dir         int
	Score       float32
	Head        int
	Body        []int
	Eating      bool
	Supported   bool
	FallDistance int
	Flood       int
	SafeMoves   int
	WallAdj     int
	BlockedAdj  int
	HeadOnRisk  bool
	HeadOnWin   bool
	RaceDelta   int
}

type Decision struct {
	G           *Game
	M           *Model
	MySnakes    []int
	AssignedDir []int
	Candidates  [MaxPSn][4]Candidate
	appleSet    []bool
	occupiedBy  []int
}

func NewDecision(g *Game) *Decision {
	return &Decision{
		G:           g,
		M:           LoadModel(),
		MySnakes:    make([]int, 0, MaxPSn),
		AssignedDir: make([]int, 0, MaxPSn),
		appleSet:    make([]bool, g.W*g.H),
		occupiedBy:  make([]int, g.W*g.H),
	}
}

func (d *Decision) Decide() {
	d.rebuildTurnMaps()
	d.collectMySnakes()
	if len(d.MySnakes) == 0 {
		fmt.Println("WAIT")
		return
	}

	d.AssignedDir = d.AssignedDir[:len(d.MySnakes)]
	for i, snIdx := range d.MySnakes {
		bestDir := d.bestDir(snIdx, i)
		d.AssignedDir[i] = bestDir
	}
	d.resolveFriendlyConflicts()
	d.emitCommands()
}

func (d *Decision) rebuildTurnMaps() {
	for i := range d.appleSet {
		d.appleSet[i] = false
		d.occupiedBy[i] = -1
	}
	for _, ap := range d.G.Ap {
		if d.G.IsInGrid(ap) {
			d.appleSet[ap] = true
		}
	}
	for i := 0; i < d.G.SNum; i++ {
		sn := &d.G.Sn[i]
		if !sn.Alive {
			continue
		}
		for _, cell := range sn.Body {
			if d.G.IsInGrid(cell) {
				d.occupiedBy[cell] = i
			}
		}
	}
}

func (d *Decision) collectMySnakes() {
	d.MySnakes = d.MySnakes[:0]
	for i := 0; i < d.G.SNum; i++ {
		sn := &d.G.Sn[i]
		if sn.Alive && sn.Owner == 0 && sn.Len > 0 {
			d.MySnakes = append(d.MySnakes, i)
		}
	}
}

func (d *Decision) bestDir(snIdx int, slot int) int {
	sn := &d.G.Sn[snIdx]

	// 1. Evaluate all 4 candidate directions (legality + safety signals)
	for dir := 0; dir < 4; dir++ {
		cand := d.simulateCandidate(snIdx, dir)
		d.Candidates[slot][dir] = cand
	}

	// 2. BFS from head for apple scoring
	blocked := d.buildBlocked(snIdx, sn.Body[:sn.Len])
	floodCount, headDists := d.floodDist(sn.Body[0], blocked)

	// 3. Score all apples with NN, pick best target
	targetApple := -1
	bestAppleScore := float32(-1e9)
	if d.M.Trained {
		var feats [featureCount]float32
		for _, ap := range d.G.Ap {
			d.fillAppleFeatures(&feats, snIdx, ap, sn.Body[0], floodCount, headDists)
			score := d.M.Score(feats[:])
			if score > bestAppleScore {
				bestAppleScore = score
				targetApple = ap
			}
		}
	}

	// Fallback: nearest reachable apple
	if targetApple < 0 {
		bestDist := 1 << 30
		for _, ap := range d.G.Ap {
			if headDists[ap] >= 0 && headDists[ap] < bestDist {
				bestDist = headDists[ap]
				targetApple = ap
			}
		}
	}

	// 4. BFS from target apple for direction finding
	var appleDists []int
	if targetApple >= 0 {
		_, appleDists = d.floodDist(targetApple, blocked)
	}

	// 5. Pick best direction: closest to target + safety
	bestDir := -1
	bestMetric := float32(-1e9)
	for dir := 0; dir < 4; dir++ {
		cand := &d.Candidates[slot][dir]
		if !cand.Legal {
			continue
		}

		metric := float32(0)

		// Distance to target apple
		if targetApple >= 0 && appleDists != nil {
			if appleDists[cand.Head] >= 0 {
				metric -= float32(appleDists[cand.Head]) / 75
			} else {
				ax, ay := d.G.XY(targetApple)
				cx, cy := d.G.XY(cand.Head)
				metric -= float32(abs(ax-cx)+abs(ay-cy)) / 75
			}
		}

		// Safety signals
		metric += float32(cand.Flood) / float32(MaxCells)
		metric += float32(cand.SafeMoves) / 3
		metric += boolf(cand.Eating) * 3
		metric += boolf(cand.Supported) * 0.1
		metric -= float32(cand.FallDistance) / float32(MaxH)
		metric -= boolf(cand.HeadOnRisk) * 2
		metric += boolf(cand.HeadOnWin)

		cand.Score = metric
		if metric > bestMetric {
			bestMetric = metric
			bestDir = dir
		}
	}

	if bestDir >= 0 {
		return bestDir
	}
	for dir := 0; dir < 4; dir++ {
		if sn.Len > 1 && dir == Do[sn.Dir] {
			continue
		}
		return dir
	}
	return sn.Dir
}

// fillAppleFeatures computes 16 features for a (snake, apple) pair.
// Must match apple_features_for_snake in common.py exactly.
func (d *Decision) fillAppleFeatures(f *[featureCount]float32, snIdx int, apple int, head int, floodCount int, dists []int) {
	g := d.G
	hx, hy := g.XY(head)
	ax, ay := g.XY(apple)

	bfsDist := dists[apple]
	if bfsDist < 0 {
		bfsDist = abs(ax-hx) + abs(ay-hy) + g.W + g.H
	}
	enemyDist := d.enemyAppleDist(snIdx, apple)
	race := bfsDist - enemyDist

	myTotal, opTotal := 0, 0
	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive {
			continue
		}
		if sn.Owner == 0 {
			myTotal += sn.Len
		} else {
			opTotal += sn.Len
		}
	}
	snakeLen := g.Sn[snIdx].Len
	friendlyDist := d.nearestFriendlyDist(snIdx, apple)

	f[0] = clampf(float32(bfsDist)/75, 0, 1)
	f[1] = clampf(float32(enemyDist)/75, 0, 1)
	f[2] = clampf(float32(race)/75, -1, 1)
	f[3] = float32(ax-hx) / float32(MaxW)
	f[4] = float32(ay-hy) / float32(MaxH)
	f[5] = float32(snakeLen) / float32(MaxSeg)
	f[6] = float32(g.TurnNum) / float32(MaxTurns)
	f[7] = float32(g.ANum) / float32(MaxAp)
	f[8] = float32(myTotal) / 128
	f[9] = float32(opTotal) / 128
	f[10] = float32(g.W) / float32(MaxW)
	f[11] = float32(g.H) / float32(MaxH)
	if dists[apple] >= 0 {
		f[12] = 1
	} else {
		f[12] = 0
	}
	f[13] = float32(floodCount) / float32(MaxCells)
	f[14] = float32(ay) / float32(MaxH)
	f[15] = clampf(float32(friendlyDist)/75, 0, 1)
}

func (d *Decision) nearestFriendlyDist(snIdx int, apple int) int {
	ax, ay := d.G.XY(apple)
	best := 75
	for i := 0; i < d.G.SNum; i++ {
		sn := &d.G.Sn[i]
		if !sn.Alive || i == snIdx || sn.Owner != 0 || sn.Len == 0 || !d.G.IsInGrid(sn.Body[0]) {
			continue
		}
		fx, fy := d.G.XY(sn.Body[0])
		dist := abs(ax-fx) + abs(ay-fy)
		if dist < best {
			best = dist
		}
	}
	return best
}

func (d *Decision) resolveFriendlyConflicts() {
	for i := 0; i < len(d.MySnakes); i++ {
		for j := i + 1; j < len(d.MySnakes); j++ {
			ci := d.Candidates[i][d.AssignedDir[i]]
			cj := d.Candidates[j][d.AssignedDir[j]]
			if !ci.Legal || !cj.Legal || ci.Head != cj.Head {
				continue
			}
			loser := j
			if cj.Score > ci.Score {
				loser = i
			}
			d.AssignedDir[loser] = d.findAlternative(loser, ci.Head)
		}
	}
}

func (d *Decision) findAlternative(slot int, blockedHead int) int {
	bestDir := d.AssignedDir[slot]
	bestScore := float32(-1e9)
	for dir := 0; dir < 4; dir++ {
		cand := d.Candidates[slot][dir]
		if !cand.Legal || cand.Head == blockedHead {
			continue
		}
		if cand.Score > bestScore {
			bestScore = cand.Score
			bestDir = dir
		}
	}
	return bestDir
}

func (d *Decision) emitCommands() {
	parts := make([]string, 0, len(d.MySnakes))
	for i, snIdx := range d.MySnakes {
		parts = append(parts, fmt.Sprintf("%d %s", d.G.Sn[snIdx].ID, Dn[d.AssignedDir[i]]))
	}
	fmt.Fprintln(os.Stdout, strings.Join(parts, ";"))
}

func (d *Decision) simulateCandidate(snIdx int, dir int) Candidate {
	g := d.G
	sn := &g.Sn[snIdx]
	cand := Candidate{Dir: dir}
	if !sn.Alive || sn.Len == 0 {
		return cand
	}
	if sn.Len > 1 && dir == Do[sn.Dir] {
		return cand
	}

	hx, hy := g.XY(sn.Body[0])
	nx := hx + Dl[dir][0]
	ny := hy + Dl[dir][1]
	if !g.InBounds(nx, ny) {
		return cand
	}
	next := g.Idx(nx, ny)
	if !g.Cell[next] {
		return cand
	}
	if d.occupiedBy[next] >= 0 && d.occupiedBy[next] != snIdx {
		return cand
	}

	eating := d.appleSet[next]
	bodyLen := sn.Len
	if eating && bodyLen < MaxSeg {
		bodyLen++
	}
	body := make([]int, bodyLen)
	body[0] = next
	if eating {
		copy(body[1:], sn.Body)
	} else if sn.Len > 1 {
		copy(body[1:], sn.Body[:sn.Len-1])
	}

	for i := 1; i < len(body); i++ {
		if body[i] == body[0] {
			return cand
		}
	}

	supported := d.isSupported(body, snIdx, next)
	fallDistance := 0
	for !supported {
		for i, cell := range body {
			x, y := g.XY(cell)
			y++
			if !g.InBounds(x, y) {
				return cand
			}
			nb := g.Idx(x, y)
			if !g.Cell[nb] {
				return cand
			}
			if d.occupiedBy[nb] >= 0 && d.occupiedBy[nb] != snIdx {
				return cand
			}
			body[i] = nb
		}
		fallDistance++
		supported = d.isSupported(body, snIdx, next)
	}

	cand.Legal = true
	cand.Head = body[0]
	cand.Body = body
	cand.Eating = eating
	cand.Supported = supported
	cand.FallDistance = fallDistance
	d.populateCandidateSignals(snIdx, &cand)
	return cand
}

func (d *Decision) isSupported(body []int, snIdx int, eatenApple int) bool {
	for _, cell := range body {
		x, y := d.G.XY(cell)
		by := y + 1
		if !d.G.InBounds(x, by) {
			continue
		}
		below := d.G.Idx(x, by)
		if !d.G.Cell[below] {
			return true
		}
		if d.appleSet[below] && below != eatenApple {
			return true
		}
		if d.occupiedBy[below] >= 0 && d.occupiedBy[below] != snIdx {
			return true
		}
	}
	return false
}

func (d *Decision) populateCandidateSignals(snIdx int, cand *Candidate) {
	if cand.Flood != 0 || cand.SafeMoves != 0 || cand.WallAdj != 0 || cand.BlockedAdj != 0 || cand.HeadOnRisk || cand.HeadOnWin || cand.RaceDelta != 0 {
		return
	}
	blocked := d.buildBlocked(snIdx, cand.Body)
	var dists []int
	cand.Flood, dists = d.floodDist(cand.Head, blocked)
	cand.SafeMoves = d.countFutureSafeMoves(snIdx, cand)
	cand.WallAdj, cand.BlockedAdj = d.adjacentCounts(snIdx, cand)
	cand.HeadOnRisk, cand.HeadOnWin = d.headOnSignals(snIdx, cand)
	cand.RaceDelta = d.bestRaceDelta(snIdx, cand, dists)
}

func (d *Decision) buildBlocked(snIdx int, body []int) []bool {
	blocked := make([]bool, d.G.W*d.G.H)
	for cell := 0; cell < len(blocked); cell++ {
		if !d.G.Cell[cell] {
			blocked[cell] = true
		}
	}
	for cell, owner := range d.occupiedBy {
		if owner >= 0 && owner != snIdx {
			blocked[cell] = true
		}
	}
	for i := 1; i < len(body); i++ {
		cell := body[i]
		if d.G.IsInGrid(cell) {
			blocked[cell] = true
		}
	}
	return blocked
}

func (d *Decision) floodDist(start int, blocked []bool) (int, []int) {
	dists := make([]int, len(blocked))
	for i := range dists {
		dists[i] = -1
	}
	if !d.G.IsInGrid(start) || blocked[start] {
		return 0, dists
	}
	queue := make([]int, 1, len(blocked))
	queue[0] = start
	dists[start] = 0
	count := 0
	for qi := 0; qi < len(queue); qi++ {
		cell := queue[qi]
		count++
		x, y := d.G.XY(cell)
		for dir := 0; dir < 4; dir++ {
			nx := x + Dl[dir][0]
			ny := y + Dl[dir][1]
			if !d.G.InBounds(nx, ny) {
				continue
			}
			next := d.G.Idx(nx, ny)
			if blocked[next] || dists[next] >= 0 {
				continue
			}
			dists[next] = dists[cell] + 1
			queue = append(queue, next)
		}
	}
	return count, dists
}

func (d *Decision) countFutureSafeMoves(snIdx int, cand *Candidate) int {
	apples := make([]bool, len(d.appleSet))
	copy(apples, d.appleSet)
	if cand.Eating && d.G.IsInGrid(cand.Head) {
		apples[cand.Head] = false
	}
	safe := 0
	for dir := 0; dir < 4; dir++ {
		if len(cand.Body) > 1 && dir == Do[cand.Dir] {
			continue
		}
		if d.futureMoveLegal(snIdx, cand.Body, cand.Dir, dir, apples) {
			safe++
		}
	}
	return safe
}

func (d *Decision) futureMoveLegal(snIdx int, body []int, facing int, dir int, apples []bool) bool {
	if len(body) == 0 || !d.G.IsInGrid(body[0]) {
		return false
	}
	if len(body) > 1 && dir == Do[facing] {
		return false
	}
	hx, hy := d.G.XY(body[0])
	nx := hx + Dl[dir][0]
	ny := hy + Dl[dir][1]
	if !d.G.InBounds(nx, ny) {
		return false
	}
	next := d.G.Idx(nx, ny)
	if !d.G.Cell[next] {
		return false
	}
	if d.occupiedBy[next] >= 0 && d.occupiedBy[next] != snIdx {
		return false
	}
	eating := apples[next]
	bodyLen := len(body)
	if eating && bodyLen < MaxSeg {
		bodyLen++
	}
	moved := make([]int, bodyLen)
	moved[0] = next
	if eating {
		copy(moved[1:], body)
	} else if len(body) > 1 {
		copy(moved[1:], body[:len(body)-1])
	}
	seen := make(map[int]bool, len(moved))
	for _, cell := range moved {
		if seen[cell] {
			return false
		}
		seen[cell] = true
	}
	eatenApple := -1
	if eating {
		eatenApple = next
	}
	supported := d.isSupported(moved, snIdx, eatenApple)
	for !supported {
		for i, cell := range moved {
			x, y := d.G.XY(cell)
			y++
			if !d.G.InBounds(x, y) {
				return false
			}
			nb := d.G.Idx(x, y)
			if !d.G.Cell[nb] {
				return false
			}
			if d.occupiedBy[nb] >= 0 && d.occupiedBy[nb] != snIdx {
				return false
			}
			moved[i] = nb
		}
		seen = make(map[int]bool, len(moved))
		for _, cell := range moved {
			if seen[cell] {
				return false
			}
			seen[cell] = true
		}
		supported = d.isSupported(moved, snIdx, eatenApple)
	}
	return true
}

func (d *Decision) adjacentCounts(snIdx int, cand *Candidate) (int, int) {
	local := make(map[int]bool, len(cand.Body))
	for i := 1; i < len(cand.Body); i++ {
		local[cand.Body[i]] = true
	}
	walls := 0
	blocked := 0
	hx, hy := d.G.XY(cand.Head)
	for dir := 0; dir < 4; dir++ {
		nx := hx + Dl[dir][0]
		ny := hy + Dl[dir][1]
		if !d.G.InBounds(nx, ny) {
			walls++
			blocked++
			continue
		}
		cell := d.G.Idx(nx, ny)
		if !d.G.Cell[cell] {
			walls++
			blocked++
			continue
		}
		if local[cell] || (d.occupiedBy[cell] >= 0 && d.occupiedBy[cell] != snIdx) {
			blocked++
		}
	}
	return walls, blocked
}

func (d *Decision) headOnSignals(snIdx int, cand *Candidate) (bool, bool) {
	risk := false
	win := false
	myLen := len(cand.Body)
	for i := 0; i < d.G.SNum; i++ {
		sn := &d.G.Sn[i]
		if !sn.Alive || i == snIdx || sn.Owner == 0 || sn.Len == 0 {
			continue
		}
		hx, hy := d.G.XY(sn.Body[0])
		for dir := 0; dir < 4; dir++ {
			if sn.Len > 1 && dir == Do[sn.Dir] {
				continue
			}
			nx := hx + Dl[dir][0]
			ny := hy + Dl[dir][1]
			if !d.G.InBounds(nx, ny) {
				continue
			}
			cell := d.G.Idx(nx, ny)
			if !d.G.Cell[cell] || cell != cand.Head {
				continue
			}
			risk = true
			if myLen > 3 && sn.Len <= 3 {
				win = true
			}
			break
		}
	}
	return risk, win
}

func (d *Decision) bestRaceDelta(snIdx int, cand *Candidate, dists []int) int {
	if len(d.G.Ap) == 0 {
		return 0
	}
	hx, hy := d.G.XY(cand.Head)
	bestOwn := 1 << 30
	bestRace := 1 << 30
	for _, ap := range d.G.Ap {
		if cand.Eating && ap == cand.Head {
			continue
		}
		ownDist := dists[ap]
		if ownDist < 0 {
			ax, ay := d.G.XY(ap)
			ownDist = abs(ax-hx) + abs(ay-hy) + d.G.W + d.G.H
		}
		race := ownDist - d.enemyAppleDist(snIdx, ap)
		if ownDist < bestOwn || (ownDist == bestOwn && race < bestRace) {
			bestOwn = ownDist
			bestRace = race
		}
	}
	if bestRace == 1<<30 {
		return 0
	}
	return bestRace
}

func (d *Decision) enemyAppleDist(snIdx int, apple int) int {
	ax, ay := d.G.XY(apple)
	best := 1 << 30
	for i := 0; i < d.G.SNum; i++ {
		sn := &d.G.Sn[i]
		if !sn.Alive || i == snIdx || sn.Owner == 0 || sn.Len == 0 || !d.G.IsInGrid(sn.Body[0]) {
			continue
		}
		ex, ey := d.G.XY(sn.Body[0])
		dist := abs(ax-ex) + abs(ay-ey)
		if dist < best {
			best = dist
		}
	}
	if best == 1<<30 {
		return d.G.W + d.G.H
	}
	return best
}

func boolf(v bool) float32 {
	if v {
		return 1
	}
	return 0
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func clampf(v, lo, hi float32) float32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
