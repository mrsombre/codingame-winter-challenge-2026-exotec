package main

// updateEnemyBeliefs updates predictions for all enemy snakes.
// Called at the start of phaseMPC each turn.
func (d *Decision) updateEnemyBeliefs() {
	g := d.G
	p := d.P

	for _, snIdx := range d.OpSnakes {
		sn := &g.Sn[snIdx]
		b := &p.Beliefs[snIdx]
		b.SnIdx = snIdx

		if !sn.Alive || sn.Len == 0 {
			b.Target = -1
			b.ETA = -1
			b.Confidence = 0
			continue
		}

		head := sn.Body[0]

		// Check if previous prediction was correct.
		if b.PrevHead >= 0 && b.Target >= 0 && b.Confidence > 0 {
			if headMovedToward(g, b.PrevHead, head, b.Target) {
				b.Confidence++
				if b.Confidence > 10 {
					b.Confidence = 10
				}
				b.ETA--
				if b.ETA < 0 {
					b.ETA = 0
				}
			} else {
				b.Confidence = 0
				b.Target = -1
			}
		}

		// Observe current heading.
		if sn.Len >= 2 {
			b.LastDir = g.DirFromTo(sn.Body[1], sn.Body[0])
		}
		b.PrevHead = head

		// If target was eaten or confidence dropped, re-predict.
		if b.Target >= 0 {
			alive := false
			for i := 0; i < g.ANum; i++ {
				if g.Ap[i] == b.Target {
					alive = true
					break
				}
			}
			if !alive {
				b.Target = -1
				b.Confidence = 0
			}
		}

		if b.Target < 0 {
			d.predictEnemyTarget(snIdx)
		}
	}
}

// predictEnemyTarget sets the predicted target for an enemy snake.
// Uses BFS.Reach for the enemy + heading/stickiness bonuses.
func (d *Decision) predictEnemyTarget(snIdx int) {
	b := &d.P.Beliefs[snIdx]

	reach := d.BFS.Reach[snIdx]
	if len(reach) == 0 {
		b.Target = -1
		b.ETA = -1
		return
	}

	bestScore := 1 << 30
	bestApple := -1
	bestDist := -1

	for _, ri := range reach {
		score := ri.Dist * 10

		// Heading bonus: first step aligns with current direction.
		if b.LastDir >= 0 && ri.FirstDir == b.LastDir {
			score -= 8
		}

		// Crowded penalty: another enemy also wants this apple.
		for _, otherIdx := range d.OpSnakes {
			if otherIdx == snIdx {
				continue
			}
			ob := &d.P.Beliefs[otherIdx]
			if ob.Target == ri.Apple && ob.Confidence > 0 {
				score += 5
			}
		}

		if score < bestScore {
			bestScore = score
			bestApple = ri.Apple
			bestDist = ri.Dist
		}
	}

	b.Target = bestApple
	b.ETA = bestDist
	b.Confidence = 1
}

// headMovedToward returns true if head moved closer to target (Manhattan).
func headMovedToward(g *Game, prevHead, curHead, target int) bool {
	if prevHead < 0 || curHead < 0 || target < 0 {
		return false
	}
	px, py := g.XY(prevHead)
	cx, cy := g.XY(curHead)
	tx, ty := g.XY(target)
	oldDist := abs(px-tx) + abs(py-ty)
	newDist := abs(cx-tx) + abs(cy-ty)
	return newDist < oldDist
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
