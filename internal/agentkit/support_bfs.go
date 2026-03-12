package agentkit

// SBResult holds the result of a support-node BFS pathfinding.
type SBResult struct {
	Waypoints []Point // support cells (wall below) along the path, start to approach
	Approach  Point   // head position adjacent to target
	MinLen    int     // minimum body length needed (max unsupported run on path)
	Dist      int     // total cell-level steps
}

// BodyInitRun computes the initial unsupported run from the head of a body.
// Returns 1 if head is supported, higher if body stretches to support.
// Returns -1 if no body part has support (body is falling).
func (t *STerrain) BodyInitRun(body []Point) int {
	for i, p := range body {
		if t.Grid.WBelow(p) {
			return i + 1
		}
	}
	return -1
}

// SupPathBFS finds the minimum-body-length path from start to a cell adjacent
// to target, tracking support waypoints along the path.
// initRun is the unsupported run at start (1 if supported, higher if body
// stretches back to support). Returns nil if unreachable.
// Uses pre-allocated SBBuf — not safe for concurrent calls on same STerrain.
func (t *STerrain) SupPathBFS(start Point, initRun int, target Point, apples *BitGrid) *SBResult {
	if t.Grid.IsWall(target) || t.Grid.IsWall(start) {
		return nil
	}

	adj, adjN := adjCells(t.Grid, target)
	if adjN == 0 {
		return nil
	}

	if initRun < 1 {
		initRun = 1
	}

	buf := &t.SBBuf
	maxLen := buf.MaxLen
	if initRun > maxLen {
		return nil
	}

	w := t.Grid.Width
	stride := maxLen + 1
	buf.Reset()

	sKey := t.Grid.CIdx(start)*stride + initRun
	buf.Mark(sKey)
	buf.SetPrev(sKey, -1) // -1 = start sentinel
	buf.Buckets[initRun] = append(buf.Buckets[initRun], SBEntry{start, initRun, 0})

	for L := initRun; L <= maxLen; L++ {
		for i := 0; i < len(buf.Buckets[L]); i++ {
			cur := buf.Buckets[L][i]
			curKey := t.Grid.CIdx(cur.Pos)*stride + cur.Run

			for a := 0; a < adjN; a++ {
				if cur.Pos == adj[a] {
					return t.sbReconstruct(curKey, w, stride, cur.Pos, L, cur.Dist)
				}
			}

			for dir := DirUp; dir <= DirLeft; dir++ {
				next := Add(cur.Pos, DirDelta[dir])
				if t.Grid.IsWall(next) {
					continue
				}

				nr := cur.Run + 1
				if t.Grid.WBelow(next) || (apples != nil && apples.Has(Point{X: next.X, Y: next.Y + 1})) {
					nr = 1
				}
				if nr > maxLen {
					continue
				}

				nKey := t.Grid.CIdx(next)*stride + nr
				if buf.Seen(nKey) {
					continue
				}
				buf.Mark(nKey)
				buf.SetPrev(nKey, int32(curKey))

				cost := L
				if nr > L {
					cost = nr
				}
				buf.Buckets[cost] = append(buf.Buckets[cost], SBEntry{next, nr, cur.Dist + 1})
			}
		}
	}

	return nil
}

// SupReachMulti does a single support-aware BFS from start and returns all
// targets reachable with MinLen <= maxBodyLen. Replaces N separate SupPathBFS
// calls. No path reconstruction, no prev tracking — much lighter per-cell work.
// Uses pre-allocated ImmBuf — not safe for concurrent calls on same STerrain.
func (t *STerrain) SupReachMulti(start Point, initRun, maxBodyLen int, targets []Point, apples *BitGrid) []Point {
	g := t.Grid
	if g.IsWall(start) || len(targets) == 0 {
		return nil
	}
	if initRun < 1 {
		initRun = 1
	}

	buf := &t.ImmBuf
	capLen := maxBodyLen
	if capLen > buf.MaxLen {
		capLen = buf.MaxLen
	}
	if initRun > capLen {
		return nil
	}

	// Mark target positions (skip walls).
	tgtBG := NewBG(g.Width, g.Height)
	remaining := 0
	for _, tgt := range targets {
		if !g.IsWall(tgt) {
			tgtBG.Set(tgt)
			remaining++
		}
	}
	if remaining == 0 {
		return nil
	}

	stride := buf.MaxLen + 1
	buf.Reset()

	sKey := g.CIdx(start)*stride + initRun
	buf.Mark(sKey)
	buf.Buckets[initRun] = append(buf.Buckets[initRun], ImmSt{start, initRun, 0})

	var result []Point

	for L := initRun; L <= capLen; L++ {
		for i := 0; i < len(buf.Buckets[L]); i++ {
			cur := buf.Buckets[L][i]

			for dir := DirUp; dir <= DirLeft; dir++ {
				next := Add(cur.Pos, DirDelta[dir])

				// Target adjacency check: is neighbor an unfound target?
				if tgtBG.Has(next) {
					tgtBG.Clear(next)
					result = append(result, next)
					remaining--
					if remaining == 0 {
						return result
					}
				}

				// BFS expansion.
				if g.IsWall(next) {
					continue
				}
				nr := cur.Run + 1
				if g.WBelow(next) || (apples != nil && apples.Has(Point{X: next.X, Y: next.Y + 1})) {
					nr = 1
				}
				if nr > capLen {
					continue
				}
				nKey := g.CIdx(next)*stride + nr
				if buf.Seen(nKey) {
					continue
				}
				buf.Mark(nKey)
				cost := L
				if nr > L {
					cost = nr
				}
				buf.Buckets[cost] = append(buf.Buckets[cost], ImmSt{next, nr, cur.Dist + 1})
			}
		}
	}

	return result
}

func (t *STerrain) sbReconstruct(goalKey, w, stride int, approach Point, minLen, dist int) *SBResult {
	buf := &t.SBBuf

	// Follow prev pointers to reconstruct path, extract support waypoints
	waypoints := make([]Point, 0, 8)
	k := goalKey
	for {
		posIdx := k / stride
		pos := Point{X: posIdx % w, Y: posIdx / w}
		if t.Grid.WBelow(pos) {
			waypoints = append(waypoints, pos)
		}
		prev, ok := buf.GetPrev(k)
		if !ok || prev == -1 {
			break
		}
		k = int(prev)
	}

	// Reverse to start-to-goal order
	for i, j := 0, len(waypoints)-1; i < j; i, j = i+1, j-1 {
		waypoints[i], waypoints[j] = waypoints[j], waypoints[i]
	}

	return &SBResult{
		Waypoints: waypoints,
		Approach:  approach,
		MinLen:    minLen,
		Dist:      dist,
	}
}
