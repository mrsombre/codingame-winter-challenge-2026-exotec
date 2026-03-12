package agentkit

import "sort"

type StaticSupportNode struct {
	Pos       Point
	Neighbors []int
}

type TargetApproach struct {
	SupportCell Point
	MinLen      int
}

// SupportTerrain stores static terrain support structure with precomputed distances.
type SupportTerrain struct {
	Grid           *ArenaGrid
	NodeID         []int
	Nodes          []StaticSupportNode
	ComponentID    []int
	ComponentCount int

	cells         int
	minLen        []int // [comp * cells + cellIdx] = min bot length
	approachCache map[Point][]int

	// Scratch buffer for apple-aware BFS, reused across calls.
	immBuf immediateBuf

	// Precomputed list of wall support cells: walls with open cell above.
	wallSupports []Point
}

// immediateBuf is a reusable scratch buffer for BFS calls.
type immediateBuf struct {
	visited []uint32
	gen     uint32
	buckets [][]immSt
	maxLen  int
}

type immSt struct {
	pos  Point
	run  int
	dist int
}

func (b *immediateBuf) init(cells, maxLen int) {
	b.maxLen = maxLen
	b.visited = make([]uint32, cells*(maxLen+1))
	b.buckets = make([][]immSt, maxLen+1)
}

func (b *immediateBuf) reset() {
	b.gen++
	if b.gen == 0 {
		for i := range b.visited {
			b.visited[i] = 0
		}
		b.gen = 1
	}
	for i := range b.buckets {
		b.buckets[i] = b.buckets[i][:0]
	}
}

func (b *immediateBuf) mark(key int) {
	b.visited[key] = b.gen
}

func (b *immediateBuf) seen(key int) bool {
	return b.visited[key] == b.gen
}

func NewSupportTerrain(grid *ArenaGrid) *SupportTerrain {
	t := &SupportTerrain{
		Grid:          grid,
		NodeID:        make([]int, grid.Width*grid.Height),
		cells:         grid.Width * grid.Height,
		approachCache: map[Point][]int{},
	}
	for i := range t.NodeID {
		t.NodeID[i] = -1
	}

	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			p := Point{X: x, Y: y}
			if grid.IsWall(p) || !t.HasStaticSupport(p) {
				continue
			}
			t.NodeID[grid.CellIdx(p)] = len(t.Nodes)
			t.Nodes = append(t.Nodes, StaticSupportNode{Pos: p})
		}
	}

	for id := range t.Nodes {
		p := t.Nodes[id].Pos
		neighbors := make([]int, 0, 8)
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				if nid := t.SupportNodeAt(Point{X: p.X + dx, Y: p.Y + dy}); nid != -1 {
					neighbors = append(neighbors, nid)
				}
			}
		}
		t.Nodes[id].Neighbors = neighbors
	}

	t.ComponentID = make([]int, len(t.Nodes))
	for i := range t.ComponentID {
		t.ComponentID[i] = -1
	}
	queue := make([]int, 0, len(t.Nodes))
	for id := range t.Nodes {
		if t.ComponentID[id] != -1 {
			continue
		}
		compID := t.ComponentCount
		t.ComponentCount++
		t.ComponentID[id] = compID
		queue = queue[:0]
		queue = append(queue, id)
		for i := 0; i < len(queue); i++ {
			for _, next := range t.Nodes[queue[i]].Neighbors {
				if t.ComponentID[next] != -1 {
					continue
				}
				t.ComponentID[next] = compID
				queue = append(queue, next)
			}
		}
	}

	// Precompute wall support list: wall cells with open cell directly above.
	for y := 1; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			support := Point{X: x, Y: y}
			if !grid.IsWall(support) {
				continue
			}
			stand := Point{X: x, Y: y - 1}
			if grid.IsWall(stand) {
				continue
			}
			t.wallSupports = append(t.wallSupports, support)
		}
	}

	t.precomputeMinLengths()

	maxLen := grid.Width + grid.Height
	if maxLen < 1 {
		maxLen = 1
	}
	t.immBuf.init(t.cells, maxLen)

	return t
}

