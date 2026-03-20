package main

import (
	"bufio"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

type Direction int

const (
	DirUp Direction = iota
	DirRight
	DirDown
	DirLeft
	DirNone
)

var DirName = [4]string{"UP", "RIGHT", "DOWN", "LEFT"}
var DirDelta = [5]Point{{0, -1}, {1, 0}, {0, 1}, {-1, 0}, {0, 0}}
var oppDir = [5]Direction{DirDown, DirLeft, DirUp, DirRight, DirNone}

func Opp(dir Direction) Direction { return oppDir[dir] }

var facingTbl = [9]Direction{DirNone, DirUp, DirNone, DirLeft, DirNone, DirRight, DirNone, DirDown, DirNone}

func FacingPts(head, neck Point) Direction {
	dx := head.X - neck.X
	dy := head.Y - neck.Y
	if dx < -1 || dx > 1 || dy < -1 || dy > 1 {
		return DirNone
	}
	return facingTbl[(dy+1)*3+(dx+1)]
}

const Unreachable = 9999

type Point struct{ X, Y int }

func Add(a, b Point) Point { return Point{a.X + b.X, a.Y + b.Y} }
func MDist(a, b Point) int { return abs(a.X-b.X) + abs(a.Y-b.Y) }
func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

type BitGrid struct {
	Width, Height int
	Bits          []uint64
}

func NewBG(width, height int) BitGrid {
	return BitGrid{width, height, make([]uint64, (width*height+63)/64)}
}
func (g *BitGrid) Reset() {
	for i := range g.Bits {
		g.Bits[i] = 0
	}
}
func (g *BitGrid) Has(p Point) bool {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return false
	}
	idx := p.Y*g.Width + p.X
	return g.Bits[idx/64]&(uint64(1)<<uint(idx%64)) != 0
}
func (g *BitGrid) Set(p Point) {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return
	}
	idx := p.Y*g.Width + p.X
	g.Bits[idx/64] |= uint64(1) << uint(idx%64)
}
func (g *BitGrid) Clear(p Point) {
	if p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return
	}
	idx := p.Y*g.Width + p.X
	g.Bits[idx/64] &^= uint64(1) << uint(idx%64)
}

type AGrid struct {
	Width, Height int
	Walls, WallBl BitGrid
	CellDirs      [][]Direction
}

func NewAG(width, height int, walls map[Point]bool) *AGrid {
	g := &AGrid{
		Width: width, Height: height,
		Walls:    NewBG(width, height),
		WallBl:   NewBG(width, height),
		CellDirs: make([][]Direction, width*height),
	}
	for p := range walls {
		if g.InB(p) {
			g.Walls.Set(p)
		}
	}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			p := Point{x, y}
			if y == height-1 || g.IsWall(Point{x, y + 1}) {
				g.WallBl.Set(p)
			}
			if g.IsWall(p) {
				continue
			}
			var dirs []Direction
			for dir := DirUp; dir <= DirLeft; dir++ {
				if !g.IsWall(Add(p, DirDelta[dir])) {
					dirs = append(dirs, dir)
				}
			}
			g.CellDirs[y*width+x] = dirs
		}
	}
	return g
}
func (g *AGrid) InB(p Point) bool {
	return p.X >= 0 && p.X < g.Width && p.Y >= 0 && p.Y < g.Height
}
func (g *AGrid) IsWall(p Point) bool {
	if !g.InB(p) {
		return true
	}
	return g.Walls.Has(p)
}
func (g *AGrid) WBelow(p Point) bool { return g.WallBl.Has(p) }
func (g *AGrid) CDirs(pos Point) []Direction {
	if !g.InB(pos) || g.IsWall(pos) {
		return nil
	}
	return g.CellDirs[pos.Y*g.Width+pos.X]
}
func (g *AGrid) CIdx(p Point) int { return p.Y*g.Width + p.X }

const MaxBody = 80
const MaxBirds = 8

type fBody struct {
	parts [MaxBody + 1]Point
	len   int
}

func (b *fBody) set(pts []Point) {
	b.len = len(pts)
	copy(b.parts[:b.len], pts)
}

func (b *fBody) slice() []Point { return b.parts[:b.len] }

func (b *fBody) facing() Direction {
	if b.len < 2 {
		return DirUp
	}
	return FacingPts(b.parts[0], b.parts[1])
}

func (b *fBody) contains(p Point) bool {
	for i := 0; i < b.len; i++ {
		if b.parts[i] == p {
			return true
		}
	}
	return false
}

type State struct {
	Grid   *AGrid
	Terr   *STerrain
	Apples BitGrid
	MvBuf  [4]Direction
	DistQ  []Point
}

