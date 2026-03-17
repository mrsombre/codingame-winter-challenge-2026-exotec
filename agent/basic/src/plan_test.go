package main

func testPlan() *Plan {
	g := testGameFull()
	p := &Plan{G: g}
	return p
}
