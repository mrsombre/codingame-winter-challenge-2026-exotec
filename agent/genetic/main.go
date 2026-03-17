package main

import (
	"bufio"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"
)

const debug = false

var scanner *bufio.Scanner

func readline() string {
	if !scanner.Scan() {
		os.Exit(0)
	}
	line := scanner.Text()
	if debug {
		fmt.Fprintln(os.Stderr, line)
	}
	return line
}

// ---------------------------------------------------------------------------
// Core types
// ---------------------------------------------------------------------------

type Pt struct{ x, y int }

func ptAdd(a, b Pt) Pt { return Pt{a.x + b.x, a.y + b.y} }

const (
	DirUp    = 0
	DirRight = 1
	DirDown  = 2
	DirLeft  = 3
	DirNone  = 4
)

var dirDelta = [5]Pt{{0, -1}, {1, 0}, {0, 1}, {-1, 0}, {0, 0}}
var dirNames = [4]string{"UP", "RIGHT", "DOWN", "LEFT"}

func opposite(d int) int { return d ^ 2 }

func facingFromPts(a, b Pt) int {
	dx := a.x - b.x
	dy := a.y - b.y
	switch {
	case dx == 0 && dy == -1:
		return DirUp
	case dx == 1 && dy == 0:
		return DirRight
	case dx == 0 && dy == 1:
		return DirDown
	case dx == -1 && dy == 0:
		return DirLeft
	}
	return DirNone
}

// ---------------------------------------------------------------------------
// Static grid
// ---------------------------------------------------------------------------

var (
	gridW, gridH int
	wallGrid     [][]bool
	// wallBelow[y][x] = true if (x, y+1) is wall or y==gridH-1
	wallBelow [24][44]bool
	// validMovesCache[y][x][facing] = slice of valid dirs
	validMovesCache [24][44][5][]int
)

func isWall(p Pt) bool {
	if p.x < 0 || p.x >= gridW || p.y < 0 || p.y >= gridH {
		return true
	}
	return wallGrid[p.y][p.x]
}

