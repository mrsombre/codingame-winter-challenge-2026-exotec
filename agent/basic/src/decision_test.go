package main

func testDecision() (*Game, *Plan, *Decision) {
	g := testGameFull()
	p := &Plan{g: g}
	p.Precompute()
	return g, p, &Decision{G: g, P: p}
}
