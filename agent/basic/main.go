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

type Body struct {
	Parts [MaxBody]Point
	Len   int
}

func NewBody(points []Point) Body {
	var b Body
	b.Set(points)
	return b
}
func (b *Body) Set(points []Point) {
	if len(points) > MaxBody {
		panic("body too large")
	}
	b.Len = len(points)
	copy(b.Parts[:b.Len], points)
}
func (b *Body) Reset() { b.Len = 0 }
func (b Body) Slice() []Point { return b.Parts[:b.Len] }
func (b Body) Head() (Point, bool) {
	if b.Len == 0 {
		return Point{}, false
	}
	return b.Parts[0], true
}
func (b Body) Tail() (Point, bool) {
	if b.Len == 0 {
		return Point{}, false
	}
	return b.Parts[b.Len-1], true
}
func (b Body) Facing() Direction {
	if b.Len < 2 {
		return DirNone
	}
	return FacingPts(b.Parts[0], b.Parts[1])
}
func (b Body) Contains(p Point) bool {
	for i := 0; i < b.Len; i++ {
		if b.Parts[i] == p {
			return true
		}
	}
	return false
}
func (b *Body) Copy(other Body) {
	b.Len = other.Len
	copy(b.Parts[:b.Len], other.Parts[:other.Len])
}

type Bot struct {
	ID, Owner int
	Alive     bool
	Body      Body
}

type State struct {
	Grid     *AGrid
	Terr     *STerrain
	Apples   BitGrid
	Bots     []Bot
	MvBuf    [4]Direction
	DistVals []int
	DistQ    []Point
}

