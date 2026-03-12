package agentkit

import "sort"

type SupNode struct {
	Pos  Point
	Nbrs []int
}

type TAppr struct {
	Cell Point
	MinL int
}

type STerrain struct {
	Grid    *AGrid
	NID     []int
	Nodes   []SupNode
	CompID  []int
	CompCnt int

	Cells    int
	MinLen   []int
	AprCache map[Point][]int

	ImmBuf IBuf
	SBBuf  SBBuf

	WallSup []Point
}

type IBuf struct {
	Visited []uint32
	Gen     uint32
	Buckets [][]ImmSt
	MaxLen  int
}

type ImmSt struct {
	Pos  Point
	Run  int
	Dist int
}

func (b *IBuf) Init(cells, maxLen int) {
	b.MaxLen = maxLen
	b.Visited = make([]uint32, cells*(maxLen+1))
	b.Buckets = make([][]ImmSt, maxLen+1)
}

func (b *IBuf) Reset() {
	b.Gen++
	if b.Gen == 0 {
		for i := range b.Visited {
			b.Visited[i] = 0
		}
		b.Gen = 1
	}
	for i := range b.Buckets {
		b.Buckets[i] = b.Buckets[i][:0]
	}
}

func (b *IBuf) Mark(key int) {
	b.Visited[key] = b.Gen
}

func (b *IBuf) Seen(key int) bool {
	return b.Visited[key] == b.Gen
}

// SBBuf is a pre-allocated buffer for SupPathBFS, analogous to IBuf.
type SBBuf struct {
	Visited []uint32
	Gen     uint32
	PrevSt  []int32
	PrevGen []uint32
	Buckets [][]SBEntry
	MaxLen  int
}

type SBEntry struct {
	Pos  Point
	Run  int
	Dist int
}

func (b *SBBuf) Init(cells, maxLen int) {
	b.MaxLen = maxLen
	n := cells * (maxLen + 1)
	b.Visited = make([]uint32, n)
	b.PrevSt = make([]int32, n)
	b.PrevGen = make([]uint32, n)
	b.Buckets = make([][]SBEntry, maxLen+1)
}

func (b *SBBuf) Reset() {
	b.Gen++
	if b.Gen == 0 {
		for i := range b.Visited {
			b.Visited[i] = 0
		}
		for i := range b.PrevGen {
			b.PrevGen[i] = 0
		}
		b.Gen = 1
	}
	for i := range b.Buckets {
		b.Buckets[i] = b.Buckets[i][:0]
	}
}

func (b *SBBuf) Mark(key int)    { b.Visited[key] = b.Gen }
func (b *SBBuf) Seen(key int) bool { return b.Visited[key] == b.Gen }

func (b *SBBuf) SetPrev(key int, prev int32) {
	b.PrevSt[key] = prev
	b.PrevGen[key] = b.Gen
}

func (b *SBBuf) GetPrev(key int) (int32, bool) {
	if b.PrevGen[key] != b.Gen {
		return -1, false
	}
	return b.PrevSt[key], true
}

// adjCells returns non-wall cardinal neighbors of target (possible approach positions).
func adjCells(g *AGrid, target Point) ([4]Point, int) {
	var adj [4]Point
	n := 0
	for dir := DirUp; dir <= DirLeft; dir++ {
		p := Add(target, DirDelta[dir])
		if !g.IsWall(p) {
			adj[n] = p
			n++
		}
	}
	return adj, n
}