// precomputeMinLengths uses bucket-BFS per component to fill minLen table.
func (t *SupportTerrain) precomputeMinLengths() {
	maxRun := t.Grid.Width + t.Grid.Height
	if maxRun < 1 {
		maxRun = 1
	}
	t.minLen = make([]int, t.ComponentCount*t.cells)
	for i := range t.minLen {
		t.minLen[i] = UnreachableDistance
	}

	type st struct {
		pos Point
		run int
	}

	for comp := 0; comp < t.ComponentCount; comp++ {
		off := comp * t.cells
		visited := make([]bool, t.cells*(maxRun+1))
		buckets := make([][]st, maxRun+1)

		for id, node := range t.Nodes {
			if t.ComponentID[id] != comp {
				continue
			}
			idx := t.Grid.CellIdx(node.Pos)
			visited[idx*(maxRun+1)+1] = true
			buckets[1] = append(buckets[1], st{pos: node.Pos, run: 1})
		}

		for L := 1; L <= maxRun; L++ {
			for i := 0; i < len(buckets[L]); i++ {
				cur := buckets[L][i]
				idx := t.Grid.CellIdx(cur.pos)
				if t.minLen[off+idx] > L {
					t.minLen[off+idx] = L
				}

				for dir := DirUp; dir <= DirLeft; dir++ {
					next := Add(cur.pos, DirectionDeltas[dir])
					if t.Grid.IsWall(next) {
						continue
					}
					nextRun := cur.run + 1
					if t.HasStaticSupport(next) {
						nextRun = 1
					}
					if nextRun > maxRun {
						continue
					}
					vKey := t.Grid.CellIdx(next)*(maxRun+1) + nextRun
					if visited[vKey] {
						continue
					}
					visited[vKey] = true
					cost := L
					if nextRun > L {
						cost = nextRun
					}
					buckets[cost] = append(buckets[cost], st{pos: next, run: nextRun})
				}
			}
		}
	}
}

func (t *SupportTerrain) HasStaticSupport(p Point) bool {
	return t.Grid.IsWall(Point{X: p.X, Y: p.Y + 1})
}

func (t *SupportTerrain) SupportNodeAt(p Point) int {
	if !t.Grid.InBounds(p) {
		return -1
	}
	return t.NodeID[t.Grid.CellIdx(p)]
}

func (t *SupportTerrain) AnchorNode(body []Point) int {
	for _, part := range body {
		if !t.HasStaticSupport(part) {
			continue
		}
		if id := t.SupportNodeAt(part); id != -1 {
			return id
		}
	}
	return -1
}

func (t *SupportTerrain) AnchorComponent(body []Point) int {
	if nodeID := t.AnchorNode(body); nodeID != -1 {
		return t.ComponentID[nodeID]
	}
	return -1
}

func (t *SupportTerrain) ApproachNodeIDs(target Point) []int {
	if cached, ok := t.approachCache[target]; ok {
		return append([]int(nil), cached...)
	}

	var nodes []int
	seen := map[int]bool{}
	if id := t.SupportNodeAt(target); id != -1 {
		nodes = append(nodes, id)
		seen[id] = true
	}
	for dir := DirUp; dir <= DirLeft; dir++ {
		id := t.SupportNodeAt(Add(target, DirectionDeltas[dir]))
		if id == -1 || seen[id] {
			continue
		}
		nodes = append(nodes, id)
		seen[id] = true
	}
	if len(nodes) == 0 {
		for span := 1; span <= 2 && len(nodes) == 0; span++ {
			bestY := UnreachableDistance
			for id, node := range t.Nodes {
				p := node.Pos
				if p.Y < target.Y || abs(p.X-target.X) > span {
					continue
				}
				if p.Y < bestY {
					bestY = p.Y
					nodes = nodes[:0]
					nodes = append(nodes, id)
					seen = map[int]bool{id: true}
					continue
				}
				if p.Y == bestY && !seen[id] {
					nodes = append(nodes, id)
					seen[id] = true
				}
			}
		}
	}

	t.approachCache[target] = append([]int(nil), nodes...)
	return append([]int(nil), nodes...)
}