func precompute() {
	for y := 0; y < gridH; y++ {
		for x := 0; x < gridW; x++ {
			wallBelow[y][x] = y == gridH-1 || wallGrid[y+1][x]
			if wallGrid[y][x] {
				continue
			}
			for f := 0; f < 5; f++ {
				var dirs []int
				back := -1
				if f < 4 {
					back = opposite(f)
				}
				for d := 0; d < 4; d++ {
					if d == back {
						continue
					}
					np := ptAdd(Pt{x, y}, dirDelta[d])
					if !isWall(np) {
						dirs = append(dirs, d)
					}
				}
				validMovesCache[y][x][f] = dirs
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Bitset for occupancy / apples — O(1) lookups
// ---------------------------------------------------------------------------

const MaxCells = 44 * 24

type BitGrid [MaxCells/64 + 1]uint64

func inBounds(p Pt) bool { return p.x >= 0 && p.x < gridW && p.y >= 0 && p.y < gridH }
func cellIdx(p Pt) int   { return p.y*gridW + p.x }
func (g *BitGrid) set(p Pt) {
	if inBounds(p) {
		i := cellIdx(p)
		g[i/64] |= 1 << uint(i%64)
	}
}
func (g *BitGrid) clear(p Pt) {
	if inBounds(p) {
		i := cellIdx(p)
		g[i/64] &^= 1 << uint(i%64)
	}
}
func (g *BitGrid) has(p Pt) bool {
	if !inBounds(p) {
		return false
	}
	i := cellIdx(p)
	return g[i/64]&(1<<uint(i%64)) != 0
}

// ---------------------------------------------------------------------------
// Simulation state
// ---------------------------------------------------------------------------

const MaxBots = 8
const MaxBody = 80

type SimBot struct {
	body    [MaxBody]Pt
	bodyLen int
	alive   bool
	owner   int
	id      int
}

func (b *SimBot) head() Pt { return b.body[0] }

func (b *SimBot) facing() int {
	if b.bodyLen < 2 {
		return DirNone
	}
	return facingFromPts(b.body[0], b.body[1])
}

type SimState struct {
	bots     [MaxBots]SimBot
	botCount int
	apples   BitGrid
	occ      BitGrid // all live bot bodies
	scores   [2]int
	losses   [2]int
}

func (s *SimState) rebuildOcc() {
	s.occ = BitGrid{}
	for i := 0; i < s.botCount; i++ {
		b := &s.bots[i]
		if !b.alive {
			continue
		}
		for k := 0; k < b.bodyLen; k++ {
			s.occ.set(b.body[k])
		}
	}
}

func (s *SimState) copyFrom(src *SimState) {
	s.botCount = src.botCount
	for i := 0; i < src.botCount; i++ {
		d := &s.bots[i]
		sr := &src.bots[i]
		d.bodyLen = sr.bodyLen
		copy(d.body[:sr.bodyLen], sr.body[:sr.bodyLen])
		d.alive = sr.alive
		d.owner = sr.owner
		d.id = sr.id
	}
	s.apples = src.apples
	s.occ = src.occ
	s.scores = src.scores
	s.losses = src.losses
}

// ---------------------------------------------------------------------------
// Turn simulation (engine parity)
// ---------------------------------------------------------------------------

func (s *SimState) simTurn(moves *[MaxBots]int) {
	s.doMoves(moves)
	s.doEats()
	s.doBeheadings()
	s.doFalls()
}

func (s *SimState) doMoves(moves *[MaxBots]int) {
	for i := 0; i < s.botCount; i++ {
		b := &s.bots[i]
		if !b.alive {
			continue
		}
		dir := moves[i]
		if dir == DirNone {
			dir = b.facing()
		}
		if dir == DirNone {
			continue
		}
		newHead := ptAdd(b.head(), dirDelta[dir])
		willEat := s.apples.has(newHead)
		if !willEat && b.bodyLen > 0 {
			s.occ.clear(b.body[b.bodyLen-1])
			b.bodyLen--
		}
		copy(b.body[1:b.bodyLen+1], b.body[:b.bodyLen])
		b.body[0] = newHead
		b.bodyLen++
		s.occ.set(newHead)
	}
}

func (s *SimState) doEats() {
	for i := 0; i < s.botCount; i++ {
		b := &s.bots[i]
		if b.alive && s.apples.has(b.head()) {
			s.apples.clear(b.head())
			s.scores[b.owner]++
		}
	}
}

func (s *SimState) doBeheadings() {
	var toBehead [MaxBots]bool
	for i := 0; i < s.botCount; i++ {
		b := &s.bots[i]
		if !b.alive {
			continue
		}
		head := b.head()
		if isWall(head) {
			toBehead[i] = true
			continue
		}
		for j := 0; j < s.botCount; j++ {
			if j == i || !s.bots[j].alive {
				continue
			}
			o := &s.bots[j]
			for k := 0; k < o.bodyLen; k++ {
				if o.body[k] == head {
					toBehead[i] = true
					break
				}
			}
			if toBehead[i] {
				break
			}
		}
		if !toBehead[i] {
			for k := 1; k < b.bodyLen; k++ {
				if b.body[k] == head {
					toBehead[i] = true
					break
				}
			}
		}
	}
	for i := 0; i < s.botCount; i++ {
		if !toBehead[i] {
			continue
		}
		b := &s.bots[i]
		if b.bodyLen <= 3 {
			for k := 0; k < b.bodyLen; k++ {
				s.occ.clear(b.body[k])
			}
			b.alive = false
			s.losses[b.owner] += b.bodyLen
		} else {
			s.occ.clear(b.body[0])
			copy(b.body[:], b.body[1:b.bodyLen])
			b.bodyLen--
			s.losses[b.owner]++
		}
	}
}

// solidUnder: O(1) via bitsets. ignoreSet is the body to skip (own or meta).
func (s *SimState) solidUnder(c Pt, ignoreSet *BitGrid) bool {
	below := Pt{c.x, c.y + 1}
	if ignoreSet.has(below) {
		return false
	}
	if isWall(below) || s.occ.has(below) || s.apples.has(below) {
		return true
	}
	return false
}

func (s *SimState) doFalls() {
	// Quick check: if every alive bot has at least one segment with wall below, skip
	allGrounded := true
	for i := 0; i < s.botCount; i++ {
		b := &s.bots[i]
		if !b.alive {
			continue
		}
		grounded := false
		for k := 0; k < b.bodyLen; k++ {
			p := b.body[k]
			if p.x >= 0 && p.x < gridW && p.y >= 0 && p.y < gridH && wallBelow[p.y][p.x] {
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

	somethingFell := true
	var bodySet BitGrid
	for somethingFell {
		for somethingFell {
			somethingFell = false
			for i := 0; i < s.botCount; i++ {
				b := &s.bots[i]
				if !b.alive {
					continue
				}
				bodySet = BitGrid{}
				for k := 0; k < b.bodyLen; k++ {
					bodySet.set(b.body[k])
				}
				canFall := true
				for k := 0; k < b.bodyLen; k++ {
					if s.solidUnder(b.body[k], &bodySet) {
						canFall = false
						break
					}
				}
				if canFall {
					somethingFell = true
					for k := 0; k < b.bodyLen; k++ {
						s.occ.clear(b.body[k])
						b.body[k].y++
						s.occ.set(b.body[k])
					}
					allOut := true
					for k := 0; k < b.bodyLen; k++ {
						if b.body[k].y < gridH+1 {
							allOut = false
							break
						}
					}
					if allOut {
						for k := 0; k < b.bodyLen; k++ {
							s.occ.clear(b.body[k])
						}
						b.alive = false
					}
				}
			}
		}
		somethingFell = s.doIntercoiledFalls()
	}
}

func (s *SimState) doIntercoiledFalls() bool {
	fell := false
	changed := true
	var metaSet BitGrid
	for changed {
		changed = false
		groups := s.getIntercoiledGroups()
		for _, group := range groups {
			metaSet = BitGrid{}
			for _, idx := range group {
				b := &s.bots[idx]
				for k := 0; k < b.bodyLen; k++ {
					metaSet.set(b.body[k])
				}
			}
			canFall := true
			for _, idx := range group {
				if !canFall {
					break
				}
				b := &s.bots[idx]
				for k := 0; k < b.bodyLen; k++ {
					if s.solidUnder(b.body[k], &metaSet) {
						canFall = false
						break
					}
				}
			}
			if canFall {
				changed = true
				fell = true
				for _, idx := range group {
					b := &s.bots[idx]
					for k := 0; k < b.bodyLen; k++ {
						s.occ.clear(b.body[k])
						b.body[k].y++
						s.occ.set(b.body[k])
					}
					if b.head().y >= gridH {
						for k := 0; k < b.bodyLen; k++ {
							s.occ.clear(b.body[k])
						}
						b.alive = false
					}
				}
			}
		}
	}
	return fell
}

func (s *SimState) getIntercoiledGroups() [][]int {
	var groups [][]int
	var visited [MaxBots]bool
	for i := 0; i < s.botCount; i++ {
		if visited[i] || !s.bots[i].alive {
			continue
		}
		var group []int
		var queue [MaxBots]int
		queue[0] = i
		qLen := 1
		visited[i] = true
		for qi := 0; qi < qLen; qi++ {
			cur := queue[qi]
			group = append(group, cur)
			ba := &s.bots[cur]
			for j := 0; j < s.botCount; j++ {
				if visited[j] || !s.bots[j].alive {
					continue
				}
				// Fast adjacency: check if any cell of ba has a neighbor in bots[j]
				bb := &s.bots[j]
				touching := false
				for k := 0; k < ba.bodyLen && !touching; k++ {
					p := ba.body[k]
					for d := 0; d < 4; d++ {
						np := ptAdd(p, dirDelta[d])
						for m := 0; m < bb.bodyLen; m++ {
							if bb.body[m] == np {
								touching = true
								break
							}
						}
						if touching {
							break
						}
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
// Evaluation
// ---------------------------------------------------------------------------

var appleDistCache [24][44]int

func precomputeAppleDists(apples *BitGrid) {
	type qi struct{ x, y, d int }
	var queue [1100]qi
	qLen := 0
	for y := 0; y < gridH; y++ {
		for x := 0; x < gridW; x++ {
			appleDistCache[y][x] = 9999
			if apples.has(Pt{x, y}) {
				appleDistCache[y][x] = 0
				queue[qLen] = qi{x, y, 0}
				qLen++
			}
		}
	}
	for i := 0; i < qLen; i++ {
		q := queue[i]
		for d := 0; d < 4; d++ {
			nx := q.x + dirDelta[d].x
			ny := q.y + dirDelta[d].y
			if nx < 0 || nx >= gridW || ny < 0 || ny >= gridH || wallGrid[ny][nx] {
				continue
			}
			nd := q.d + 1
			if nd < appleDistCache[ny][nx] {
				appleDistCache[ny][nx] = nd
				if qLen < len(queue) {
					queue[qLen] = qi{nx, ny, nd}
					qLen++
				}
			}
		}
	}
}

// floodCount does a quick BFS flood from head, ignoring occ, returns reachable cells.
func floodCount(head Pt, occ *BitGrid, maxCount int) int {
	if head.x < 0 || head.x >= gridW || head.y < 0 || head.y >= gridH {
		return 0
	}
	var visited BitGrid
	visited.set(head)
	var queue [256]Pt
	queue[0] = head
	qLen := 1
	count := 0
	for i := 0; i < qLen && count < maxCount; i++ {
		p := queue[i]
		count++
		for d := 0; d < 4; d++ {
			np := ptAdd(p, dirDelta[d])
			if np.x < 0 || np.x >= gridW || np.y < 0 || np.y >= gridH {
				continue
			}
			if wallGrid[np.y][np.x] || visited.has(np) || occ.has(np) {
				continue
			}
			visited.set(np)
			if qLen < 256 {
				queue[qLen] = np
				qLen++
			}
		}
	}
	return count
}

func (s *SimState) evaluate(myPlayer int) int {
	score := 0

	// Count dead bots — catastrophic penalty per dead own bot
	myDead := 0
	oppDead := 0
	for i := 0; i < s.botCount; i++ {
		if !s.bots[i].alive {
			if s.bots[i].owner == myPlayer {
				myDead++
			} else {
				oppDead++
			}
		}
	}
	// Dead bot = lost all segments + can never eat again. Heavy but not paralyzing.
	score -= myDead * 10000
	score += oppDead * 10000

	// Apples eaten difference
	score += (s.scores[myPlayer] - s.scores[1-myPlayer]) * 1500

	// Loss segments (beheadings)
	score -= (s.losses[myPlayer] - s.losses[1-myPlayer]) * 800

	for i := 0; i < s.botCount; i++ {
		b := &s.bots[i]
		if !b.alive {
			continue
		}
		h := b.head()
		sign := 1
		if b.owner != myPlayer {
			sign = -1
		}

		// Body length
		score += sign * b.bodyLen * 300

		if h.x >= 0 && h.x < gridW && h.y >= 0 && h.y < gridH {
			// Apple distance — aggressive pursuit
			ad := appleDistCache[h.y][h.x]
			if ad > 100 {
				ad = 100
			}
			score -= sign * ad * 20
		}
	}
	return score
}

// ---------------------------------------------------------------------------
// Genetic algorithm
// ---------------------------------------------------------------------------

const (
	PopSize    = 20
	MaxDepth   = 6
	Elitism    = 2
	MutateRate = 25
)

type Gene struct {
	moves   [MaxDepth][4]int
	numBots int
	botIDs  [4]int
	fitness int
}

func (g *Gene) initBots(state *SimState, player int) {
	g.numBots = 0
	for i := 0; i < state.botCount; i++ {
		if state.bots[i].owner == player && state.bots[i].alive {
			g.botIDs[g.numBots] = i
			g.numBots++
		}
	}
}

func (g *Gene) randomize(rng *rand.Rand, state *SimState, player int) {
	g.initBots(state, player)
	for t := 0; t < MaxDepth; t++ {
		for b := 0; b < g.numBots; b++ {
			g.moves[t][b] = rng.Intn(4)
		}
	}
}

// greedySeed fills gene with "move toward nearest apple" heuristic per turn.
func (g *Gene) greedySeed(state *SimState, player int, variant int) {
	g.initBots(state, player)
	var sim SimState
	sim.copyFrom(state)

	for t := 0; t < MaxDepth; t++ {
		for b := 0; b < g.numBots; b++ {
			idx := g.botIDs[b]
			bot := &sim.bots[idx]
			if !bot.alive {
				g.moves[t][b] = DirUp
				continue
			}
			head := bot.head()
			if head.x < 0 || head.x >= gridW || head.y < 0 || head.y >= gridH {
				g.moves[t][b] = DirUp
				continue
			}
			f := bot.facing()
			valid := validMovesCache[head.y][head.x][f]
			if len(valid) == 0 {
				g.moves[t][b] = DirUp
				continue
			}

			bestDir := valid[0]
			bestScore := -999999

			for _, d := range valid {
				np := ptAdd(head, dirDelta[d])
				sc := 0
				// Apple proximity
				if np.x >= 0 && np.x < gridW && np.y >= 0 && np.y < gridH {
					if sim.apples.has(np) {
						sc += 10000 // instant eat
					} else {
						sc -= appleDistCache[np.y][np.x] * 10
					}
					// Prefer not colliding with other bodies
					if sim.occ.has(np) {
						sc -= 5000
					}
					// Variant: slight direction preference to diversify
					sc += (d ^ variant) % 3
				} else {
					sc -= 50000
				}
				if sc > bestScore {
					bestScore = sc
					bestDir = d
				}
			}
			g.moves[t][b] = bestDir
		}
		// Simulate forward to update state for next turn
		var moves [MaxBots]int
		for i := 0; i < sim.botCount; i++ {
			moves[i] = DirNone
		}
		for b := 0; b < g.numBots; b++ {
			idx := g.botIDs[b]
			if sim.bots[idx].alive {
				moves[idx] = sanitize(&sim.bots[idx], g.moves[t][b])
			}
		}
		// Other player uses facing
		sim.simTurn(&moves)
	}
}

func (g *Gene) mutate(rng *rand.Rand) {
	for t := 0; t < MaxDepth; t++ {
		for b := 0; b < g.numBots; b++ {
			if rng.Intn(100) < MutateRate {
				g.moves[t][b] = rng.Intn(4)
			}
		}
	}
}

func crossoverGenes(p1, p2, child *Gene, rng *rand.Rand) {
	child.numBots = p1.numBots
	child.botIDs = p1.botIDs
	cut := rng.Intn(MaxDepth)
	for t := 0; t < MaxDepth; t++ {
		if t < cut {
			child.moves[t] = p1.moves[t]
		} else {
			child.moves[t] = p2.moves[t]
		}
	}
}

func sanitize(bot *SimBot, dir int) int {
	if !bot.alive {
		return DirUp
	}
	head := bot.head()
	if head.x < 0 || head.x >= gridW || head.y < 0 || head.y >= gridH {
		return DirUp
	}
	f := bot.facing()
	valid := validMovesCache[head.y][head.x][f]
	if len(valid) == 0 {
		return dir
	}
	for _, v := range valid {
		if v == dir {
			return dir
		}
	}
	return valid[0]
}

func simulateGene(base *SimState, myGene, oppGene *Gene, myPlayer int) int {
	var state SimState
	state.copyFrom(base)

	var moves [MaxBots]int
	for t := 0; t < MaxDepth; t++ {
		anyMy, anyOpp := false, false
		for i := 0; i < state.botCount; i++ {
			if !state.bots[i].alive {
				continue
			}
			if state.bots[i].owner == myPlayer {
				anyMy = true
			} else {
				anyOpp = true
			}
		}
		if !anyMy || !anyOpp {
			break
		}
		for i := 0; i < state.botCount; i++ {
			moves[i] = DirNone
		}
		for b := 0; b < myGene.numBots; b++ {
			idx := myGene.botIDs[b]
			if state.bots[idx].alive {
				moves[idx] = sanitize(&state.bots[idx], myGene.moves[t][b])
			}
		}
		for b := 0; b < oppGene.numBots; b++ {
			idx := oppGene.botIDs[b]
			if state.bots[idx].alive {
				moves[idx] = sanitize(&state.bots[idx], oppGene.moves[t][b])
			}
		}
		state.simTurn(&moves)
	}
	return state.evaluate(myPlayer)
}

// ---------------------------------------------------------------------------
// Genetic search
// ---------------------------------------------------------------------------

var (
	prevMyPop   [PopSize]Gene
	prevOppPop  [PopSize]Gene
	hasPrevPop  bool
	prevBestOpp int
)

func shiftPop(pop *[PopSize]Gene, rng *rand.Rand, state *SimState, player int) {
	for i := 0; i < PopSize; i++ {
		pop[i].initBots(state, player)
		for t := 0; t < MaxDepth-1; t++ {
			pop[i].moves[t] = pop[i].moves[t+1]
		}
		for b := 0; b < pop[i].numBots; b++ {
			pop[i].moves[MaxDepth-1][b] = rng.Intn(4)
		}
	}
}

func evolve(pop *[PopSize]Gene, rng *rand.Rand, maximize bool) {
	var next [PopSize]Gene
	// Elites
	var eliteIdx [Elitism]int
	for e := 0; e < Elitism; e++ {
		best := -1
		bestFit := 0
		for i := 0; i < PopSize; i++ {
			skip := false
			for j := 0; j < e; j++ {
				if eliteIdx[j] == i {
					skip = true
				}
			}
			if skip {
				continue
			}
			fit := pop[i].fitness
			if !maximize {
				fit = -fit
			}
			if best == -1 || fit > bestFit {
				bestFit = fit
				best = i
			}
		}
		eliteIdx[e] = best
		next[e] = pop[best]
	}
	for i := Elitism; i < PopSize; i++ {
		a := rng.Intn(PopSize)
		b := rng.Intn(PopSize)
		if maximize {
			if pop[b].fitness > pop[a].fitness {
				a, b = b, a
			}
		} else {
			if pop[b].fitness < pop[a].fitness {
				a, b = b, a
			}
		}
		c := rng.Intn(PopSize)
		d := rng.Intn(PopSize)
		if maximize {
			if pop[d].fitness > pop[c].fitness {
				c, d = d, c
			}
		} else {
			if pop[d].fitness < pop[c].fitness {
				c, d = d, c
			}
		}
		crossoverGenes(&pop[a], &pop[c], &next[i], rng)
		next[i].mutate(rng)
	}
	*pop = next
}

func geneticSearch(base *SimState, myPlayer int, deadline time.Time) [4]int {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))

	var myPop, oppPop [PopSize]Gene

	if hasPrevPop {
		myPop = prevMyPop
		oppPop = prevOppPop
		shiftPop(&myPop, rng, base, myPlayer)
		shiftPop(&oppPop, rng, base, 1-myPlayer)
	} else {
		// Seed with greedy heuristic
		seedCount := 3
		for i := 0; i < seedCount; i++ {
			myPop[i].greedySeed(base, myPlayer, i)
			oppPop[i].greedySeed(base, 1-myPlayer, i)
		}
		for i := seedCount; i < PopSize; i++ {
			myPop[i].randomize(rng, base, myPlayer)
			oppPop[i].randomize(rng, base, 1-myPlayer)
		}
	}

	bestOppIdx := 0
	generations := 0

	for time.Now().Before(deadline) {
		generations++

		bestMyFit := -9999999
		bestMyIdx := 0
		timedOut := false
		for i := 0; i < PopSize; i++ {
			if i&3 == 0 && time.Now().After(deadline) {
				timedOut = true
				break
			}
			myPop[i].fitness = simulateGene(base, &myPop[i], &oppPop[bestOppIdx], myPlayer)
			if myPop[i].fitness > bestMyFit {
				bestMyFit = myPop[i].fitness
				bestMyIdx = i
			}
		}
		if timedOut {
			break
		}

		bestOppFit := 9999999
		for i := 0; i < PopSize; i++ {
			if i&3 == 0 && time.Now().After(deadline) {
				timedOut = true
				break
			}
			oppPop[i].fitness = simulateGene(base, &myPop[bestMyIdx], &oppPop[i], myPlayer)
			if oppPop[i].fitness < bestOppFit {
				bestOppFit = oppPop[i].fitness
				bestOppIdx = i
			}
		}
		if timedOut {
			break
		}

		evolve(&myPop, rng, true)
		evolve(&oppPop, rng, false)
	}

	if debug {
		fmt.Fprintf(os.Stderr, "GA: %d gens\n", generations)
	}

	prevMyPop = myPop
	prevOppPop = oppPop
	prevBestOpp = bestOppIdx
	hasPrevPop = true

	bestIdx := 0
	bestFit := myPop[0].fitness
	for i := 1; i < PopSize; i++ {
		if myPop[i].fitness > bestFit {
			bestFit = myPop[i].fitness
			bestIdx = i
		}
	}

	var result [4]int
	best := &myPop[bestIdx]
	for b := 0; b < best.numBots; b++ {
		idx := best.botIDs[b]
		result[b] = sanitize(&base.bots[idx], best.moves[0][b])
	}
	return result
}

// ---------------------------------------------------------------------------
// I/O
// ---------------------------------------------------------------------------

func parseBody(s string) []Pt {
	parts := strings.Split(s, ":")
	pts := make([]Pt, len(parts))
	for i, p := range parts {
		fmt.Sscanf(p, "%d,%d", &pts[i].x, &pts[i].y)
	}
	return pts
}

func main() {
	scanner = bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1000000), 1000000)

	var myId int
	fmt.Sscan(readline(), &myId)

	var width, height int
	fmt.Sscan(readline(), &width)
	fmt.Sscan(readline(), &height)

	gridW, gridH = width, height
	wallGrid = make([][]bool, height)
	for i := 0; i < height; i++ {
		row := readline()
		wallGrid[i] = make([]bool, width)
		for x, ch := range row {
			if ch == '#' {
				wallGrid[i][x] = true
			}
		}
	}
	precompute()

	var snakebotsPerPlayer int
	fmt.Sscan(readline(), &snakebotsPerPlayer)

	myBots := make(map[int]bool)
	for i := 0; i < snakebotsPerPlayer; i++ {
		var id int
		fmt.Sscan(readline(), &id)
		myBots[id] = true
	}
	for i := 0; i < snakebotsPerPlayer; i++ {
		readline()
	}

	firstTurn := true
	for {
		var powerSourceCount int
		fmt.Sscan(readline(), &powerSourceCount)

		var apples BitGrid
		for i := 0; i < powerSourceCount; i++ {
			var x, y int
			fmt.Sscan(readline(), &x, &y)
			apples.set(Pt{x, y})
		}

		var snakebotCount int
		fmt.Sscan(readline(), &snakebotCount)

		var state SimState
		state.apples = apples
		state.botCount = snakebotCount

		type bInfo struct{ id, localIdx, stateIdx int }
		var myInfos []bInfo
		localIdx := 0

		for i := 0; i < snakebotCount; i++ {
			var id int
			var body string
			fmt.Sscan(readline(), &id, &body)
			pts := parseBody(body)
			b := &state.bots[i]
			b.id = id
			b.alive = true
			b.bodyLen = len(pts)
			copy(b.body[:], pts)
			if myBots[id] {
				b.owner = 0
				myInfos = append(myInfos, bInfo{id, localIdx, i})
				localIdx++
			} else {
				b.owner = 1
			}
		}
		state.rebuildOcc()
		precomputeAppleDists(&state.apples)

		// Start timer after all I/O and precompute — engine latency is excluded.
		turnStart := time.Now()
		var budget time.Duration
		if firstTurn {
			budget = 950 * time.Millisecond
		} else {
			budget = 43 * time.Millisecond
		}
		deadline := turnStart.Add(budget)

		result := geneticSearch(&state, 0, deadline)

		var actions []string
		for _, info := range myInfos {
			dir := result[info.localIdx]
			actions = append(actions, fmt.Sprintf("%d %s", info.id, dirNames[dir]))
		}
		output := "WAIT"
		if len(actions) > 0 {
			output = strings.Join(actions, ";")
		}
		if debug {
			fmt.Fprintln(os.Stderr, output)
		}
		fmt.Println(output)
		firstTurn = false
	}
}