func NewSTerrain(grid *AGrid) *STerrain {
	t := &STerrain{
		Grid:     grid,
		NID:      make([]int, grid.Width*grid.Height),
		Cells:    grid.Width * grid.Height,
		AprCache: map[Point][]int{},
	}
	for i := range t.NID {
		t.NID[i] = -1
	}

	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			p := Point{X: x, Y: y}
			if grid.IsWall(p) || !t.HasSup(p) {
				continue
			}
			t.NID[grid.CIdx(p)] = len(t.Nodes)
			t.Nodes = append(t.Nodes, SupNode{Pos: p})
		}
	}

	for id := range t.Nodes {
		p := t.Nodes[id].Pos
		nbrs := make([]int, 0, 8)
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				if nid := t.NodeAt(Point{X: p.X + dx, Y: p.Y + dy}); nid != -1 {
					nbrs = append(nbrs, nid)
				}
			}
		}
		t.Nodes[id].Nbrs = nbrs
	}

	t.CompID = make([]int, len(t.Nodes))
	for i := range t.CompID {
		t.CompID[i] = -1
	}
	queue := make([]int, 0, len(t.Nodes))
	for id := range t.Nodes {
		if t.CompID[id] != -1 {
			continue
		}
		cid := t.CompCnt
		t.CompCnt++
		t.CompID[id] = cid
		queue = queue[:0]
		queue = append(queue, id)
		for i := 0; i < len(queue); i++ {
			for _, next := range t.Nodes[queue[i]].Nbrs {
				if t.CompID[next] != -1 {
					continue
				}
				t.CompID[next] = cid
				queue = append(queue, next)
			}
		}
	}

	for y := 1; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			sup := Point{X: x, Y: y}
			if !grid.IsWall(sup) {
				continue
			}
			stand := Point{X: x, Y: y - 1}
			if grid.IsWall(stand) {
				continue
			}
			t.WallSup = append(t.WallSup, sup)
		}
	}

	t.CMinLen()

	maxLen := grid.Width + grid.Height
	if maxLen < 1 {
		maxLen = 1
	}
	t.ImmBuf.Init(t.Cells, maxLen)
	t.SBBuf.Init(t.Cells, maxLen)

	return t
}

func (t *STerrain) CMinLen() {
	maxRun := t.Grid.Width + t.Grid.Height
	if maxRun < 1 {
		maxRun = 1
	}
	t.MinLen = make([]int, t.CompCnt*t.Cells)
	for i := range t.MinLen {
		t.MinLen[i] = Unreachable
	}

	type st struct {
		p Point
		r int
	}

	for comp := 0; comp < t.CompCnt; comp++ {
		off := comp * t.Cells
		visited := make([]bool, t.Cells*(maxRun+1))
		buckets := make([][]st, maxRun+1)

		for id, node := range t.Nodes {
			if t.CompID[id] != comp {
				continue
			}
			idx := t.Grid.CIdx(node.Pos)
			visited[idx*(maxRun+1)+1] = true
			buckets[1] = append(buckets[1], st{p: node.Pos, r: 1})
		}

		for L := 1; L <= maxRun; L++ {
			for i := 0; i < len(buckets[L]); i++ {
				cur := buckets[L][i]
				idx := t.Grid.CIdx(cur.p)
				if t.MinLen[off+idx] > L {
					t.MinLen[off+idx] = L
				}
				for dir := DirUp; dir <= DirLeft; dir++ {
					next := Add(cur.p, DirDelta[dir])
					if t.Grid.IsWall(next) {
						continue
					}
					nr := cur.r + 1
					if t.Grid.WBelow(next) {
						nr = 1
					}
					if nr > maxRun {
						continue
					}
					vKey := t.Grid.CIdx(next)*(maxRun+1) + nr
					if visited[vKey] {
						continue
					}
					visited[vKey] = true
					cost := L
					if nr > L {
						cost = nr
					}
					buckets[cost] = append(buckets[cost], st{p: next, r: nr})
				}
			}
		}
	}
}

func (t *STerrain) HasSup(p Point) bool {
	return t.Grid.WBelow(p)
}

func (t *STerrain) NodeAt(p Point) int {
	if !t.Grid.InB(p) {
		return -1
	}
	return t.NID[t.Grid.CIdx(p)]
}

func (t *STerrain) Anchor(body []Point) int {
	for _, part := range body {
		if !t.HasSup(part) {
			continue
		}
		if id := t.NodeAt(part); id != -1 {
			return id
		}
	}
	return -1
}