// MinSoloLengthFromComponentToTarget returns the precomputed min bot length
// needed for a bot in the given component to reach target using static support only.
func (t *SupportTerrain) MinSoloLengthFromComponentToTarget(componentID int, target Point) int {
	if componentID < 0 || componentID >= t.ComponentCount || t.Grid.IsWall(target) {
		return UnreachableDistance
	}
	return t.minLen[componentID*t.cells+t.Grid.CellIdx(target)]
}

func (t *SupportTerrain) MinSoloLengthFromBodyToTarget(body []Point, target Point) int {
	return t.MinSoloLengthFromComponentToTarget(t.AnchorComponent(body), target)
}

// --- Apple-aware support finding ---

// minImmediateLengthFromSupportToTarget computes the min bot length needed
// to reach an adjacent cell of target, starting from the stand position
// (one row above supportCell). Uses single-pass bucket-BFS with reusable buffers.
func (t *SupportTerrain) minImmediateLengthFromSupportToTarget(supportCell, target Point, apples *BitGrid) (int, int) {
	stand := Point{X: supportCell.X, Y: supportCell.Y - 1}
	if stand.Y < 0 || t.Grid.IsWall(stand) {
		return UnreachableDistance, UnreachableDistance
	}

	var adj [4]Point
	adjN := 0
	for dir := DirUp; dir <= DirLeft; dir++ {
		head := Add(target, DirectionDeltas[Opposite(dir)])
		if !t.Grid.IsWall(head) {
			adj[adjN] = head
			adjN++
		}
	}
	if adjN == 0 {
		return UnreachableDistance, UnreachableDistance
	}

	isAppleTarget := apples != nil && apples.Has(target)
	buf := &t.immBuf
	maxLen := buf.maxLen
	buf.reset()

	startIdx := t.Grid.CellIdx(stand)
	buf.mark(startIdx*(maxLen+1) + 1)
	buf.buckets[1] = append(buf.buckets[1], immSt{pos: stand, run: 1, dist: 0})

	for L := 1; L <= maxLen; L++ {
		for i := 0; i < len(buf.buckets[L]); i++ {
			cur := buf.buckets[L][i]
			for a := 0; a < adjN; a++ {
				if cur.pos == adj[a] {
					return L, cur.dist
				}
			}

			for dir := DirUp; dir <= DirLeft; dir++ {
				next := Add(cur.pos, DirectionDeltas[dir])
				if t.Grid.IsWall(next) {
					continue
				}

				below := Point{X: next.X, Y: next.Y + 1}
				if below != supportCell && (!isAppleTarget || below != target) {
					if t.Grid.IsWall(below) || (apples != nil && apples.Has(below)) {
						continue
					}
				}

				nextRun := cur.run + 1
				if isAppleTarget && below == target {
					nextRun = 1
				}
				if nextRun > maxLen {
					continue
				}

				vKey := t.Grid.CellIdx(next)*(maxLen+1) + nextRun
				if buf.seen(vKey) {
					continue
				}
				buf.mark(vKey)

				cost := L
				if nextRun > L {
					cost = nextRun
				}
				buf.buckets[cost] = append(buf.buckets[cost], immSt{pos: next, run: nextRun, dist: cur.dist + 1})
			}
		}
	}

	return UnreachableDistance, UnreachableDistance
}

func (t *SupportTerrain) targetApproaches(apples *BitGrid, target Point) []TargetApproach {
	if t.Grid.IsWall(target) {
		return nil
	}

	var approaches []TargetApproach

	// Wall supports (precomputed list).
	for _, support := range t.wallSupports {
		if support.Y < target.Y || support == target {
			continue
		}
		minLen, _ := t.minImmediateLengthFromSupportToTarget(support, target, apples)
		if minLen != UnreachableDistance {
			approaches = append(approaches, TargetApproach{SupportCell: support, MinLen: minLen})
		}
	}

	// Apple supports.
	if apples != nil {
		for y := target.Y; y < t.Grid.Height; y++ {
			for x := 0; x < t.Grid.Width; x++ {
				p := Point{X: x, Y: y}
				if p == target || t.Grid.IsWall(p) || !apples.Has(p) {
					continue
				}
				stand := Point{X: x, Y: y - 1}
				if stand.Y < 0 || t.Grid.IsWall(stand) {
					continue
				}
				minLen, _ := t.minImmediateLengthFromSupportToTarget(p, target, apples)
				if minLen != UnreachableDistance {
					approaches = append(approaches, TargetApproach{SupportCell: p, MinLen: minLen})
				}
			}
		}
	}

	sort.Slice(approaches, func(i, j int) bool {
		if approaches[i].MinLen != approaches[j].MinLen {
			return approaches[i].MinLen < approaches[j].MinLen
		}
		if approaches[i].SupportCell.Y != approaches[j].SupportCell.Y {
			return approaches[i].SupportCell.Y < approaches[j].SupportCell.Y
		}
		return approaches[i].SupportCell.X < approaches[j].SupportCell.X
	})

	return approaches
}

