// Package engine
// Source: source/src/main/java/com/codingame/game/Game.java
package engine

const MaxTurns = 200

// Game holds the full game state and implements the turn logic.
// Source: source/src/main/java/com/codingame/game/Game.java
type Game struct {
	Players []*Player
	Grid    *Grid
	Random  Rng
	Turn    int
	Losses  [2]int

	ended bool
}

func NewGame(seed int64, leagueLevel int) *Game {
	rng := NewSHA1PRNG(seed)
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
				if len(bird.Body) == 1 {
					left := c.AddXY(-1, 0)
					right := c.AddXY(1, 0)
					if g.Grid.Get(left).Type == TileWall && g.Grid.Get(right).Type == TileWall {
						g.Grid.Get(left).Clear()
						g.Grid.Get(g.Grid.Opposite(left)).Clear()
					}
				}
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

func (g *Game) hasTileOrAppleUnder(c Coord) bool {
	below := c.Add(Coord{0, 1})
	if g.Grid.Get(below).Type == TileWall {
		return true
	}
	return g.Grid.HasApple(below)
}

func (g *Game) isGrounded(c Coord, frozenBirds map[*Bird]bool) bool {
	under := c.Add(Coord{0, 1})
	if g.hasTileOrAppleUnder(c) {
		return true
	}
	for bird := range frozenBirds {
		if bird.BodyContains(under) {
			return true
		}
	}
	return false
}

// doFalls applies gravity to all birds.
func (g *Game) doFalls() {
	somethingFell := true
	fallDistances := make(map[int]int)
	var outOfBounds []*Bird
	airborneBirds := make(map[*Bird]bool)
	for _, bird := range g.LiveBirds() {
		airborneBirds[bird] = true
	}
	groundedBirds := make(map[*Bird]bool)

	for somethingFell {
		somethingFell = false
		somethingGotGrounded := true

		for somethingGotGrounded {
			somethingGotGrounded = false
			var newlyGrounded []*Bird
			for bird := range airborneBirds {
				isGrounded := false
				for _, c := range bird.Body {
					if g.isGrounded(c, groundedBirds) {
						isGrounded = true
						break
					}
				}
				if isGrounded {
					newlyGrounded = append(newlyGrounded, bird)
				}
			}
			if len(newlyGrounded) > 0 {
				somethingGotGrounded = true
				for _, bird := range newlyGrounded {
					groundedBirds[bird] = true
					delete(airborneBirds, bird)
				}
			}
		}

		for bird := range airborneBirds {
			somethingFell = true
			newBody := make([]Coord, len(bird.Body))
			for i, c := range bird.Body {
				newBody[i] = c.Add(Coord{0, 1})
			}
			bird.Body = newBody
			fallDistances[bird.ID]++

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

		for _, bird := range outOfBounds {
			delete(airborneBirds, bird)
		}
	}
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