func (t *STerrain) AnchorComp(body []Point) int {
	if nid := t.Anchor(body); nid != -1 {
		return t.CompID[nid]
	}
	return -1
}

// ApprNodes returns support node IDs near target. Cached; do not mutate result.
func (t *STerrain) ApprNodes(target Point) []int {
	if cached, ok := t.AprCache[target]; ok {
		return cached
	}

	var nodes []int
	n := 0
	if id := t.NodeAt(target); id != -1 {
		nodes = append(nodes, id)
		n++
	}
	for dir := DirUp; dir <= DirLeft; dir++ {
		id := t.NodeAt(Add(target, DirDelta[dir]))
		if id == -1 {
			continue
		}
		dup := false
		for j := 0; j < n; j++ {
			if nodes[j] == id {
				dup = true
				break
			}
		}
		if !dup {
			nodes = append(nodes, id)
			n++
		}
	}
	if n == 0 {
		for span := 1; span <= 2 && n == 0; span++ {
			bestY := Unreachable
			for id, node := range t.Nodes {
				p := node.Pos
				if p.Y < target.Y || abs(p.X-target.X) > span {
					continue
				}
				if p.Y < bestY {
					bestY = p.Y
					nodes = nodes[:0]
					n = 0
					nodes = append(nodes, id)
					n++
					continue
				}
				if p.Y == bestY {
					dup := false
					for j := 0; j < n; j++ {
						if nodes[j] == id {
							dup = true
							break
						}
					}
					if !dup {
						nodes = append(nodes, id)
						n++
					}
				}
			}
		}
	}

	t.AprCache[target] = nodes
	return nodes
}

func (t *STerrain) MinSoloLen(compID int, target Point) int {
	if compID < 0 || compID >= t.CompCnt || t.Grid.IsWall(target) {
		return Unreachable
	}
	return t.MinLen[compID*t.Cells+t.Grid.CIdx(target)]
}

func (t *STerrain) MinBodyLen(body []Point, target Point) int {
	return t.MinSoloLen(t.AnchorComp(body), target)
}

// --- Apple-aware support finding ---