func (t *SupportTerrain) closestSupports(apples *BitGrid, target Point) []TargetApproach {
	approaches := t.targetApproaches(apples, target)
	if len(approaches) == 0 {
		return nil
	}

	supports := make(map[Point]TargetApproach, len(approaches))
	for _, a := range approaches {
		supports[a.SupportCell] = a
	}

	componentOf := make(map[Point]int, len(supports))
	queue := make([]Point, 0, len(supports))
	componentCount := 0
	for support := range supports {
		if _, seen := componentOf[support]; seen {
			continue
		}
		componentOf[support] = componentCount
		queue = queue[:0]
		queue = append(queue, support)
		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			curStand := Point{X: cur.X, Y: cur.Y - 1}
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					next := Point{X: cur.X + dx, Y: cur.Y + dy}
					if _, ok := supports[next]; !ok {
						continue
					}
					if _, seen := componentOf[next]; seen {
						continue
					}
					nextStand := Point{X: next.X, Y: next.Y - 1}
					if abs(curStand.X-nextStand.X) > 1 || abs(curStand.Y-nextStand.Y) > 1 {
						continue
					}
					componentOf[next] = componentCount
					queue = append(queue, next)
				}
			}
		}
		componentCount++
	}

	bestByComponent := make(map[int]TargetApproach, componentCount)
	for _, a := range approaches {
		compID := componentOf[a.SupportCell]
		best, ok := bestByComponent[compID]
		if !ok ||
			a.MinLen < best.MinLen ||
			(a.MinLen == best.MinLen &&
				ManhattanDistance(a.SupportCell, target) < ManhattanDistance(best.SupportCell, target)) ||
			(a.MinLen == best.MinLen &&
				ManhattanDistance(a.SupportCell, target) == ManhattanDistance(best.SupportCell, target) &&
				(a.SupportCell.Y < best.SupportCell.Y ||
					(a.SupportCell.Y == best.SupportCell.Y && a.SupportCell.X < best.SupportCell.X))) {
			bestByComponent[compID] = a
		}
	}

	closest := make([]TargetApproach, 0, len(bestByComponent))
	for _, a := range bestByComponent {
		closest = append(closest, a)
	}

	sort.Slice(closest, func(i, j int) bool {
		if closest[i].MinLen != closest[j].MinLen {
			return closest[i].MinLen < closest[j].MinLen
		}
		di := ManhattanDistance(closest[i].SupportCell, target)
		dj := ManhattanDistance(closest[j].SupportCell, target)
		if di != dj {
			return di < dj
		}
		if closest[i].SupportCell.Y != closest[j].SupportCell.Y {
			return closest[i].SupportCell.Y < closest[j].SupportCell.Y
		}
		return closest[i].SupportCell.X < closest[j].SupportCell.X
	})

	return closest
}

func TargetApproaches(state *State, target Point) []TargetApproach {
	if state == nil || state.Grid == nil {
		return nil
	}
	terrain := state.Terrain
	if terrain == nil {
		terrain = NewSupportTerrain(state.Grid)
	}
	return terrain.targetApproaches(&state.Apples, target)
}

func ClosestSupports(state *State, target Point) []TargetApproach {
	if state == nil || state.Grid == nil {
		return nil
	}
	terrain := state.Terrain
	if terrain == nil {
		terrain = NewSupportTerrain(state.Grid)
	}
	return terrain.closestSupports(&state.Apples, target)
}
