package engine

import (
	"math/rand"
)

const MaxTurns = 200

// Game holds the full game state and implements the turn logic.
// Source: source/src/main/java/com/codingame/game/Game.java
type Game struct {
	Players []*Player
	Grid    *Grid
	Random  *rand.Rand
	Turn    int
	Losses  [2]int

	ended bool
}

func NewGame(seed int64, leagueLevel int) *Game {
	rng := rand.New(rand.NewSource(seed))
	gm := NewGridMaker(rng, leagueLevel)
	grid := gm.Make()

	g := &Game{
		Grid:   grid,
		Random: rng,
	}
	return g
}

func (g *Game) Init(players []*Player) {
	g.Players = players

	birdID := 0
	spawnLocations := g.findSpawnLocations()

	for _, p := range players {
		for _, spawn := range spawnLocations {
			bird := NewBird(birdID, p)
			birdID++
			p.birds = append(p.birds, bird)
			for _, c := range spawn {
				if p.GetIndex() == 1 {
					c = g.Grid.Opposite(c)
				}
				bird.Body = append(bird.Body, c)
			}
		}
	}
}

func (g *Game) findSpawnLocations() [][]Coord {
	islands := g.Grid.DetectSpawnIslands()
	result := make([][]Coord, len(islands))
	for i, island := range islands {
		result[i] = sortedCoords(island)
	}
	return result
}

func (g *Game) AllBirds() []*Bird {
	var birds []*Bird
	for _, p := range g.Players {
		birds = append(birds, p.birds...)
	}
	return birds
}

func (g *Game) LiveBirds() []*Bird {
	var birds []*Bird
	for _, p := range g.Players {
		for _, b := range p.birds {
			if b.Alive {
				birds = append(birds, b)
			}
		}
	}
	return birds
}

func (g *Game) ResetGameTurnData() {
	for _, p := range g.Players {
		p.Reset()
	}
}

func (g *Game) PerformGameUpdate(turn int) {
	g.Turn = turn

	g.doMoves()
	g.doEats()
	g.doBeheadings()
	g.doFalls()

	if g.IsGameOver() {
		g.ended = true
	}
}

func (g *Game) IsGameOver() bool {
	noApples := len(g.Grid.Apples) == 0
	for _, p := range g.Players {
		allDead := true
		for _, b := range p.birds {
			if b.Alive {
				allDead = false
				break
			}
		}
		if allDead {
			return true
		}
	}
	return noApples
}

func (g *Game) Ended() bool {
	return g.ended
}

func (g *Game) EndGame() {
	g.ended = true
}

// doMoves applies each bird's direction to move it one cell.
func (g *Game) doMoves() {
	for _, p := range g.Players {
		for _, bird := range p.birds {
			if !bird.Alive {
				continue
			}
			dir := bird.Direction
			if dir == DirUnset {
				dir = bird.GetFacing()
			}
			if dir == DirUnset {
				continue
			}

			newHead := bird.HeadPos().Add(dir.Coord())
			willEatApple := g.Grid.HasApple(newHead)

			if !willEatApple {
				// Remove tail
				bird.Body = bird.Body[:len(bird.Body)-1]
			}
			// Add new head
			bird.Body = append([]Coord{newHead}, bird.Body...)
		}
	}
}

// doEats removes eaten apples from the grid.
func (g *Game) doEats() {
	eaten := make(map[Coord]bool)
	for _, p := range g.Players {
		for _, bird := range p.birds {
			if bird.Alive && g.Grid.HasApple(bird.HeadPos()) {
				eaten[bird.HeadPos()] = true
			}
		}
	}
	for c := range eaten {
		g.Grid.RemoveApple(c)
	}
}

// doBeheadings checks for head collisions with walls and other bird bodies.
func (g *Game) doBeheadings() {
	liveBirds := g.LiveBirds()
	allBirds := g.AllBirds()

	var toBehead []*Bird
	for _, bird := range liveBirds {
		isInWall := g.Grid.Get(bird.HeadPos()).Type == TileWall

		isInBird := false
		for _, other := range allBirds {
			if !other.Alive {
				continue
			}
			if !other.BodyContains(bird.HeadPos()) {
				continue
			}
			if other.ID != bird.ID {
				// Head intersects with another bird
				isInBird = true
				break
			}
			// Head intersects with same bird on a pos that is not its head
			for _, part := range other.Body[1:] {
				if part == other.HeadPos() {
					isInBird = true
					break
				}
			}
			if isInBird {
				break
			}
		}

		if isInWall || isInBird {
			toBehead = append(toBehead, bird)
		}
	}

	for _, b := range toBehead {
		if len(b.Body) <= 3 {
			b.Alive = false
			g.Losses[b.Owner.GetIndex()] += len(b.Body)
		} else {
			// Behead: remove head
			b.Body = b.Body[1:]
			g.Losses[b.Owner.GetIndex()]++
		}
	}
}