func (t *STerrain) MinImmLen(supCell, target Point, apples *BitGrid) (int, int) {
	stand := Point{X: supCell.X, Y: supCell.Y - 1}
	if stand.Y < 0 || t.Grid.IsWall(stand) {
		return Unreachable, Unreachable
	}

	adj, adjN := adjCells(t.Grid, target)
	if adjN == 0 {
		return Unreachable, Unreachable
	}

	buf := &t.ImmBuf
	maxLen := buf.MaxLen
	buf.Reset()

	startIdx := t.Grid.CIdx(stand)
	buf.Mark(startIdx*(maxLen+1) + 1)
	buf.Buckets[1] = append(buf.Buckets[1], ImmSt{Pos: stand, Run: 1, Dist: 0})

	for L := 1; L <= maxLen; L++ {
		for i := 0; i < len(buf.Buckets[L]); i++ {
			cur := buf.Buckets[L][i]
			for a := 0; a < adjN; a++ {
				if cur.Pos == adj[a] {
					return L, cur.Dist
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

				vKey := t.Grid.CIdx(next)*(maxLen+1) + nr
				if buf.Seen(vKey) {
					continue
				}
				buf.Mark(vKey)

				cost := L
				if nr > L {
					cost = nr
				}
				buf.Buckets[cost] = append(buf.Buckets[cost], ImmSt{Pos: next, Run: nr, Dist: cur.Dist + 1})
			}
		}
	}

	return Unreachable, Unreachable
}

func (t *STerrain) TAppr(apples *BitGrid, target Point) []TAppr {
	if t.Grid.IsWall(target) {
		return nil
	}

	var appr []TAppr

	for _, sup := range t.WallSup {
		if sup == target {
			continue
		}
		ml, _ := t.MinImmLen(sup, target, apples)
		if ml != Unreachable {
			appr = append(appr, TAppr{Cell: sup, MinL: ml})
		}
	}

	if apples != nil {
		for y := 0; y < t.Grid.Height; y++ {
			for x := 0; x < t.Grid.Width; x++ {
				p := Point{X: x, Y: y}
				if p == target || t.Grid.IsWall(p) || !apples.Has(p) {
					continue
				}
				stand := Point{X: x, Y: y - 1}
				if stand.Y < 0 || t.Grid.IsWall(stand) {
					continue
				}
				ml, _ := t.MinImmLen(p, target, apples)
				if ml != Unreachable {
					appr = append(appr, TAppr{Cell: p, MinL: ml})
				}
			}
		}
	}

	sort.Slice(appr, func(i, j int) bool {
		if appr[i].MinL != appr[j].MinL {
			return appr[i].MinL < appr[j].MinL
		}
		if appr[i].Cell.Y != appr[j].Cell.Y {
			return appr[i].Cell.Y < appr[j].Cell.Y
		}
		return appr[i].Cell.X < appr[j].Cell.X
	})

	return appr
}

func (t *STerrain) Closest(apples *BitGrid, target Point) []TAppr {
	appr := t.TAppr(apples, target)
	if len(appr) == 0 {
		return nil
	}

	sups := make(map[Point]TAppr, len(appr))
	for _, a := range appr {
		sups[a.Cell] = a
	}

	compOf := make(map[Point]int, len(sups))
	queue := make([]Point, 0, len(sups))
	compCnt := 0
	for sup := range sups {
		if _, seen := compOf[sup]; seen {
			continue
		}
		compOf[sup] = compCnt
		queue = queue[:0]
		queue = append(queue, sup)
		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			curSt := Point{X: cur.X, Y: cur.Y - 1}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					next := Point{X: cur.X + dx, Y: cur.Y + dy}
					if _, ok := sups[next]; !ok {
						continue
					}
					if _, seen := compOf[next]; seen {
						continue
					}
					nextSt := Point{X: next.X, Y: next.Y - 1}
					if abs(curSt.X-nextSt.X) > 1 || abs(curSt.Y-nextSt.Y) > 1 {
						continue
					}
					compOf[next] = compCnt
					queue = append(queue, next)
				}
			}
		}
		compCnt++
	}

	bestByComp := make(map[int]TAppr, compCnt)
	for _, a := range appr {
		cid := compOf[a.Cell]
		best, ok := bestByComp[cid]
		if !ok ||
			a.MinL < best.MinL ||
			(a.MinL == best.MinL &&
				MDist(a.Cell, target) < MDist(best.Cell, target)) ||
			(a.MinL == best.MinL &&
				MDist(a.Cell, target) == MDist(best.Cell, target) &&
				(a.Cell.Y < best.Cell.Y ||
					(a.Cell.Y == best.Cell.Y && a.Cell.X < best.Cell.X))) {
			bestByComp[cid] = a
		}
	}

	out := make([]TAppr, 0, len(bestByComp))
	for _, a := range bestByComp {
		out = append(out, a)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].MinL != out[j].MinL {
			return out[i].MinL < out[j].MinL
		}
		di := MDist(out[i].Cell, target)
		dj := MDist(out[j].Cell, target)
		if di != dj {
			return di < dj
		}
		if out[i].Cell.Y != out[j].Cell.Y {
			return out[i].Cell.Y < out[j].Cell.Y
		}
		return out[i].Cell.X < out[j].Cell.X
	})

	return out
}

func TgtAppr(state *State, target Point) []TAppr {
	if state == nil || state.Grid == nil || state.Terr == nil {
		return nil
	}
	return state.Terr.TAppr(&state.Apples, target)
}

func CloseSup(state *State, target Point) []TAppr {
	if state == nil || state.Grid == nil || state.Terr == nil {
		return nil
	}
	return state.Terr.Closest(&state.Apples, target)
}