func NewState(grid *AGrid) State {
	s := State{Grid: grid}
	if grid != nil {
		n := grid.Width * grid.Height
		s.Terr = NewSTerrain(grid)
		s.Apples = NewBG(grid.Width, grid.Height)
		s.DistVals = make([]int, n)
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
type SBResult struct {
	Waypoints []Point
	Approach  Point
	MinLen    int
	Dist      int
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
func (b *SBBuf) Mark(key int)      { b.Visited[key] = b.Gen }
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

type STerrain struct {
	Grid     *AGrid
	NID      []int
	Nodes    []SupNode
	CompID   []int
	CompCnt  int
	Cells    int
	MinLen   []int
	ImmBuf IBuf
	SBBuf    SBBuf
	WallSup  []Point
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
		Grid:     grid,
		NID:      make([]int, grid.Width*grid.Height),
		Cells:    grid.Width * grid.Height,
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
	for y := 1; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			sup := Point{x, y}
			if !grid.IsWall(sup) {
				continue
			}
			stand := Point{x, y - 1}
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
	buf.SetPrev(sKey, -1)
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
func (t *STerrain) sbReconstruct(goalKey, w, stride int, approach Point, minLen, dist int) *SBResult {
	buf := &t.SBBuf
	waypoints := make([]Point, 0, 8)
	k := goalKey
	for {
		posIdx := k / stride
		pos := Point{posIdx % w, posIdx / w}
		if t.Grid.WBelow(pos) {
			waypoints = append(waypoints, pos)
		}
		prev, ok := buf.GetPrev(k)
		if !ok || prev == -1 {
			break
		}
		k = int(prev)
	}
	for i, j := 0, len(waypoints)-1; i < j; i, j = i+1, j-1 {
		waypoints[i], waypoints[j] = waypoints[j], waypoints[i]
	}
	return &SBResult{waypoints, approach, minLen, dist}
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

func legalDirs(facing Direction) [3]Direction {
	back := Opp(facing)
	var out [3]Direction
	n := 0
	for d := DirUp; d <= DirLeft; d++ {
		if d != back {
			out[n] = d
			n++
		}
	}
	return out
}

func srcScore(head, target Point) int {
	d := MDist(head, target)
	if target.Y < head.Y {
		d += head.Y - target.Y
	}
	if grid.WBelow(target) {
		d--
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

func instantEat(body []Point, facing Direction, sources []Point, srcBG, occupied *BitGrid) SearchResult {
	head := body[0]
	var best SearchResult
	for _, dir := range state.VMoves(head, facing) {
		target := Add(head, DirDelta[dir])
		if !srcBG.Has(target) {
			continue
		}
		_, _, alive, _, _ := simMove(body, facing, dir, srcBG, occupied)
		if !alive {
			continue
		}
		score := srcScore(head, target)
		if !best.ok || score < best.score {
			best = SearchResult{dir: dir, target: target, steps: 1, score: score, ok: true}
		}
	}
	return best
}

func pathBFS(body []Point, facing Direction, sources []Point,
	maxDepth int, dirInfo map[Direction]*DirInfo, enemyDists []int,
	srcBG, occupied *BitGrid, deadline time.Time) SearchResult {

	if len(sources) == 0 {
		return SearchResult{}
	}

	type qItem struct {
		body  []Point
		face  Direction
		first Direction
		depth int
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
		for _, dir := range state.VMoves(head, item.face) {
			nb, nf, alive, ate, eatenAt := simMove(item.body, item.face, dir, srcBG, occupied)
			if !alive {
				continue
			}
			first := item.first
			if first == DirNone {
				first = dir
			}
			if ate && srcBG.Has(eatenAt) {
				rawSteps := item.depth + 1
				score := rawSteps * 1000
				score += srcScore(body[0], eatenAt)
				if di, ok := dirInfo[first]; ok && di.alive {
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
				if state.Terr.MinBodyLen(body, eatenAt) <= bodyLen {
					score -= 200
				}
				cand := SearchResult{dir: first, target: eatenAt, steps: rawSteps, score: score, ok: true}
				if !best.ok || cand.score < best.score {
					best = cand
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
			queue = append(queue, qItem{body: cp, face: nf, first: first, depth: item.depth + 1})
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
	type srcCand struct {
		pt   Point
		dist int
	}
	var reachable []srcCand
	for _, s := range sources {
		res := state.Terr.SupPathBFS(head, initRun, s, srcBG)
		if res != nil && res.MinLen <= bodyLen {
			reachable = append(reachable, srcCand{s, res.Dist})
		}
	}

	var best SearchResult
	for _, dir := range legalDirs(facing) {
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
					d = di.dists[c.pt.Y*W+c.pt.X]
				} else {
					d = srcScore(nb[0], c.pt)
				}
				if d < bestDist {
					bestDist = d
					bestTarget = c.pt
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
				for _, edir := range legalDirs(e.facing) {
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

func calcEnemyDist(enemies []enemyInfo, allOcc *BitGrid) []int {
	n := W * H
	result := make([]int, n)
	for i := range result {
		result[i] = Unreachable
	}
	for _, e := range enemies {
		blocked := occExcept(allOcc, e.body)
		_, eDists := floodDist(e.head, &blocked)
		for i, d := range eDists {
			if d < result[i] {
				result[i] = d
			}
		}
	}
	return result
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

		type botEntry struct {
			id   int
			body []Point
		}
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

		eDanger := NewBG(W, H)
		for _, e := range enemies {
			for _, d := range legalDirs(e.facing) {
				eDanger.Set(Add(e.head, DirDelta[d]))
			}
		}

		enemyDists := calcEnemyDist(enemies, &allOcc)

		sort.Slice(mine, func(i, j int) bool {
			di, dj := Unreachable, Unreachable
			for _, s := range sources {
				if d := MDist(mine[i].body[0], s); d < di {
					di = d
				}
				if d := MDist(mine[j].body[0], s); d < dj {
					dj = d
				}
			}
			if di != dj {
				return di < dj
			}
			return mine[i].id < mine[j].id
		})

		vsrc := make([][]Point, len(mine))
		{
			botDists := make([][]int, len(mine))
			for i, bot := range mine {
				occ := occExcept(&allOcc, bot.body)
				_, botDists[i] = floodDist(bot.body[0], &occ)
			}
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
		}

		var actions []string
		var marks []Point
		plannedHeads := NewBG(W, H)

		for botIdx, bot := range mine {
			body := bot.body
			head := body[0]
			facing := DirUp
			if len(body) >= 2 {
				facing = FacingPts(body[0], body[1])
			}
			bodyLen := len(body)

			otherOcc := occExcept(&allOcc, body)
			for i := range otherOcc.Bits {
				otherOcc.Bits[i] |= plannedHeads.Bits[i]
			}

			dirInfo := calcDirInfo(body, facing, &otherOcc)

			_, myDists := floodDist(head, &otherOcc)

			srcBG := NewBG(W, H)
			fillBG(&srcBG, sources)
			allCompetitive := filtSrc(sources, myDists, enemyDists)
			plan := instantEat(body, facing, allCompetitive, &srcBG, &otherOcc)
			isInstantEat := false

			if plan.ok {
				di := dirInfo[plan.dir]
				if di != nil && di.alive {
					isInstantEat = true
				} else {
					altPlan := instantEat(body, facing, sources, &srcBG, &otherOcc)
					if altPlan.ok {
						altDi := dirInfo[altPlan.dir]
						if altDi != nil && altDi.alive {
							plan = altPlan
							isInstantEat = true
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

				plan = pathBFS(body, facing, competitive, maxDepth, dirInfo, enemyDists, &srcBG, &otherOcc, turnDeadline)

				if plan.ok && !isSafeDir(plan.dir, dirInfo, bodyLen) {
					if bs, ok := bestSafeDir(dirInfo); ok && isSafeDir(bs, dirInfo, bodyLen) {
						plan.dir = bs
					}
				}
			}

			if !plan.ok {
				fillBG(&srcBG, available)
				plan = bestAction(body, facing, available, dirInfo, enemies, enemyDists, &srcBG, &otherOcc, &eDanger)
			}

			if !plan.ok {
				plan.dir = facing
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
				}
			}

			if !isInstantEat {
				if di, ok := dirInfo[plan.dir]; ok && di.alive {
					if di.flood < bodyLen+2 {
						if bs, ok := bestSafeDir(dirInfo); ok && dirInfo[bs].flood >= bodyLen*3 {
							plan.dir = bs
						}
					}
				}
			}

			if plan.ok {
				marks = append(marks, plan.target)
			}
			plannedHeads.Set(Add(head, DirDelta[plan.dir]))
			actions = append(actions, fmt.Sprintf("%d %s", bot.id, DirName[plan.dir]))
		}

		for i, m := range marks {
			if i >= 4 {
				break
			}
			actions = append(actions, fmt.Sprintf("MARK %d %d", m.X, m.Y))
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
