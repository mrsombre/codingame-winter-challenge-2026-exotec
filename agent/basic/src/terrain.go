package main

type SupNode struct {
	Pos  Point
	Nbrs []int
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
func (b *IBuf) Mark(key int)      { b.Visited[key] = b.Gen }
func (b *IBuf) Seen(key int) bool { return b.Visited[key] == b.Gen }

type SBBuf struct {
	Visited []uint32
	Gen     uint32
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
	b.Visited = make([]uint32, cells*(maxLen+1))
	b.Buckets = make([][]SBEntry, maxLen+1)
}
func (b *SBBuf) Reset() {
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
func (b *SBBuf) Mark(key int)      { b.Visited[key] = b.Gen }
func (b *SBBuf) Seen(key int) bool { return b.Visited[key] == b.Gen }

type STerrain struct {
	Grid    *AGrid
	NID     []int
	Nodes   []SupNode
	CompID  []int
	CompCnt int
	Cells   int
	MinLen  []int
	ImmBuf  IBuf
	SBBuf   SBBuf
}

func NewSTerrain(grid *AGrid) *STerrain {
	t := &STerrain{
		Grid:  grid,
		NID:   make([]int, grid.Width*grid.Height),
		Cells: grid.Width * grid.Height,
	}
	for i := range t.NID {
		t.NID[i] = -1
	}
	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			p := Point{x, y}
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
				if nid := t.NodeAt(Point{p.X + dx, p.Y + dy}); nid != -1 {
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
			buckets[1] = append(buckets[1], st{node.Pos, 1})
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
					if t.Grid.IsWallFast(next) {
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
					buckets[cost] = append(buckets[cost], st{next, nr})
				}
			}
		}
	}
}

func (t *STerrain) HasSup(p Point) bool { return t.Grid.WBelow(p) }
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
func (t *STerrain) MinSoloLen(compID int, target Point) int {
	if compID < 0 || compID >= t.CompCnt || t.Grid.IsWall(target) {
		return Unreachable
	}
	return t.MinLen[compID*t.Cells+t.Grid.CIdx(target)]
}
func (t *STerrain) MinBodyLen(body []Point, target Point) int {
	return t.MinSoloLen(t.AnchorComp(body), target)
}
func (t *STerrain) MinImmLen(supCell, target Point, apples *BitGrid) (int, int) {
	stand := Point{supCell.X, supCell.Y - 1}
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
	buf.Buckets[1] = append(buf.Buckets[1], ImmSt{stand, 1, 0})
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
				if t.Grid.IsWallFast(next) {
					continue
				}
				nr := cur.Run + 1
				if t.Grid.WBelow(next) || (apples != nil && apples.Has(Point{next.X, next.Y + 1})) {
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
				buf.Buckets[cost] = append(buf.Buckets[cost], ImmSt{next, nr, cur.Dist + 1})
			}
		}
	}
	return Unreachable, Unreachable
}
func (t *STerrain) BodyInitRun(body []Point) int {
	for i, p := range body {
		if t.Grid.WBelow(p) {
			return i + 1
		}
	}
	return -1
}
func (t *STerrain) SupPathBFS(start Point, initRun int, target Point, apples *BitGrid) int {
	if t.Grid.IsWall(target) || t.Grid.IsWall(start) {
		return Unreachable
	}
	adj, adjN := adjCells(t.Grid, target)
	if adjN == 0 {
		return Unreachable
	}
	if initRun < 1 {
		initRun = 1
	}
	buf := &t.SBBuf
	maxLen := buf.MaxLen
	if initRun > maxLen {
		return Unreachable
	}
	stride := maxLen + 1
	buf.Reset()
	sKey := t.Grid.CIdx(start)*stride + initRun
	buf.Mark(sKey)
	buf.Buckets[initRun] = append(buf.Buckets[initRun], SBEntry{start, initRun, 0})
	for L := initRun; L <= maxLen; L++ {
		for i := 0; i < len(buf.Buckets[L]); i++ {
			cur := buf.Buckets[L][i]
			for a := 0; a < adjN; a++ {
				if cur.Pos == adj[a] {
					return L
				}
			}
			for dir := DirUp; dir <= DirLeft; dir++ {
				next := Add(cur.Pos, DirDelta[dir])
				if t.Grid.IsWallFast(next) {
					continue
				}
				nr := cur.Run + 1
				if t.Grid.WBelow(next) || (apples != nil && apples.Has(Point{next.X, next.Y + 1})) {
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
				cost := L
				if nr > L {
					cost = nr
				}
				buf.Buckets[cost] = append(buf.Buckets[cost], SBEntry{next, nr, cur.Dist + 1})
			}
		}
	}
	return Unreachable
}

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

				if tgtBG.Has(next) {
					tgtBG.Clear(next)
					result = append(result, next)
					remaining--
					if remaining == 0 {
						return result
					}
				}

				if g.IsWallFast(next) {
					continue
				}
				nr := cur.Run + 1
				if g.WBelow(next) || (apples != nil && apples.Has(Point{next.X, next.Y + 1})) {
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