// somethingSolidUnder checks if there's a wall, bird body, or apple directly below c.
func (g *Game) somethingSolidUnder(c Coord, ignoreBody []Coord) bool {
	below := c.Add(Coord{0, 1})

	for _, ic := range ignoreBody {
		if ic == below {
			return false
		}
	}

	if g.Grid.Get(below).Type == TileWall {
		return true
	}
	for _, b := range g.LiveBirds() {
		if b.BodyContains(below) {
			return true
		}
	}
	if g.Grid.HasApple(below) {
		return true
	}
	return false
}

// doFalls applies gravity to all birds.
func (g *Game) doFalls() {
	somethingFell := true
	fallDistances := make(map[int]int)
	var outOfBounds []*Bird

	for somethingFell {
		for somethingFell {
			somethingFell = false
			allBirds := g.LiveBirds()

			for _, bird := range allBirds {
				canFall := true
				for _, c := range bird.Body {
					if g.somethingSolidUnder(c, bird.Body) {
						canFall = false
						break
					}
				}
				if canFall {
					somethingFell = true
					newBody := make([]Coord, len(bird.Body))
					for i, c := range bird.Body {
						newBody[i] = c.Add(Coord{0, 1})
					}
					bird.Body = newBody
					fallDistances[bird.ID]++

					// Check out of bounds
					allOut := true
					for _, part := range bird.Body {
						if part.Y < g.Grid.Height+1 {
							allOut = false
							break
						}
					}
					if allOut {
						bird.Alive = false
						outOfBounds = append(outOfBounds, bird)
					}
				}
			}
		}
		// Handle intercoiled birds
		fell := g.doIntercoiledFalls(fallDistances, &outOfBounds)
		somethingFell = fell
	}
}

func (g *Game) doIntercoiledFalls(fallDistances map[int]int, outOfBounds *[]*Bird) bool {
	somethingFellAtSomePoint := false

	somethingFell := true
	for somethingFell {
		somethingFell = false
		intercoiledGroups := g.getIntercoiledBirds()

		for _, birds := range intercoiledGroups {
			// Build meta body
			var metaBody []Coord
			for _, b := range birds {
				metaBody = append(metaBody, b.Body...)
			}

			canFall := true
			for _, c := range metaBody {
				if g.somethingSolidUnder(c, metaBody) {
					canFall = false
					break
				}
			}

			if canFall {
				somethingFell = true
				somethingFellAtSomePoint = true
				for _, bird := range birds {
					newBody := make([]Coord, len(bird.Body))
					for i, c := range bird.Body {
						newBody[i] = c.Add(Coord{0, 1})
					}
					bird.Body = newBody
					fallDistances[bird.ID]++

					if bird.HeadPos().Y >= g.Grid.Height {
						bird.Alive = false
						*outOfBounds = append(*outOfBounds, bird)
					}
				}
			}
		}
	}
	return somethingFellAtSomePoint
}

func (g *Game) getIntercoiledBirds() [][]*Bird {
	var groups [][]*Bird
	allBirds := g.LiveBirds()
	visited := make(map[int]bool)

	for _, bird := range allBirds {
		if visited[bird.ID] {
			continue
		}
		var group []*Bird
		toVisit := []*Bird{bird}
		for len(toVisit) > 0 {
			current := toVisit[0]
			toVisit = toVisit[1:]
			if visited[current.ID] {
				continue
			}
			visited[current.ID] = true
			group = append(group, current)
			for _, other := range allBirds {
				if current == other || visited[other.ID] {
					continue
				}
				if birdsAreTouching(current, other) {
					toVisit = append(toVisit, other)
				}
			}
		}
		if len(group) > 1 {
			groups = append(groups, group)
		}
	}
	return groups
}

func birdsAreTouching(a, b *Bird) bool {
	for _, c1 := range a.Body {
		for _, c2 := range b.Body {
			if c1.ManhattanTo(c2) == 1 {
				return true
			}
		}
	}
	return false
}

// OnEnd computes final scores.
func (g *Game) OnEnd() {
	for _, p := range g.Players {
		if p.IsDeactivated() {
			p.SetScore(-1)
			continue
		}
		total := 0
		for _, b := range p.birds {
			if b.Alive {
				total += len(b.Body)
			}
		}
		p.SetScore(total)
	}

	// Tie-breaker: subtract losses
	if g.Players[0].GetScore() == g.Players[1].GetScore() && g.Players[0].GetScore() != -1 {
		for _, p := range g.Players {
			p.SetScore(p.GetScore() - g.Losses[p.GetIndex()])
		}
	}
}

func (g *Game) ShouldSkipPlayerTurn(_ *Player) bool {
	return false
}
