package main

import (
	"fmt"
	"math/rand"
	"sort"
	"testing"
	"time"

	enginegrid "codingame/internal/engine/grid"
)

func deterministicRng() *rand.Rand { return rand.New(rand.NewSource(42)) }

// smallMapSeed is a SHA1PRNG seed that produces a bronze-league map with
// height<=10 and exactly 2 spawn islands (2 bots per player).
// Set to 0 to auto-scan; once found the value is printed so you can hardcode it.
const smallMapSeed = int64(18) // map 18x10, 2 bots per player

func findAndBuildPerfGrid() (int64, *enginegrid.Grid) {
	if smallMapSeed != 0 {
		rng := enginegrid.NewSHA1PRNG(smallMapSeed)
		g := enginegrid.NewGridMaker(rng, 1).Make()
		return smallMapSeed, g
	}
	for seed := int64(1); ; seed++ {
		rng := enginegrid.NewSHA1PRNG(seed)
		g := enginegrid.NewGridMaker(rng, 1).Make()
		islands := g.DetectSpawnIslands()
		if g.Height <= 10 && len(islands) == 2 {
			fmt.Printf("// found seed: const smallMapSeed = int64(%d) — map %dx%d\n", seed, g.Width, g.Height)
			return seed, g
		}
	}
}

func islandToSortedPts(island map[enginegrid.Coord]bool) []Pt {
	coords := make([]enginegrid.Coord, 0, len(island))
	for c := range island {
		coords = append(coords, c)
	}
	sort.Slice(coords, func(i, j int) bool {
		if coords[i].X != coords[j].X {
			return coords[i].X < coords[j].X
		}
		return coords[i].Y < coords[j].Y
	})
	pts := make([]Pt, len(coords))
	for i, c := range coords {
		pts[i] = Pt{c.X, c.Y}
	}
	return pts
}

func buildPerfState() SimState {
	_, g := findAndBuildPerfGrid()

	gridW, gridH = g.Width, g.Height
	wallGrid = make([][]bool, gridH)
	for y := 0; y < gridH; y++ {
		wallGrid[y] = make([]bool, gridW)
		for x := 0; x < gridW; x++ {
			wallGrid[y][x] = g.GetXY(x, y).Type == enginegrid.TileWall
		}
	}
	precompute()

	islands := g.DetectSpawnIslands()
	var state SimState
	state.botCount = len(islands) * 2

	for i, island := range islands {
		pts := islandToSortedPts(island)

		b0 := &state.bots[i]
		b0.id = i
		b0.alive = true
		b0.owner = 0
		b0.bodyLen = len(pts)
		copy(b0.body[:], pts)

		b1 := &state.bots[len(islands)+i]
		b1.id = len(islands) + i
		b1.alive = true
		b1.owner = 1
		b1.bodyLen = len(pts)
		for j, p := range pts {
			opp := g.Opposite(enginegrid.Coord{X: p.x, Y: p.y})
			b1.body[j] = Pt{opp.X, opp.Y}
		}
	}

	for _, c := range g.Apples {
		state.apples.set(Pt{c.X, c.Y})
	}
	state.rebuildOcc()
	precomputeAppleDists(&state.apples)
	return state
}

func BenchmarkSimulateGene(b *testing.B) {
	state := buildPerfState()
	myGene, oppGene := Gene{}, Gene{}
	myGene.randomize(deterministicRng(), &state, 0)
	oppGene.randomize(deterministicRng(), &state, 1)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		simulateGene(&state, &myGene, &oppGene, 0)
	}
}

func BenchmarkGAGeneration(b *testing.B) {
	state := buildPerfState()
	rng := deterministicRng()
	var myPop, oppPop [PopSize]Gene
	for i := 0; i < PopSize; i++ {
		myPop[i].randomize(rng, &state, 0)
		oppPop[i].randomize(rng, &state, 1)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < PopSize; j++ {
			myPop[j].fitness = simulateGene(&state, &myPop[j], &oppPop[0], 0)
		}
		for j := 0; j < PopSize; j++ {
			oppPop[j].fitness = simulateGene(&state, &myPop[0], &oppPop[j], 0)
		}
		evolve(&myPop, rng, true)
		evolve(&oppPop, rng, false)
	}
}

func TestBudget45ms(t *testing.T) {
	const budget = 45 * time.Millisecond
	state := buildPerfState()
	fmt.Printf("map: %dx%d, bots: %d\n", gridW, gridH, state.botCount)
	myGene, oppGene := Gene{}, Gene{}
	myGene.randomize(deterministicRng(), &state, 0)
	oppGene.randomize(deterministicRng(), &state, 1)

	// Raw simulations
	{
		n := 0
		deadline := time.Now().Add(budget)
		for time.Now().Before(deadline) {
			simulateGene(&state, &myGene, &oppGene, 0)
			n++
		}
		fmt.Printf("simulateGene in 45ms: %d  (~%d/ms)\n", n, n/45)
	}

	// Full GA generations
	{
		rng := deterministicRng()
		var myPop, oppPop [PopSize]Gene
		for i := 0; i < PopSize; i++ {
			myPop[i].randomize(rng, &state, 0)
			oppPop[i].randomize(rng, &state, 1)
		}
		gens := 0
		deadline := time.Now().Add(budget)
		for time.Now().Before(deadline) {
			for j := 0; j < PopSize; j++ {
				myPop[j].fitness = simulateGene(&state, &myPop[j], &oppPop[0], 0)
			}
			for j := 0; j < PopSize; j++ {
				oppPop[j].fitness = simulateGene(&state, &myPop[0], &oppPop[j], 0)
			}
			evolve(&myPop, rng, true)
			evolve(&oppPop, rng, false)
			gens++
		}
		fmt.Printf("GA generations in 45ms: %d  (%d sims/gen × %d = %d sims total)\n",
			gens, PopSize*2, gens, PopSize*2*gens)
	}
}
