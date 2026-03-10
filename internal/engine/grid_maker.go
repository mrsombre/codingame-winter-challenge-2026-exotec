package engine

import (
	"math"
	"math/rand"
	"sort"
)

const (
	MinGridHeight = 10
	MaxGridHeight = 24
	AspectRatio   = 1.8
	SpawnHeight   = 3
	DesiredSpawns = 4
)

type GridMaker struct {
	random      *rand.Rand
	leagueLevel int
}

func NewGridMaker(random *rand.Rand, leagueLevel int) *GridMaker {
	return &GridMaker{random: random, leagueLevel: leagueLevel}
}

func (gm *GridMaker) Make() *Grid {
	var skew float64
	switch gm.leagueLevel {
	case 1:
		skew = 2 // bronze
	case 2:
		skew = 1 // silver
	case 3:
		skew = 0.8 // gold
	default:
		skew = 0.3 // legend
	}

	r := gm.random.Float64()
	height := MinGridHeight + int(math.Round(math.Pow(r, skew)*float64(MaxGridHeight-MinGridHeight)))
	width := int(math.Round(float64(height) * AspectRatio))
	if width%2 != 0 {
		width++
	}
	grid := NewGrid(width, height)

	b := 5 + gm.random.Float64()*10

	// Bottom row is all walls
	for x := 0; x < width; x++ {
		grid.GetXY(x, height-1).Type = TileWall
	}

	// Generate walls row by row from bottom to top
	for y := height - 2; y >= 0; y-- {
		yNorm := float64(height-1-y) / float64(height-1)
		blockChanceEl := 1 / (yNorm + 0.1) / b

		for x := 0; x < width; x++ {
			if gm.random.Float64() < blockChanceEl {
				grid.GetXY(x, y).Type = TileWall
			}
		}
	}

	// X-symmetry: mirror left half to right
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := Coord{x, y}
			opp := grid.Opposite(c)
			grid.Get(opp).Type = grid.Get(c).Type
		}
	}

	// Fill small air pockets
	islands := grid.DetectAirPockets()
	for _, island := range islands {
		if len(island) < 10 {
			for c := range island {
				grid.Get(c).Type = TileWall
			}
		}
	}

	// Remove 1-wide gaps (cells with >=3 wall neighbors)
	somethingDestroyed := true
	for somethingDestroyed {
		somethingDestroyed = false
		for _, c := range grid.Coords() {
			if grid.Get(c).Type == TileWall {
				continue
			}
			neighbours := grid.GetNeighbours4(c)
			var wallCount int
			var destroyable []Coord
			for _, n := range neighbours {
				if grid.Get(n).Type == TileWall {
					wallCount++
					if n.Y <= c.Y {
						destroyable = append(destroyable, n)
					}
				}
			}
			if wallCount >= 3 && len(destroyable) > 0 {
				gm.shuffleCoords(destroyable)
				grid.Get(destroyable[0]).Type = TileEmpty
				grid.Get(grid.Opposite(destroyable[0])).Type = TileEmpty
				somethingDestroyed = true
			}
		}
	}

	// Sink lowest island
	island := grid.DetectLowestIsland()
	islandSet := make(map[Coord]bool)
	for _, c := range island {
		islandSet[c] = true
	}
	lowerBy := 0
	canLower := true
	for canLower {
		for x := 0; x < width; x++ {
			c := Coord{x, height - 1 - (lowerBy + 1)}
			if !islandSet[c] {
				canLower = false
				break
			}
		}
		if canLower {
			lowerBy++
		}
	}
	if lowerBy >= 2 {
		lowerBy = gm.randomIntRange(2, lowerBy+1)
	}

	for _, c := range island {
		grid.Get(c).Type = TileEmpty
		grid.Get(grid.Opposite(c)).Type = TileEmpty
	}
	for _, c := range island {
		lowered := Coord{c.X, c.Y + lowerBy}
		if grid.Get(lowered).IsValid() {
			grid.Get(lowered).Type = TileWall
			grid.Get(grid.Opposite(lowered)).Type = TileWall
		}
	}

	// Spawn apples
	for y := 0; y < height; y++ {
		for x := 0; x < width/2; x++ {
			c := Coord{x, y}
			if grid.Get(c).Type == TileEmpty && gm.random.Float64() < 0.025 {
				grid.Apples = append(grid.Apples, c)
				grid.Apples = append(grid.Apples, grid.Opposite(c))
			}
		}
	}

	// Convert lone walls to apples
	for _, c := range grid.Coords() {
		if grid.Get(c).Type == TileEmpty {
			continue
		}
		neighbours8 := grid.GetNeighbours(c, adjacency8)
		wallCount := 0
		for _, n := range neighbours8 {
			if grid.Get(n).Type == TileWall {
				wallCount++
			}
		}
		if wallCount == 0 {
			grid.Get(c).Type = TileEmpty
			grid.Get(grid.Opposite(c)).Type = TileEmpty
			grid.Apples = append(grid.Apples, c)
			grid.Apples = append(grid.Apples, grid.Opposite(c))
		}
	}

	// Find spawn locations
	potentialSpawns := make([]Coord, 0)
	for _, c := range grid.Coords() {
		if grid.Get(c).Type != TileWall {
			continue
		}
		aboves := gm.getFreeAbove(grid, c, SpawnHeight)
		if len(aboves) >= SpawnHeight {
			potentialSpawns = append(potentialSpawns, c)
		}
	}
	gm.shuffleCoords(potentialSpawns)

	desiredSpawns := DesiredSpawns
	if height <= 15 {
		desiredSpawns--
	}
	if height <= 10 {
		desiredSpawns--
	}

	for desiredSpawns > 0 && len(potentialSpawns) > 0 {
		spawn := potentialSpawns[0]
		potentialSpawns = potentialSpawns[1:]

		spawnLoc := gm.getFreeAbove(grid, spawn, SpawnHeight)
		tooClose := false
		for _, c := range spawnLoc {
			if c.X == width/2-1 || c.X == width/2 {
				tooClose = true
				break
			}
			for _, n := range grid.GetNeighbours(c, adjacency8) {
				if coordInSlice(n, grid.Spawns) || coordInSlice(grid.Opposite(n), grid.Spawns) {
					tooClose = true
					break
				}
			}
			if tooClose {
				break
			}
		}
		if tooClose {
			continue
		}

		for _, ac := range spawnLoc {
			grid.Spawns = append(grid.Spawns, ac)
			grid.RemoveApple(ac)
			grid.RemoveApple(grid.Opposite(ac))
		}
		desiredSpawns--
	}

	return grid
}

func (gm *GridMaker) getFreeAbove(grid *Grid, c Coord, by int) []Coord {
	var result []Coord
	for i := 1; i <= by; i++ {
		above := Coord{c.X, c.Y - i}
		cell := grid.Get(above)
		if cell.IsValid() && cell.Type == TileEmpty {
			result = append(result, above)
		} else {
			break
		}
	}
	return result
}

func (gm *GridMaker) shuffleCoords(coords []Coord) {
	gm.random.Shuffle(len(coords), func(i, j int) {
		coords[i], coords[j] = coords[j], coords[i]
	})
}

func (gm *GridMaker) randomIntRange(min, max int) int {
	return min + gm.random.Intn(max-min)
}

func coordInSlice(c Coord, slice []Coord) bool {
	for _, s := range slice {
		if s == c {
			return true
		}
	}
	return false
}

func sortedCoords(set map[Coord]bool) []Coord {
	coords := make([]Coord, 0, len(set))
	for c := range set {
		coords = append(coords, c)
	}
	sort.Slice(coords, func(i, j int) bool {
		return coords[i].Less(coords[j])
	})
	return coords
}
