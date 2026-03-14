package game

// HasSupport reports whether any body part has physical support:
//   - wall directly below (g.WBelow),
//   - occupied cell directly below (not part of the same body), or
//   - source directly below that was not just eaten.
func HasSupport(g *AGrid, body []Point, sources, occupied *BitGrid, eaten *Point) bool {
	for _, part := range body {
		below := Point{X: part.X, Y: part.Y + 1}
		isBody := false
		for _, bp := range body {
			if bp == below {
				isBody = true
				break
			}
		}
		if isBody {
			continue
		}
		if g.WBelow(part) {
			return true
		}
		if occupied != nil && occupied.Has(below) {
			return true
		}
		if sources != nil && sources.Has(below) && (eaten == nil || below != *eaten) {
			return true
		}
	}
	return false
}

// SimMove simulates one step of body in direction dir.
// Returns (newBody, newFacing, alive, didEat, newHead).
// newBody aliases s.SimBuf — copy before calling SimMove again.
func (s *State) SimMove(body []Point, facing, dir Direction, sources, occupied *BitGrid) ([]Point, Direction, bool, bool, Point) {
	g := s.Grid
	nh := Add(body[0], DirDelta[dir])
	willEat := sources != nil && sources.Has(nh)

	n := 0
	s.SimBuf[n] = nh
	n++
	if willEat {
		copy(s.SimBuf[n:], body)
		n += len(body)
	} else {
		copy(s.SimBuf[n:], body[:len(body)-1])
		n += len(body) - 1
	}

	collision := g.IsWall(nh) || (occupied != nil && occupied.Has(nh))
	if !collision {
		for k := 1; k < n; k++ {
			if s.SimBuf[k] == nh {
				collision = true
				break
			}
		}
	}
	if collision {
		if n <= 3 {
			return nil, DirNone, false, willEat, nh
		}
		copy(s.SimBuf[:], s.SimBuf[1:n])
		n--
	}
	nb := s.SimBuf[:n]

	var eaten *Point
	if willEat {
		eaten = &nh
	}
	for {
		if HasSupport(g, nb, sources, occupied, eaten) {
			break
		}
		allOut := true
		for i := range nb {
			nb[i].Y++
			if nb[i].Y < g.Height+1 {
				allOut = false
			}
		}
		if allOut {
			return nil, DirNone, false, willEat, nh
		}
	}

	f := DirUp
	if n >= 2 {
		f = FacingPts(nb[0], nb[1])
	}
	return nb, f, true, willEat, nh
}

// SrcScore estimates movement cost from head to target.
// Penalises upward travel; rewards grounded targets.
func SrcScore(g *AGrid, head, target Point) int {
	d := MDist(head, target)
	if target.Y < head.Y {
		d += head.Y - target.Y
	}
	if g.WBelow(target) {
		d--
	}
	return d
}
