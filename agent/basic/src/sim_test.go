package main

func testMovePlan(gridLines []string, apples [][2]int) (*Game, *Plan) {
	g := testGridInput(gridLines)
	p := &Plan{Sim: NewSim(g)}
	g.Ap = g.Ap[:0]
	for _, a := range apples {
		g.Ap = append(g.Ap, g.Idx(a[0], a[1]))
	}
	g.ANum = len(g.Ap)
	p.Precompute()
	p.RebuildAppleMap()
	return g, p
}
