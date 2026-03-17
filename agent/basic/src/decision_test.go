package main

func testDecision() (*Game, *Plan, *Decision) {
	g := testGameFull()
	p := &Plan{Sim: NewSim(g)}
	p.Precompute()
	return g, p, &Decision{G: g, P: p}
}
