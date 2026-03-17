package main

// --- Phase 1: Gravity-aware BFS ---

func (d *Decision) phaseBFS() {
	g := d.G
	p := d.P

	d.MySnakes = d.MySnakes[:0]
	d.BFS = d.BFS[:0]
	d.OpSnakes = d.OpSnakes[:0]
	d.OpBFS = d.OpBFS[:0]
	d.SimTargets = d.SimTargets[:0]

	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive || sn.Body[0] < 0 {
			continue
		}
		// Simple BFS for all snakes (influence, safety, enemy distances).
		results := p.BFSFindAllSimple(sn.Body)
		if sn.Owner == 0 {
			d.MySnakes = append(d.MySnakes, i)
			d.BFS = append(d.BFS, results)
			// SimBFS for my snakes: accurate body-sim target finding.
			d.SimTargets = append(d.SimTargets, p.SimBFS(sn.Body))
		} else {
			d.OpSnakes = append(d.OpSnakes, i)
			d.OpBFS = append(d.OpBFS, results)
		}
	}
}

// --- BFS ---

// BFSFindAll runs chain-aware BFS (tracks apple eating along path).
func (p *Plan) BFSFindAll(body []int) []PathResult {
	return p.bfsFindAll(body, true)
}

// BFSFindAllSimple runs BFS without chain tracking (faster, for enemies).
func (p *Plan) BFSFindAllSimple(body []int) []PathResult {
	return p.bfsFindAll(body, false)
}

func (p *Plan) bfsFindAll(body []int, trackChain bool) []PathResult {
	g := p.G
	n := g.NCells
	bodyLen := len(body)
	if bodyLen == 0 {
		return nil
	}
	head := body[0]
	if !g.IsInGrid(head) {
		return nil
	}

	maxAG := bodyLen
	if maxAG > MaxAG {
		maxAG = MaxAG
	}

	p.RebuildAppleMap()

	results := make([]PathResult, n)
	for i := range results {
		results[i].Dist = -1
	}

	p.curGen++
	if p.curGen == 0 {
		for i := range p.visitGen {
			p.visitGen[i] = 0
		}
		p.curGen = 1
	}

	stride := MaxChainEaten + 1

	neck := neckOf(body)
	results[head] = PathResult{Dist: 0, FirstDir: -1}
	startAG := p.initialAG(body)
	if startAG >= maxAG {
		startAG = maxAG - 1
	}
	p.visitGen[(head*MaxAG+startAG)*stride] = p.curGen

	p.queue = p.queue[:0]
	for d := 0; d < 4; d++ {
		nc := g.Nbm[head][d]
		if nc == -1 || nc == neck {
			continue
		}

		var firstEaten int8
		if trackChain && p.isApple(nc) {
			firstEaten = 1
		}

		finalCell, nag := p.simulateFirstMove(body, d)
		if finalCell < 0 || finalCell >= g.NCells {
			continue
		}
		eMaxAG := maxAG + int(firstEaten)
		if eMaxAG > MaxAG {
			eMaxAG = MaxAG
		}
		if nag >= eMaxAG {
			nag = eMaxAG - 1
		}
		vk := (finalCell*MaxAG+nag)*stride + int(firstEaten)
		if p.visitGen[vk] == p.curGen {
			continue
		}
		p.visitGen[vk] = p.curGen
		if g.IsInGrid(finalCell) {
			r := &results[finalCell]
			if r.Dist == -1 {
				*r = PathResult{Dist: 1, FirstDir: d, Apples: int(firstEaten)}
			}
		}
		p.queue = append(p.queue, bfsNode{
			cell: int16(finalCell), ag: int8(nag),
			firstDir: int8(d), dist: 1,
			eaten: firstEaten,
		})
	}

	for qi := 0; qi < len(p.queue); qi++ {
		cur := p.queue[qi]
		cc := int(cur.cell)
		cag := int(cur.ag)

		for d := 0; d < 4; d++ {
			nc := g.Nbm[cc][d]
			if nc == -1 {
				continue
			}

			newEaten := cur.eaten
			if trackChain && p.isApple(nc) && newEaten < MaxChainEaten {
				newEaten++
			}

			effectiveBodyLen := bodyLen + int(newEaten)
			eMaxAG := effectiveBodyLen
			if eMaxAG > MaxAG {
				eMaxAG = MaxAG
			}

			var nag int
			finalCell := nc

			if p.isGroundedAt(nc) {
				nag = 0
			} else {
				nag = cag + 1
			}

			if nag >= eMaxAG {
				if !trackChain && p.isApple(nc) && nag < eMaxAG+1 {
					nag = cag
				} else {
					var fallAG int
					finalCell, fallAG = p.computeFallWithBody(nc, d, effectiveBodyLen)
					nag = fallAG
					if finalCell < 0 || finalCell >= g.NCells {
						continue
					}
				}
			}

			if finalCell < 0 || finalCell >= g.NCells {
				continue
			}
			vk := (finalCell*MaxAG+nag)*stride + int(newEaten)
			if p.visitGen[vk] == p.curGen {
				continue
			}
			p.visitGen[vk] = p.curGen

			fd := int(cur.firstDir)
			newDist := cur.dist + 1

			if g.IsInGrid(finalCell) {
				r := &results[finalCell]
				if r.Dist == -1 || (int(newDist) == r.Dist && int(newEaten) > r.Apples) {
					r.Dist = int(newDist)
					r.FirstDir = fd
					r.Apples = int(newEaten)
				}
			}
			p.queue = append(p.queue, bfsNode{
				cell: int16(finalCell), ag: int8(nag),
				firstDir: int8(fd), dist: newDist,
				eaten: newEaten,
			})
		}
	}

	return results
}