func NewState(grid *AGrid) State {
	s := State{Grid: grid}
	if grid != nil {
		n := grid.Width * grid.Height
		s.Terr = NewSTerrain(grid)
		s.Apples = NewBG(grid.Width, grid.Height)
		s.DistQ = make([]Point, 0, n)
	}
	return s
}
func (s *State) VMoves(pos Point, facing Direction) []Direction {
	dirs := s.Grid.CDirs(pos)
	if facing == DirNone {
		return dirs
	}
	back := Opp(facing)
	n := 0
	for _, d := range dirs {
		if d != back {
			s.MvBuf[n] = d
			n++
		}
	}
	return s.MvBuf[:n]
}

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
				if t.Grid.IsWall(next) {
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
				if t.Grid.IsWall(next) {
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

				if g.IsWall(next) {
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

func fillBG(bg *BitGrid, pts []Point) {
	bg.Reset()
	for _, p := range pts {
		bg.Set(p)
	}
}

func occExcept(base *BitGrid, body []Point) BitGrid {
	bg := NewBG(base.Width, base.Height)
	copy(bg.Bits, base.Bits)
	for _, p := range body {
		bg.Clear(p)
	}
	return bg
}

const debug = false

var (
	scanner *bufio.Scanner
	grid    *AGrid
	state   State
	rsc     refScratch
	W, H    int
)

func readline() string {
	if !scanner.Scan() {
		os.Exit(0)
	}
	line := scanner.Text()
	if debug {
		fmt.Fprintln(os.Stderr, line)
	}
	return line
}

func parseBody(s string) []Point {
	parts := strings.Split(s, ":")
	pts := make([]Point, len(parts))
	for i, p := range parts {
		fmt.Sscanf(p, "%d,%d", &pts[i].X, &pts[i].Y)
	}
	return pts
}

func hasSupport(body []Point, sources, occupied *BitGrid, eaten *Point) bool {
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
		if grid.WBelow(part) {
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

var simBuf [MaxBody + 1]Point

func simMove(body []Point, facing, dir Direction, sources, occupied *BitGrid) ([]Point, Direction, bool, bool, Point) {
	nh := Add(body[0], DirDelta[dir])
	willEat := sources != nil && sources.Has(nh)

	n := 0
	simBuf[n] = nh
	n++
	if willEat {
		copy(simBuf[n:], body)
		n += len(body)
	} else {
		copy(simBuf[n:], body[:len(body)-1])
		n += len(body) - 1
	}

	collision := grid.IsWall(nh) || (occupied != nil && occupied.Has(nh))
	if !collision {
		for k := 1; k < n; k++ {
			if simBuf[k] == nh {
				collision = true
				break
			}
		}
	}
	if collision {
		if n <= 3 {
			return nil, DirNone, false, willEat, nh
		}
		copy(simBuf[:], simBuf[1:n])
		n--
	}
	nb := simBuf[:n]

	var eaten *Point
	if willEat {
		eaten = &nh
	}
	for {
		if hasSupport(nb, sources, occupied, eaten) {
			break
		}
		allOut := true
		for i := range nb {
			nb[i].Y++
			if nb[i].Y < H+1 {
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

func floodDist(start Point, blocked *BitGrid) (int, []int) {
	n := W * H
	dist := make([]int, n)
	for i := range dist {
		dist[i] = Unreachable
	}
	if grid.IsWall(start) || (blocked != nil && blocked.Has(start)) {
		return 0, dist
	}
	dist[start.Y*W+start.X] = 0
	q := state.DistQ[:0]
	q = append(q, start)
	count := 0
	for i := 0; i < len(q); i++ {
		p := q[i]
		count++
		d := dist[p.Y*W+p.X]
		for dir := DirUp; dir <= DirLeft; dir++ {
			np := Add(p, DirDelta[dir])
			if grid.IsWall(np) {
				continue
			}
			ni := np.Y*W + np.X
			if dist[ni] != Unreachable || (blocked != nil && blocked.Has(np)) {
				continue
			}
			dist[ni] = d + 1
			q = append(q, np)
		}
	}
	state.DistQ = q
	return count, dist
}

func cmdFlood(body []Point, facing Direction, occupied *BitGrid) (int, []int) {
	n := W * H
	dist := make([]int, n)
	for i := range dist {
		dist[i] = Unreachable
	}
	head := body[0]
	if grid.IsWall(head) || (occupied != nil && occupied.Has(head)) {
		return 0, dist
	}
	dist[head.Y*W+head.X] = 0
	type landing struct {
		pos  Point
		dist int
	}
	var landings []landing
	landings = append(landings, landing{pos: head, dist: 0})
	for _, dir := range state.VMoves(head, facing) {
		nb, _, alive, _, _ := simMove(body, facing, dir, nil, occupied)
		if !alive {
			continue
		}
		nh := nb[0]
		ni := nh.Y*W + nh.X
		if ni >= 0 && ni < n && dist[ni] == Unreachable {
			dist[ni] = 1
			landings = append(landings, landing{pos: nh, dist: 1})
		}
	}
	q := state.DistQ[:0]
	for _, l := range landings {
		q = append(q, l.pos)
	}
	count := 0
	for i := 0; i < len(q); i++ {
		p := q[i]
		count++
		d := dist[p.Y*W+p.X]
		for dir := DirUp; dir <= DirLeft; dir++ {
			np := Add(p, DirDelta[dir])
			if grid.IsWall(np) {
				continue
			}
			ni := np.Y*W + np.X
			if dist[ni] != Unreachable || (occupied != nil && occupied.Has(np)) {
				continue
			}
			dist[ni] = d + 1
			q = append(q, np)
		}
	}
	state.DistQ = q
	return count, dist
}

func validDirs(facing Direction) ([4]Direction, int) {
	if facing == DirNone {
		return [4]Direction{DirUp, DirRight, DirDown, DirLeft}, 4
	}
	back := Opp(facing)
	var out [4]Direction
	n := 0
	for d := DirUp; d <= DirLeft; d++ {
		if d != back {
			out[n] = d
			n++
		}
	}
	return out, n
}

func srcScore(head, target Point) int {
	d := MDist(head, target)
	if target.Y < head.Y {
		d += head.Y - target.Y
	}
	if grid.WBelow(target) {
		d -= 3
	}
	return d
}

type DirInfo struct {
	flood int
	dists []int
	body  []Point
	alive bool
}

func calcDirInfo(body []Point, facing Direction, occupied *BitGrid) map[Direction]*DirInfo {
	head := body[0]
	info := make(map[Direction]*DirInfo, 3)
	for _, dir := range state.VMoves(head, facing) {
		nb, _, alive, _, _ := simMove(body, facing, dir, nil, occupied)
		di := &DirInfo{alive: alive}
		if alive {
			di.body = make([]Point, len(nb))
			copy(di.body, nb)
			blocked := NewBG(W, H)
			copy(blocked.Bits, occupied.Bits)
			for _, p := range di.body[1:] {
				blocked.Set(p)
			}
			di.flood, di.dists = floodDist(di.body[0], &blocked)
		}
		info[dir] = di
	}
	return info
}

func isSafeDir(dir Direction, dirInfo map[Direction]*DirInfo, bodyLen int) bool {
	di, ok := dirInfo[dir]
	if !ok || !di.alive {
		return false
	}
	thresh := bodyLen * 2
	if thresh < 4 {
		thresh = 4
	}
	return di.flood >= thresh
}

func bestSafeDir(dirInfo map[Direction]*DirInfo) (Direction, bool) {
	best := DirNone
	bestFlood := -1
	for dir, di := range dirInfo {
		if di.alive && di.flood > bestFlood {
			bestFlood = di.flood
			best = dir
		}
	}
	return best, best != DirNone
}

type SearchResult struct {
	dir    Direction
	target Point
	steps  int
	score  int
	ok     bool
}

type botPlan struct {
	id     int
	body   []Point
	facing Direction
	dir    Direction
	target Point
	reason string
	ok     bool
}

type botEntry struct {
	id   int
	body []Point
}

type supportJob struct {
	climberID int
	apple     Point
	cell      Point
	score     int
}

func stateHash(facing Direction, body []Point) uint64 {
	h := uint64(14695981039346656037)
	h ^= uint64(facing)
	h *= 1099511628211
	for _, p := range body {
		h ^= uint64(p.X)
		h *= 1099511628211
		h ^= uint64(p.Y)
		h *= 1099511628211
	}
	return h
}

func bodyFacing(body []Point) Direction {
	if len(body) < 2 {
		return DirUp
	}
	return FacingPts(body[0], body[1])
}

func actionString(id int, dir Direction, reason string) string {
	if debug && reason != "" {
		return fmt.Sprintf("%d %s %s", id, DirName[dir], reason)
	}
	return fmt.Sprintf("%d %s", id, DirName[dir])
}

type refBird struct {
	owner  int
	body   fBody
	facing Direction
	alive  bool
}

type oneTurnOutcome struct {
	losses  [2]int
	deaths  [2]int
	trapped [2]int
}

type refScratch struct {
	birds     [MaxBirds]refBird
	apples    BitGrid
	occ       BitGrid
	otherOc   BitGrid
	toBehead  [MaxBirds]bool
	airborne  [MaxBirds]bool
	grounded  [MaxBirds]bool
	newlyGrnd [MaxBirds]int
	eaten     [MaxBirds]Point
	nextBuf   [MaxBody + 1]Point
	enemyDirs [MaxBirds]Direction
	ourDirs   [4]Direction
	candidate [4]Direction
}

func newRefScratch(w, h int) refScratch {
	return refScratch{
		apples:  NewBG(w, h),
		occ:     NewBG(w, h),
		otherOc: NewBG(w, h),
	}
}

func isGroundedRef(c Point, grounded []bool, birds []refBird, apples *BitGrid) bool {
	below := Point{c.X, c.Y + 1}
	if grid.WBelow(c) {
		return true
	}
	if apples.Has(below) {
		return true
	}
	for i, ok := range grounded {
		if ok && birds[i].alive && birds[i].body.contains(below) {
			return true
		}
	}
	return false
}

func simulateOneTurn(sc *refScratch, mine []botEntry, enemies []enemyInfo, ourDirs, enemyDirs []Direction, sources []Point) oneTurnOutcome {
	nMine := len(mine)
	nEnemy := len(enemies)
	nBirds := nMine + nEnemy

	// --- init birds ---
	for i, bot := range mine {
		b := &sc.birds[i]
		b.owner = 0
		b.body.set(bot.body)
		b.facing = b.body.facing()
		b.alive = true
	}
	for i, enemy := range enemies {
		b := &sc.birds[nMine+i]
		b.owner = 1
		b.body.set(enemy.body)
		b.facing = enemy.facing
		b.alive = true
	}

	// --- apples ---
	fillBG(&sc.apples, sources)

	// --- move ---
	for i := 0; i < nBirds; i++ {
		b := &sc.birds[i]
		if !b.alive || b.body.len == 0 {
			continue
		}
		var dir Direction
		if i < nMine {
			dir = ourDirs[i]
		} else {
			dir = enemyDirs[i-nMine]
		}
		if dir == DirNone {
			dir = b.facing
		}
		if dir == DirNone {
			continue
		}

		head := b.body.parts[0]
		newHead := Add(head, DirDelta[dir])
		willEat := sc.apples.Has(newHead)

		n := 0
		sc.nextBuf[n] = newHead
		n++
		if willEat {
			copy(sc.nextBuf[n:], b.body.parts[:b.body.len])
			n += b.body.len
		} else {
			copy(sc.nextBuf[n:], b.body.parts[:b.body.len-1])
			n += b.body.len - 1
		}
		b.body.len = n
		copy(b.body.parts[:n], sc.nextBuf[:n])
	}

	// --- eat apples ---
	nEaten := 0
	for i := 0; i < nBirds; i++ {
		b := &sc.birds[i]
		if b.alive && b.body.len > 0 && sc.apples.Has(b.body.parts[0]) {
			sc.eaten[nEaten] = b.body.parts[0]
			nEaten++
		}
	}
	for k := 0; k < nEaten; k++ {
		sc.apples.Clear(sc.eaten[k])
	}

	// --- beheadings ---
	var outcome oneTurnOutcome
	for i := 0; i < nBirds; i++ {
		sc.toBehead[i] = false
	}
	for i := 0; i < nBirds; i++ {
		bi := &sc.birds[i]
		if !bi.alive || bi.body.len == 0 {
			continue
		}
		head := bi.body.parts[0]
		if grid.IsWall(head) {
			sc.toBehead[i] = true
			continue
		}
		for j := 0; j < nBirds; j++ {
			bj := &sc.birds[j]
			if !bj.alive || bj.body.len == 0 {
				continue
			}
			if !bj.body.contains(head) {
				continue
			}
			if i != j {
				sc.toBehead[i] = true
				break
			}
			for k := 1; k < bj.body.len; k++ {
				if bj.body.parts[k] == head {
					sc.toBehead[i] = true
					break
				}
			}
			if sc.toBehead[i] {
				break
			}
		}
	}
	for i := 0; i < nBirds; i++ {
		if !sc.toBehead[i] {
			continue
		}
		b := &sc.birds[i]
		if b.body.len <= 3 {
			outcome.losses[b.owner] += b.body.len
			outcome.deaths[b.owner]++
			b.alive = false
			continue
		}
		outcome.losses[b.owner]++
		copy(b.body.parts[:b.body.len-1], b.body.parts[1:b.body.len])
		b.body.len--
	}

	// --- gravity ---
	for i := 0; i < nBirds; i++ {
		sc.airborne[i] = sc.birds[i].alive
		sc.grounded[i] = false
	}
	somethingFell := true
	for somethingFell {
		somethingFell = false
		somethingGotGrounded := true
		for somethingGotGrounded {
			somethingGotGrounded = false
			nNewlyGrnd := 0
			for i := 0; i < nBirds; i++ {
				if !sc.airborne[i] {
					continue
				}
				bi := &sc.birds[i]
				isGrnd := false
				for k := 0; k < bi.body.len; k++ {
					if isGroundedRef(bi.body.parts[k], sc.grounded[:nBirds], sc.birds[:nBirds], &sc.apples) {
						isGrnd = true
						break
					}
				}
				if isGrnd {
					sc.newlyGrnd[nNewlyGrnd] = i
					nNewlyGrnd++
				}
			}
			if nNewlyGrnd > 0 {
				somethingGotGrounded = true
				for k := 0; k < nNewlyGrnd; k++ {
					idx := sc.newlyGrnd[k]
					sc.grounded[idx] = true
					sc.airborne[idx] = false
				}
			}
		}

		for i := 0; i < nBirds; i++ {
			if !sc.airborne[i] {
				continue
			}
			somethingFell = true
			bi := &sc.birds[i]
			for j := 0; j < bi.body.len; j++ {
				bi.body.parts[j].Y++
			}
			allOut := true
			for j := 0; j < bi.body.len; j++ {
				if bi.body.parts[j].Y < H+1 {
					allOut = false
					break
				}
			}
			if allOut {
				outcome.deaths[bi.owner]++
				bi.alive = false
				sc.airborne[i] = false
			}
		}
	}

	// --- update facing ---
	for i := 0; i < nBirds; i++ {
		if sc.birds[i].alive {
			sc.birds[i].facing = sc.birds[i].body.facing()
		}
	}

	// --- trapped check (our bots only) ---
	sc.occ.Reset()
	for i := 0; i < nBirds; i++ {
		if !sc.birds[i].alive {
			continue
		}
		bi := &sc.birds[i]
		for k := 0; k < bi.body.len; k++ {
			sc.occ.Set(bi.body.parts[k])
		}
	}
	for i := 0; i < nMine; i++ {
		bi := &sc.birds[i]
		if !bi.alive || bi.body.len == 0 {
			continue
		}
		copy(sc.otherOc.Bits, sc.occ.Bits)
		for k := 0; k < bi.body.len; k++ {
			sc.otherOc.Clear(bi.body.parts[k])
		}
		hasEscape := false
		body := bi.body.slice()
		for _, dir := range state.VMoves(body[0], bi.facing) {
			_, _, alive, _, _ := simMove(body, bi.facing, dir, &sc.apples, &sc.otherOc)
			if alive {
				hasEscape = true
				break
			}
		}
		if !hasEscape {
			outcome.trapped[0]++
		}
	}

	return outcome
}

func outcomeRisk(outcome oneTurnOutcome) int {
	return outcome.deaths[0]*100000 + outcome.trapped[0]*5000 + outcome.losses[0]*100 - outcome.deaths[1]*20 - outcome.losses[1]
}

func worstCasePlanRisk(sc *refScratch, mine []botEntry, enemies []enemyInfo, sources []Point, ourDirs []Direction) int {
	if len(enemies) == 0 {
		return outcomeRisk(simulateOneTurn(sc, mine, nil, ourDirs, nil, sources))
	}

	worst := -1
	var walk func(idx int)
	walk = func(idx int) {
		if idx == len(enemies) {
			risk := outcomeRisk(simulateOneTurn(sc, mine, enemies, ourDirs, sc.enemyDirs[:len(enemies)], sources))
			if risk > worst {
				worst = risk
			}
			return
		}
		dirs, nd := validDirs(enemies[idx].facing)
		for di := 0; di < nd; di++ {
			sc.enemyDirs[idx] = dirs[di]
			walk(idx + 1)
		}
	}
	walk(0)
	return worst
}

func refinePlansWithOneTurnSafety(sc *refScratch, mine []botEntry, enemies []enemyInfo, sources []Point, plans []botPlan, deadline time.Time) {
	if len(mine) == 0 || len(enemies) == 0 || time.Until(deadline) < 8*time.Millisecond {
		return
	}

	combos := 1
	for _, enemy := range enemies {
		_, nd := validDirs(enemy.facing)
		combos *= nd
		if combos > 128 {
			return
		}
	}

	nPlans := len(plans)
	for i := 0; i < nPlans; i++ {
		sc.ourDirs[i] = plans[i].dir
	}

	bestRisk := worstCasePlanRisk(sc, mine, enemies, sources, sc.ourDirs[:nPlans])
	for i := 0; i < nPlans; i++ {
		if time.Until(deadline) < 4*time.Millisecond {
			break
		}
		currentDir := sc.ourDirs[i]
		dirs, nd := validDirs(plans[i].facing)
		for _, dir := range dirs[:nd] {
			if dir == currentDir {
				continue
			}
			sc.candidate = sc.ourDirs
			sc.candidate[i] = dir
			risk := worstCasePlanRisk(sc, mine, enemies, sources, sc.candidate[:nPlans])
			if risk < bestRisk {
				bestRisk = risk
				sc.ourDirs = sc.candidate
				plans[i].dir = dir
				plans[i].target = Add(plans[i].body[0], DirDelta[dir])
				plans[i].reason = "safety"
				plans[i].ok = true
			}
		}
	}
}

func instantEat(body []Point, facing Direction, sources []Point, srcBG, occupied *BitGrid) SearchResult {
	head := body[0]
	var best SearchResult
	for _, dir := range state.VMoves(head, facing) {
		target := Add(head, DirDelta[dir])
		if !srcBG.Has(target) {
			continue
		}
		nb, nf, alive, ate, eatenAt := simMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}
		if ate && !hasFollowupEscape(nb, nf, srcBG, occupied, eatenAt) {
			continue
		}
		score := srcScore(head, target)
		if !best.ok || score < best.score {
			best = SearchResult{dir: dir, target: target, steps: 1, score: score, ok: true}
		}
	}
	return best
}

func hasFollowupEscape(body []Point, facing Direction, sources, occupied *BitGrid, eatenAt Point) bool {
	nextSources := sources
	if sources != nil && sources.Has(eatenAt) {
		cloned := NewBG(sources.Width, sources.Height)
		copy(cloned.Bits, sources.Bits)
		cloned.Clear(eatenAt)
		nextSources = &cloned
	}

	head := body[0]
	for _, dir := range state.VMoves(head, facing) {
		_, _, alive, _, _ := simMove(body, facing, dir, nextSources, occupied)
		if alive {
			return true
		}
	}
	return false
}

func cmdBFS(body []Point, facing Direction, sources []Point,
	maxDepth int, dirInfo map[Direction]*DirInfo, enemyDists []int,
	srcBG, occupied *BitGrid, deadline time.Time) SearchResult {

	if len(sources) == 0 {
		return SearchResult{}
	}

	appleIdx := make(map[Point]int, len(sources))
	for i, s := range sources {
		if i >= 64 {
			break
		}
		appleIdx[s] = i
	}

	type qItem struct {
		body     []Point
		face     Direction
		first    Direction
		depth    int
		eatenSet uint64
	}

	startBody := make([]Point, len(body))
	copy(startBody, body)
	queue := []qItem{{body: startBody, face: facing, first: DirNone}}
	seen := map[uint64]bool{stateHash(facing, body): true}
	best := SearchResult{}
	iters := 0
	bodyLen := len(body)

	for qi := 0; qi < len(queue); qi++ {
		item := queue[qi]
		if item.depth >= maxDepth {
			continue
		}
		iters++
		if iters&255 == 0 && time.Now().After(deadline) {
			break
		}

		head := item.body[0]

		var restored []Point
		if item.eatenSet != 0 {
			for s, idx := range appleIdx {
				if item.eatenSet&(1<<uint(idx)) != 0 {
					if srcBG.Has(s) {
						srcBG.Clear(s)
						restored = append(restored, s)
					}
				}
			}
		}

		for _, dir := range state.VMoves(head, item.face) {
			nb, nf, alive, ate, eatenAt := simMove(item.body, item.face, dir, srcBG, occupied)
			if !alive {
				continue
			}
			first := item.first
			if first == DirNone {
				first = dir
			}

			newEaten := item.eatenSet
			if ate && srcBG.Has(eatenAt) {
				if !hasFollowupEscape(nb, nf, srcBG, occupied, eatenAt) {
					continue
				}
				if idx, ok := appleIdx[eatenAt]; ok {
					newEaten |= 1 << uint(idx)
				}

				rawSteps := item.depth + 1
				score := rawSteps * 1000
				score += srcScore(body[0], eatenAt)
				if di, ok := dirInfo[first]; ok && di.alive && rawSteps == 1 {
					if di.flood < bodyLen*2 {
						score += 3000
					} else if di.flood < bodyLen*3 {
						score += 1000
					}
				}
				ei := eatenAt.Y*W + eatenAt.X
				if enemyDists[ei] != Unreachable {
					ed := enemyDists[ei]
					if rawSteps <= ed {
						score -= 300
					} else if rawSteps <= ed+2 {
						score += 500
					} else {
						score += 2000
					}
				}
				if minLen := state.Terr.MinBodyLen(body, eatenAt); minLen <= bodyLen {
					surplus := bodyLen - minLen
					if surplus > 4 {
						surplus = 4
					}
					score -= 50 + surplus*50
				}
				for _, s := range sources {
					if s != eatenAt && MDist(eatenAt, s) <= 4 {
						score -= 100
					}
				}
				cand := SearchResult{dir: first, target: eatenAt, steps: rawSteps, score: score, ok: true}
				if !best.ok || cand.score < best.score {
					best = cand
				}
				h := stateHash(nf, nb)
				if !seen[h] {
					seen[h] = true
					cp := make([]Point, len(nb))
					copy(cp, nb)
					queue = append(queue, qItem{body: cp, face: nf, first: first, depth: item.depth + 1, eatenSet: newEaten})
				}
				continue
			}
			if best.ok && item.depth+1 >= best.steps {
				continue
			}
			h := stateHash(nf, nb)
			if seen[h] {
				continue
			}
			seen[h] = true
			cp := make([]Point, len(nb))
			copy(cp, nb)
			queue = append(queue, qItem{body: cp, face: nf, first: first, depth: item.depth + 1, eatenSet: newEaten})
		}

		for _, s := range restored {
			srcBG.Set(s)
		}
	}
	return best
}

type enemyInfo struct {
	head    Point
	facing  Direction
	bodyLen int
	body    []Point
}

func bestAction(body []Point, facing Direction, sources []Point,
	dirInfo map[Direction]*DirInfo, enemies []enemyInfo, enemyDists []int,
	srcBG, occupied, danger *BitGrid) SearchResult {

	if len(sources) == 0 {
		return SearchResult{dir: DirUp, ok: true}
	}
	head := body[0]
	bodyLen := len(body)

	initRun := state.Terr.BodyInitRun(body)
	reachable := state.Terr.SupReachMulti(head, initRun, bodyLen, sources, srcBG)

	var best SearchResult
	vd1, nd1 := validDirs(facing)
	for _, dir := range vd1[:nd1] {
		nb, _, alive, ate, eatenAt := simMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}

		di := dirInfo[dir]
		bestTarget := sources[0]
		bestDist := Unreachable

		if len(reachable) > 0 {
			useBFS := di != nil && di.alive && di.dists != nil
			for _, c := range reachable {
				var d int
				if useBFS {
					d = di.dists[c.Y*W+c.X]
				} else {
					d = srcScore(nb[0], c)
				}
				if d < bestDist {
					bestDist = d
					bestTarget = c
				}
			}
		} else {
			useBFS := di != nil && di.alive && di.dists != nil
			for _, s := range sources {
				var d int
				if useBFS {
					d = di.dists[s.Y*W+s.X]
				} else {
					d = srcScore(nb[0], s)
				}
				if d < bestDist {
					bestDist = d
					bestTarget = s
				}
			}
		}

		score := bestDist
		if bodyLen >= 8 && bestDist < Unreachable {
			score = bestDist / 2
		}
		if ate && srcBG.Has(eatenAt) {
			score = -1000
			bestTarget = eatenAt
		}

		expectedLen := bodyLen
		if ate {
			expectedLen++
		}
		if len(nb) < expectedLen {
			if bodyLen <= 5 {
				score += 1000
			} else {
				score += 300
			}
		}

		if danger.Has(nb[0]) {
			dangerPen := 20
			if bodyLen <= 3 {
				dangerPen = 500
			} else if bodyLen <= 5 {
				dangerPen = 100
			}
			for _, e := range enemies {
				canReach := false
				evd, end := validDirs(e.facing)
				for _, edir := range evd[:end] {
					if Add(e.head, DirDelta[edir]) == nb[0] {
						canReach = true
						break
					}
				}
				if canReach && e.bodyLen <= 3 && bodyLen > 3 {
					dangerPen = -500
				}
			}
			score += dangerPen
		}

		if nb[0] == head {
			score += 200
		}
		for d := DirUp; d <= DirLeft; d++ {
			if grid.IsWall(Add(nb[0], DirDelta[d])) {
				score--
			}
		}

		if di != nil && di.alive {
			if di.flood < bodyLen {
				score += 2000
			} else if di.flood < bodyLen*2 {
				score += 500
			}
			if bodyLen >= 8 {
				spaceBonus := di.flood * 10 / bodyLen
				if spaceBonus > 200 {
					spaceBonus = 200
				}
				score -= spaceBonus
			}
		} else if di == nil {
			score += 1500
		}

		if grid.WBelow(nb[0]) {
			score -= 3
		}

		ti := bestTarget.Y*W + bestTarget.X
		if enemyDists[ti] != Unreachable {
			if bestDist < Unreachable && enemyDists[ti] < bestDist-3 {
				score += 50
			}
		}

		cand := SearchResult{dir: dir, target: bestTarget, score: score, ok: true}
		if !best.ok || cand.score < best.score {
			best = cand
		}
	}

	if best.ok {
		return best
	}
	return SearchResult{dir: facing, ok: true}
}

func bestGroundAction(body []Point, facing Direction, target Point,
	dirInfo map[Direction]*DirInfo, enemies []enemyInfo,
	srcBG, occupied, danger *BitGrid) SearchResult {

	bodyLen := len(body)
	var best SearchResult
	vd2, nd2 := validDirs(facing)
	for _, dir := range vd2[:nd2] {
		nb, _, alive, ate, eatenAt := simMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}

		di := dirInfo[dir]
		score := MDist(nb[0], target) * 12
		if nb[0].X == target.X {
			score -= 12
		}
		if nb[0].Y > target.Y {
			score -= 6
		}
		if nb[0] == target {
			score -= 120
		}
		if ate && srcBG.Has(eatenAt) {
			score -= 60
		}

		below := Point{nb[0].X, nb[0].Y + 1}
		if grid.WBelow(nb[0]) || (srcBG != nil && srcBG.Has(below)) {
			score -= 10
		}

		if danger.Has(nb[0]) {
			dangerPen := 40
			if bodyLen <= 3 {
				dangerPen = 600
			} else if bodyLen <= 5 {
				dangerPen = 150
			}
			for _, e := range enemies {
				canReach := false
				evd2, end2 := validDirs(e.facing)
				for _, edir := range evd2[:end2] {
					if Add(e.head, DirDelta[edir]) == nb[0] {
						canReach = true
						break
					}
				}
				if canReach && e.bodyLen <= 3 && bodyLen > 3 {
					dangerPen = -400
				}
			}
			score += dangerPen
		}

		if di != nil && di.alive {
			if di.flood < bodyLen {
				score += 2500
			} else if di.flood < bodyLen*2 {
				score += 700
			}
		} else {
			score += 2000
		}

		cand := SearchResult{dir: dir, target: target, score: score, ok: true}
		if !best.ok || cand.score < best.score {
			best = cand
		}
	}

	if best.ok {
		return best
	}
	return SearchResult{dir: facing, target: target, ok: true}
}

func calcEnemyDist(enemies []enemyInfo, allOcc *BitGrid) []int {
	n := W * H
	result := make([]int, n)
	for i := range result {
		result[i] = Unreachable
	}
	for _, e := range enemies {
		blocked := occExcept(allOcc, e.body)
		_, eDists := cmdFlood(e.body, e.facing, &blocked)
		for i, d := range eDists {
			if d < result[i] {
				result[i] = d
			}
		}
	}
	return result
}

func predictEnemyWalls(enemies []enemyInfo, sources []Point, allOcc *BitGrid) BitGrid {
	walls := NewBG(W, H)
	for _, e := range enemies {
		for _, p := range e.body {
			walls.Set(p)
		}
		bestDir := e.facing
		bestDist := Unreachable
		dirs, nd := validDirs(e.facing)
		for _, dir := range dirs[:nd] {
			nh := Add(e.head, DirDelta[dir])
			if grid.IsWall(nh) || allOcc.Has(nh) {
				continue
			}
			for _, s := range sources {
				if d := MDist(nh, s); d < bestDist {
					bestDist = d
					bestDir = dir
				}
			}
		}
		predicted := Add(e.head, DirDelta[bestDir])
		if !grid.IsWall(predicted) {
			walls.Set(predicted)
		}
	}
	return walls
}

func filtSrc(sources []Point, myDists, enemyDists []int) []Point {
	out := make([]Point, 0, len(sources))
	for _, s := range sources {
		si := s.Y*W + s.X
		md, ed := myDists[si], enemyDists[si]
		if md != Unreachable && ed != Unreachable && ed < md-3 {
			continue
		}
		out = append(out, s)
	}
	if len(out) == 0 {
		return sources
	}
	return out
}

func limitedSupportTargets(targets []Point) []Point {
	if len(targets) <= 4 {
		return targets
	}
	cp := append([]Point(nil), targets...)
	sort.Slice(cp, func(i, j int) bool {
		if cp[i].Y != cp[j].Y {
			return cp[i].Y > cp[j].Y
		}
		return cp[i].X < cp[j].X
	})
	return cp[:4]
}

func planSupportJobs(mine []botEntry, preferred [][]Point, sources []Point, botDists [][]int, deadline time.Time) map[int]supportJob {
	if len(mine) < 2 || len(sources) == 0 || time.Until(deadline) < 18*time.Millisecond {
		return nil
	}

	srcBG := NewBG(W, H)
	fillBG(&srcBG, sources)

	hasReachable := make([]bool, len(mine))
	for i, bot := range mine {
		bodyLen := len(bot.body)
		initRun := state.Terr.BodyInitRun(bot.body)
		targets := limitedSupportTargets(preferred[i])
		if len(targets) == 0 {
			targets = limitedSupportTargets(sources)
		}
		if len(state.Terr.SupReachMulti(bot.body[0], initRun, bodyLen, targets, &srcBG)) > 0 {
			hasReachable[i] = true
		}
	}

	type supportCand struct {
		supporter int
		climber   int
		apple     Point
		cell      Point
		score     int
	}
	cands := make([]supportCand, 0, len(mine)*len(sources))

	for supporter := range mine {
		if hasReachable[supporter] {
			continue
		}
		if time.Until(deadline) < 8*time.Millisecond {
			break
		}
		supporterLen := len(mine[supporter].body)
		for climber := range mine {
			if climber == supporter || len(mine[climber].body) <= supporterLen {
				continue
			}
			if time.Until(deadline) < 8*time.Millisecond {
				break
			}

			climberLen := len(mine[climber].body)
			targets := limitedSupportTargets(preferred[climber])
			if len(targets) == 0 {
				targets = limitedSupportTargets(sources)
			}
			bestScore := Unreachable
			var bestApple Point
			var bestCell Point

			for _, apple := range targets {
				if time.Until(deadline) < 8*time.Millisecond {
					break
				}
				minLen := state.Terr.SupPathBFS(mine[climber].body[0], state.Terr.BodyInitRun(mine[climber].body), apple, &srcBG)
				if minLen <= climberLen {
					continue
				}

				maxY := apple.Y + 6
				if maxY >= H {
					maxY = H - 1
				}
				for dx := -1; dx <= 1; dx++ {
					sx := apple.X + dx
					if sx < 0 || sx >= W {
						continue
					}
					for y := apple.Y + 1; y <= maxY; y++ {
						cell := Point{sx, y}
						if grid.IsWall(cell) {
							break
						}
						ci := cell.Y*W + cell.X
						if botDists[supporter][ci] == Unreachable {
							continue
						}
						minLen, climbDist := state.Terr.MinImmLen(cell, apple, &srcBG)
						if minLen == Unreachable || minLen > climberLen {
							continue
						}

						score := botDists[supporter][ci] * 20
						score += climbDist * 8
						score += MDist(mine[climber].body[0], cell) * 6
						score -= apple.Y * 25
						score += abs(dx) * 10
						if grid.WBelow(cell) {
							score -= 15
						}
						if score < bestScore {
							bestScore = score
							bestApple = apple
							bestCell = cell
						}
					}
				}
			}

			if bestScore != Unreachable {
				cands = append(cands, supportCand{
					supporter: supporter,
					climber:   climber,
					apple:     bestApple,
					cell:      bestCell,
					score:     bestScore,
				})
			}
		}
	}

	if len(cands) == 0 {
		return nil
	}

	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score < cands[j].score
		}
		if cands[i].apple.Y != cands[j].apple.Y {
			return cands[i].apple.Y > cands[j].apple.Y
		}
		return mine[cands[i].supporter].id < mine[cands[j].supporter].id
	})

	usedSupporter := make([]bool, len(mine))
	usedClimber := make([]bool, len(mine))
	jobs := make(map[int]supportJob, len(mine))
	for _, cand := range cands {
		if usedSupporter[cand.supporter] || usedClimber[cand.climber] {
			continue
		}
		usedSupporter[cand.supporter] = true
		usedClimber[cand.climber] = true
		jobs[mine[cand.supporter].id] = supportJob{
			climberID: mine[cand.climber].id,
			apple:     cand.apple,
			cell:      cand.cell,
			score:     cand.score,
		}
	}

	if len(jobs) == 0 {
		return nil
	}
	return jobs
}

// isHeadLockedWorstCase checks whether, after our bot moves in dir,
// there exists ANY combination of enemy moves that leaves our bot
// with zero valid follow-up moves (head locked).
func isHeadLockedWorstCase(body []Point, facing, dir Direction, enemies []enemyInfo, otherOcc, srcBG *BitGrid) bool {
	// Only check if an enemy head is within range 2 of our head
	head := body[0]
	newHead := Add(head, DirDelta[dir])
	nearbyEnemies := make([]enemyInfo, 0, len(enemies))
	for _, e := range enemies {
		if MDist(newHead, e.head) <= 3 {
			nearbyEnemies = append(nearbyEnemies, e)
		}
	}
	if len(nearbyEnemies) == 0 {
		return false
	}

	// Simulate our move
	nb, nf, alive, _, _ := simMove(body, facing, dir, srcBG, otherOcc)
	if !alive {
		return true
	}

	// Build occupied grid: otherOcc + our new body
	baseOcc := NewBG(W, H)
	copy(baseOcc.Bits, otherOcc.Bits)
	for _, p := range nb[1:] {
		baseOcc.Set(p)
	}

	// Cap combos on nearby enemies only
	combos := 1
	for _, e := range nearbyEnemies {
		_, nd := validDirs(e.facing)
		combos *= nd
		if combos > 27 {
			return false
		}
	}

	finalHead := nb[0]
	eDirs := make([]Direction, len(nearbyEnemies))
	testOcc := NewBG(W, H) // pre-allocate once
	locked := false

	var walk func(idx int)
	walk = func(idx int) {
		if locked {
			return
		}
		if idx == len(nearbyEnemies) {
			copy(testOcc.Bits, baseOcc.Bits)
			for ei, e := range nearbyEnemies {
				nh := Add(e.head, DirDelta[eDirs[ei]])
				if !grid.IsWall(nh) {
					testOcc.Set(nh)
				}
			}
			for _, d := range state.VMoves(finalHead, nf) {
				nh := Add(finalHead, DirDelta[d])
				if !grid.IsWall(nh) && !testOcc.Has(nh) {
					return
				}
			}
			locked = true
			return
		}
		dirs, nd := validDirs(nearbyEnemies[idx].facing)
		for di := 0; di < nd; di++ {
			eDirs[idx] = dirs[di]
			walk(idx + 1)
		}
	}
	walk(0)
	return locked
}

func main() {
	scanner = bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	readline()
	fmt.Sscan(readline(), &W)
	fmt.Sscan(readline(), &H)

	walls := make(map[Point]bool)
	for y := 0; y < H; y++ {
		row := readline()
		for x, ch := range row {
			if ch == '#' {
				walls[Point{X: x, Y: y}] = true
			}
		}
	}
	grid = NewAG(W, H, walls)
	state = NewState(grid)
	rsc = newRefScratch(W, H)

	var botsPerPlayer int
	fmt.Sscan(readline(), &botsPerPlayer)
	myBots := make(map[int]bool)
	for i := 0; i < botsPerPlayer; i++ {
		var id int
		fmt.Sscan(readline(), &id)
		myBots[id] = true
	}
	for i := 0; i < botsPerPlayer; i++ {
		readline()
	}

	turn := 0
	for {
		var srcN int
		fmt.Sscan(readline(), &srcN)
		sources := make([]Point, srcN)
		for i := range sources {
			fmt.Sscan(readline(), &sources[i].X, &sources[i].Y)
		}

		var botN int
		fmt.Sscan(readline(), &botN)

		allOcc := NewBG(W, H)
		var mine []botEntry
		var enemies []enemyInfo

		for i := 0; i < botN; i++ {
			var id int
			var body string
			fmt.Sscan(readline(), &id, &body)
			pts := parseBody(body)
			for _, p := range pts {
				allOcc.Set(p)
			}
			f := DirUp
			if len(pts) >= 2 {
				f = FacingPts(pts[0], pts[1])
			}
			if myBots[id] {
				mine = append(mine, botEntry{id: id, body: pts})
			} else {
				enemies = append(enemies, enemyInfo{head: pts[0], facing: f, bodyLen: len(pts), body: pts})
			}
		}

		budget := 45 * time.Millisecond
		if turn == 0 {
			budget = 900 * time.Millisecond
		}
		turnDeadline := time.Now().Add(budget)

		enemyWalls := predictEnemyWalls(enemies, sources, &allOcc)
		eDanger := NewBG(W, H)
		for _, e := range enemies {
			ed, edn := validDirs(e.facing)
			for _, d := range ed[:edn] {
				eDanger.Set(Add(e.head, DirDelta[d]))
			}
		}

		enemyDists := calcEnemyDist(enemies, &allOcc)

		type sortEntry struct {
			idx     int
			minDist int
		}
		sortKeys := make([]sortEntry, len(mine))
		tmpDists := make([][]int, len(mine))
		for i, bot := range mine {
			occ := occExcept(&allOcc, bot.body)
			for j := range occ.Bits {
				occ.Bits[j] |= enemyWalls.Bits[j]
			}
			f := bodyFacing(bot.body)
			_, tmpDists[i] = cmdFlood(bot.body, f, &occ)
			md := Unreachable
			for _, s := range sources {
				if d := tmpDists[i][s.Y*W+s.X]; d < md {
					md = d
				}
			}
			sortKeys[i] = sortEntry{idx: i, minDist: md}
		}
		sort.Slice(sortKeys, func(i, j int) bool {
			if sortKeys[i].minDist != sortKeys[j].minDist {
				return sortKeys[i].minDist < sortKeys[j].minDist
			}
			return mine[sortKeys[i].idx].id < mine[sortKeys[j].idx].id
		})
		sortedMine := make([]botEntry, len(mine))
		botDists := make([][]int, len(mine))
		for i, sk := range sortKeys {
			sortedMine[i] = mine[sk.idx]
			botDists[i] = tmpDists[sk.idx]
		}
		mine = sortedMine

		vsrc := make([][]Point, len(mine))
		for _, s := range sources {
			si := s.Y*W + s.X
			bestBot := -1
			bestDist := Unreachable
			for i := range mine {
				d := botDists[i][si]
				if d < bestDist {
					bestDist = d
					bestBot = i
				}
			}
			if bestBot >= 0 {
				vsrc[bestBot] = append(vsrc[bestBot], s)
			}
		}
		// Sort each bot's apples: prioritize by race margin minus distance penalty.
		// margin = enemy_dist - our_dist (positive = we're closer)
		// Penalty -dist/4 discourages chasing far uncontested apples over near ones.
		for i := range vsrc {
			bd := botDists[i]
			sort.Slice(vsrc[i], func(a, b int) bool {
				sa := vsrc[i][a].Y*W + vsrc[i][a].X
				sb := vsrc[i][b].Y*W + vsrc[i][b].X
				ma := (enemyDists[sa] - bd[sa]) - bd[sa]/4
				mb := (enemyDists[sb] - bd[sb]) - bd[sb]/4
				return ma > mb
			})
		}
		supportJobs := planSupportJobs(mine, vsrc, sources, botDists, turnDeadline)

		plans := make([]botPlan, 0, len(mine))
		plannedHeads := NewBG(W, H)

		for botIdx, bot := range mine {
			body := bot.body
			head := body[0]
			facing := bodyFacing(body)
			bodyLen := len(body)

			otherOcc := occExcept(&allOcc, body)
			for i := range otherOcc.Bits {
				otherOcc.Bits[i] |= plannedHeads.Bits[i]
				otherOcc.Bits[i] |= enemyWalls.Bits[i]
			}

			dirInfo := calcDirInfo(body, facing, &otherOcc)

			_, myDists := cmdFlood(body, facing, &otherOcc)

			srcBG := NewBG(W, H)
			fillBG(&srcBG, sources)
			allCompetitive := filtSrc(sources, myDists, enemyDists)
			plan := instantEat(body, facing, allCompetitive, &srcBG, &otherOcc)
			isInstantEat := false
			planReason := ""

			if plan.ok {
				di := dirInfo[plan.dir]
				if di != nil && di.alive {
					isInstantEat = true
					planReason = "eat"
				} else {
					altPlan := instantEat(body, facing, sources, &srcBG, &otherOcc)
					if altPlan.ok {
						altDi := dirInfo[altPlan.dir]
						if altDi != nil && altDi.alive {
							plan = altPlan
							isInstantEat = true
							planReason = "eat"
						} else {
							plan.ok = false
						}
					} else {
						plan.ok = false
					}
				}
			}

			available := vsrc[botIdx]
			if len(available) == 0 {
				available = sources
			}
			competitive := filtSrc(available, myDists, enemyDists)
			if len(competitive) == 0 {
				competitive = available
			}

			if !plan.ok {
				fillBG(&srcBG, competitive)

				maxDepth := 8
				if bodyLen <= 5 {
					maxDepth = 12
				}
				remaining := time.Until(turnDeadline)
				if remaining < 15*time.Millisecond {
					maxDepth = 4
				} else if remaining < 25*time.Millisecond {
					maxDepth = 6
				}

				plan = cmdBFS(body, facing, competitive, maxDepth, dirInfo, enemyDists, &srcBG, &otherOcc, turnDeadline)
				if plan.ok {
					planReason = "bfs"
				}

				if plan.ok && !isSafeDir(plan.dir, dirInfo, bodyLen) {
					if bs, ok := bestSafeDir(dirInfo); ok && isSafeDir(bs, dirInfo, bodyLen) {
						plan.dir = bs
						planReason = "safe"
					}
				}
			}

			if !plan.ok {
				if job, ok := supportJobs[bot.id]; ok {
					fillBG(&srcBG, sources)
					plan = bestGroundAction(body, facing, job.cell, dirInfo, enemies, &srcBG, &otherOcc, &eDanger)
					if plan.ok {
						planReason = "support"
					}
				}
			}

			if !plan.ok {
				fillBG(&srcBG, available)
				plan = bestAction(body, facing, available, dirInfo, enemies, enemyDists, &srcBG, &otherOcc, &eDanger)
				if plan.ok {
					planReason = "bmove"
				}
			}

			if !plan.ok {
				plan.dir = facing
				planReason = "face"
			}

			nextHead := Add(head, DirDelta[plan.dir])
			if grid.IsWall(nextHead) || otherOcc.Has(nextHead) {
				bestDir := DirNone
				bestFlood := -1
				for dir, di := range dirInfo {
					if !di.alive {
						continue
					}
					t := Add(head, DirDelta[dir])
					if otherOcc.Has(t) {
						continue
					}
					if di.flood > bestFlood {
						bestFlood = di.flood
						bestDir = dir
					}
				}
				if bestDir != DirNone {
					plan.dir = bestDir
					planReason = "escape"
				}
			}

			if !isInstantEat {
				if di, ok := dirInfo[plan.dir]; ok && di.alive {
					if di.flood < bodyLen+2 {
						if bs, ok := bestSafeDir(dirInfo); ok && dirInfo[bs].flood >= bodyLen*3 {
							plan.dir = bs
							planReason = "safe"
						}
					}
				}
			}

			// Head-lock rejection for long snakes: reject any move where
			// an enemy combo can leave us with zero valid follow-up moves.
			if bodyLen >= 6 && isHeadLockedWorstCase(body, facing, plan.dir, enemies, &otherOcc, &srcBG) {
				replaced := false
				bestFlood := -1
				vd, nd := validDirs(facing)
				for _, d := range vd[:nd] {
					if d == plan.dir {
						continue
					}
					di := dirInfo[d]
					if di == nil || !di.alive {
						continue
					}
					if isHeadLockedWorstCase(body, facing, d, enemies, &otherOcc, &srcBG) {
						continue
					}
					if di.flood > bestFlood {
						bestFlood = di.flood
						plan.dir = d
						planReason = "nolock"
						replaced = true
					}
				}
				_ = replaced
			}

			if plan.ok {
				plans = append(plans, botPlan{id: bot.id, body: append([]Point(nil), body...), facing: facing, dir: plan.dir, target: plan.target, reason: planReason, ok: true})
			} else {
				plans = append(plans, botPlan{id: bot.id, body: append([]Point(nil), body...), facing: facing, dir: plan.dir, target: Add(head, DirDelta[plan.dir]), reason: planReason})
			}
			plannedHeads.Set(Add(head, DirDelta[plan.dir]))
		}

		refinePlansWithOneTurnSafety(&rsc, mine, enemies, sources, plans, turnDeadline)

		// Final guard: never emit a move that sends head into neck (body[1]).
		for i := range plans {
			body := plans[i].body
			if len(body) < 2 {
				continue
			}
			neck := body[1]
			nextHead := Add(body[0], DirDelta[plans[i].dir])
			if nextHead == neck {
				// Pick any other direction that doesn't go into neck or wall.
				replaced := false
				for d := DirUp; d <= DirLeft; d++ {
					if d == plans[i].dir {
						continue
					}
					alt := Add(body[0], DirDelta[d])
					if alt == neck {
						continue
					}
					if !grid.IsWall(alt) {
						plans[i].dir = d
						plans[i].reason = "fix"
						replaced = true
						break
					}
				}
				if !replaced {
					// All directions blocked — pick anything that isn't neck.
					for d := DirUp; d <= DirLeft; d++ {
						alt := Add(body[0], DirDelta[d])
						if alt != neck {
							plans[i].dir = d
							plans[i].reason = "fix"
							break
						}
					}
				}
			}
		}

		var actions []string
		for _, plan := range plans {
			actions = append(actions, actionString(plan.id, plan.dir, plan.reason))
		}

		if debug {
			n := 0
			for _, plan := range plans {
				if plan.ok && plan.target.X >= 0 && plan.target.Y >= 0 && n < 4 {
					actions = append(actions, fmt.Sprintf("MARK %d %d", plan.target.X, plan.target.Y))
					n++
				}
			}
		}

		out := "WAIT"
		if len(actions) > 0 {
			out = strings.Join(actions, ";")
		}
		if debug {
			fmt.Fprintln(os.Stderr, out)
		}
		fmt.Println(out)
		turn++
	}
}
