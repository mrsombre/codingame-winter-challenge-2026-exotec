package main

// --- Phase 1: Gravity-aware BFS ---

func (d *Decision) phaseBFS() {
	g := d.G
	p := d.P

	d.MySnakes = d.MySnakes[:0]
	d.BFS = d.BFS[:0]
	d.OpSnakes = d.OpSnakes[:0]
	d.OpBFS = d.OpBFS[:0]

	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive || sn.Body[0] < 0 {
			continue
		}
		results := p.BFSFindAll(sn.Body)
		if sn.Owner == 0 {
			d.MySnakes = append(d.MySnakes, i)
			d.BFS = append(d.BFS, results)
		} else {
			d.OpSnakes = append(d.OpSnakes, i)
			d.OpBFS = append(d.OpBFS, results)
		}
	}
}
