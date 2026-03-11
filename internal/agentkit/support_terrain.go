package agentkit

import "sort"

type StaticSupportNode struct {
	Pos       Point
	Neighbors []int
}

type supportTargetKey struct {
	ComponentID int
	Target      Point
}

type TargetApproach struct {
	SupportCell Point
	MinLen      int
}

// SupportTerrain stores static terrain-only support structure and solo climb gates.
type SupportTerrain struct {
	Grid           *ArenaGrid
	NodeID         []int
	Nodes          []StaticSupportNode
	ComponentID    []int
	ComponentCount int

	approachCache map[Point][]int
	minLenCache   map[supportTargetKey]int
}

func NewSupportTerrain(grid *ArenaGrid) *SupportTerrain {
	terrain := &SupportTerrain{
		Grid:          grid,
		NodeID:        make([]int, grid.Width*grid.Height),
		approachCache: map[Point][]int{},
		minLenCache:   map[supportTargetKey]int{},
	}
	for i := range terrain.NodeID {
		terrain.NodeID[i] = -1
	}

	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			p := Point{X: x, Y: y}
			if grid.IsWall(p) || !terrain.HasStaticSupport(p) {
				continue
			}
			terrain.NodeID[grid.cellIdx(p)] = len(terrain.Nodes)
			terrain.Nodes = append(terrain.Nodes, StaticSupportNode{Pos: p})
		}
	}

	for id := range terrain.Nodes {
		p := terrain.Nodes[id].Pos
		neighbors := make([]int, 0, 8)
		for dy := -1; dy <= 1; dy++ {
			for dx := -1; dx <= 1; dx++ {
				if dx == 0 && dy == 0 {
					continue
				}
				nid := terrain.SupportNodeAt(Point{X: p.X + dx, Y: p.Y + dy})
				if nid == -1 {
					continue
				}
				neighbors = append(neighbors, nid)
			}
		}
		terrain.Nodes[id].Neighbors = neighbors
	}

	terrain.ComponentID = make([]int, len(terrain.Nodes))
	for i := range terrain.ComponentID {
		terrain.ComponentID[i] = -1
	}

	queue := make([]int, 0, len(terrain.Nodes))
	for id := range terrain.Nodes {
		if terrain.ComponentID[id] != -1 {
			continue
		}
		compID := terrain.ComponentCount
		terrain.ComponentCount++
		terrain.ComponentID[id] = compID
		queue = queue[:0]
		queue = append(queue, id)
		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			for _, next := range terrain.Nodes[cur].Neighbors {
				if terrain.ComponentID[next] != -1 {
					continue
				}
				terrain.ComponentID[next] = compID
				queue = append(queue, next)
			}
		}
	}

	return terrain
}

func (t *SupportTerrain) HasStaticSupport(p Point) bool {
	return t.Grid.IsWall(Point{X: p.X, Y: p.Y + 1})
}

func (t *SupportTerrain) SupportNodeAt(p Point) int {
	if !t.Grid.InBounds(p) {
		return -1
	}
	return t.NodeID[t.Grid.cellIdx(p)]
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

func (t *SupportTerrain) MinSoloLengthFromComponentToTarget(componentID int, target Point) int {
	if componentID < 0 || componentID >= t.ComponentCount || t.Grid.IsWall(target) {
		return UnreachableDistance
	}
	key := supportTargetKey{ComponentID: componentID, Target: target}
	if cached, ok := t.minLenCache[key]; ok {
		return cached
	}

	type state struct {
		pos Point
		run int
	}

	maxLen := t.Grid.Width + t.Grid.Height
	if maxLen < 1 {
		maxLen = 1
	}

	for need := 1; need <= maxLen; need++ {
		visited := make([][]bool, t.Grid.Width*t.Grid.Height)
		for i := range visited {
			visited[i] = make([]bool, need+1)
		}

		queue := make([]state, 0, len(t.Nodes))
		for id, node := range t.Nodes {
			if t.ComponentID[id] != componentID {
				continue
			}
			idx := t.Grid.cellIdx(node.Pos)
			visited[idx][1] = true
			queue = append(queue, state{pos: node.Pos, run: 1})
		}

		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			if cur.pos == target {
				t.minLenCache[key] = need
				return need
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
				if nextRun > need {
					continue
				}
				idx := t.Grid.cellIdx(next)
				if visited[idx][nextRun] {
					continue
				}
				visited[idx][nextRun] = true
				queue = append(queue, state{pos: next, run: nextRun})
			}
		}
	}

	t.minLenCache[key] = UnreachableDistance
	return UnreachableDistance
}

func (t *SupportTerrain) MinSoloLengthFromBodyToTarget(body []Point, target Point) int {
	return t.MinSoloLengthFromComponentToTarget(t.AnchorComponent(body), target)
}

func (t *SupportTerrain) hasSupportBelowWithApples(p Point, apples *BitGrid) bool {
	below := Point{X: p.X, Y: p.Y + 1}
	if t.Grid.IsWall(below) {
		return true
	}
	return apples != nil && apples.Has(below)
}

func (t *SupportTerrain) minLengthFromStandToPoint(stand, target Point, apples *BitGrid) (int, int) {
	if t.Grid.IsWall(stand) || t.Grid.IsWall(target) {
		return UnreachableDistance, UnreachableDistance
	}

	type state struct {
		pos Point
		run int
	}

	maxLen := t.Grid.Width + t.Grid.Height
	if maxLen < 1 {
		maxLen = 1
	}

	for need := 1; need <= maxLen; need++ {
		visited := make([][]bool, t.Grid.Width*t.Grid.Height)
		dist := make([][]int, t.Grid.Width*t.Grid.Height)
		for i := range visited {
			visited[i] = make([]bool, need+1)
			dist[i] = make([]int, need+1)
			for run := range dist[i] {
				dist[i][run] = UnreachableDistance
			}
		}

		queue := make([]state, 0, t.Grid.Width*t.Grid.Height)
		startIdx := t.Grid.cellIdx(stand)
		visited[startIdx][1] = true
		dist[startIdx][1] = 0
		queue = append(queue, state{pos: stand, run: 1})

		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			curIdx := t.Grid.cellIdx(cur.pos)
			curDist := dist[curIdx][cur.run]
			if cur.pos == target {
				return need, curDist
			}

			for dir := DirUp; dir <= DirLeft; dir++ {
				next := Add(cur.pos, DirectionDeltas[dir])
				if t.Grid.IsWall(next) {
					continue
				}
				nextRun := cur.run + 1
				if t.hasSupportBelowWithApples(next, apples) {
					nextRun = 1
				}
				if nextRun > need {
					continue
				}
				nextIdx := t.Grid.cellIdx(next)
				if visited[nextIdx][nextRun] {
					continue
				}
				visited[nextIdx][nextRun] = true
				dist[nextIdx][nextRun] = curDist + 1
				queue = append(queue, state{pos: next, run: nextRun})
			}
		}
	}

	return UnreachableDistance, UnreachableDistance
}

func (t *SupportTerrain) minImmediateLengthFromSupportToTarget(supportCell, target Point, apples *BitGrid) (int, int) {
	stand := Point{X: supportCell.X, Y: supportCell.Y - 1}
	if stand.Y < 0 || t.Grid.IsWall(stand) {
		return UnreachableDistance, UnreachableDistance
	}

	adjacent := make([]Point, 0, 4)
	for dir := DirUp; dir <= DirLeft; dir++ {
		head := Add(target, DirectionDeltas[Opposite(dir)])
		if t.Grid.IsWall(head) {
			continue
		}
		adjacent = append(adjacent, head)
	}
	if len(adjacent) == 0 {
		return UnreachableDistance, UnreachableDistance
	}

	type state struct {
		pos Point
		run int
	}

	isAppleTarget := apples != nil && apples.Has(target)
	maxLen := t.Grid.Width + t.Grid.Height
	if maxLen < 1 {
		maxLen = 1
	}

	for need := 1; need <= maxLen; need++ {
		visited := make([][]bool, t.Grid.Width*t.Grid.Height)
		dist := make([][]int, t.Grid.Width*t.Grid.Height)
		for i := range visited {
			visited[i] = make([]bool, need+1)
			dist[i] = make([]int, need+1)
			for run := range dist[i] {
				dist[i][run] = UnreachableDistance
			}
		}

		queue := make([]state, 0, t.Grid.Width*t.Grid.Height)
		startIdx := t.Grid.cellIdx(stand)
		visited[startIdx][1] = true
		dist[startIdx][1] = 0
		queue = append(queue, state{pos: stand, run: 1})

		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			curIdx := t.Grid.cellIdx(cur.pos)
			curDist := dist[curIdx][cur.run]
			for _, head := range adjacent {
				if cur.pos == head {
					return need, curDist
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
				if nextRun > need {
					continue
				}
				nextIdx := t.Grid.cellIdx(next)
				if visited[nextIdx][nextRun] {
					continue
				}
				visited[nextIdx][nextRun] = true
				dist[nextIdx][nextRun] = curDist + 1
				queue = append(queue, state{pos: next, run: nextRun})
			}
		}
	}

	return UnreachableDistance, UnreachableDistance
}

func (t *SupportTerrain) targetApproaches(apples *BitGrid, target Point) []TargetApproach {
	if t.Grid.IsWall(target) {
		return nil
	}

	var supportCells []TargetApproach
	for y := target.Y; y < t.Grid.Height; y++ {
		for x := 0; x < t.Grid.Width; x++ {
			support := Point{X: x, Y: y}
			if support == target {
				continue
			}
			stand := Point{X: x, Y: y - 1}
			if stand.Y < 0 || t.Grid.IsWall(stand) {
				continue
			}
			if t.Grid.IsWall(support) {
				supportCells = append(supportCells, TargetApproach{
					SupportCell: support,
				})
				continue
			}
			if apples != nil && apples.Has(support) {
				supportCells = append(supportCells, TargetApproach{
					SupportCell: support,
				})
			}
		}
	}

	approaches := make([]TargetApproach, 0, len(supportCells))
	for _, base := range supportCells {
		best := TargetApproach{}
		best.MinLen = UnreachableDistance
		minLen, dist := t.minImmediateLengthFromSupportToTarget(base.SupportCell, target, apples)
		bestDist := dist
		best = base
		best.MinLen = minLen
		if best.MinLen != UnreachableDistance {
			approaches = append(approaches, best)
			_ = bestDist
		}
	}

	sort.Slice(approaches, func(i, j int) bool {
		if approaches[i].MinLen != approaches[j].MinLen {
			return approaches[i].MinLen < approaches[j].MinLen
		}
		if approaches[i].SupportCell.Y != approaches[j].SupportCell.Y {
			return approaches[i].SupportCell.Y < approaches[j].SupportCell.Y
		}
		if approaches[i].SupportCell.X != approaches[j].SupportCell.X {
			return approaches[i].SupportCell.X < approaches[j].SupportCell.X
		}
		return false
	})

	return approaches
}

func (t *SupportTerrain) closestSupports(apples *BitGrid, target Point) []TargetApproach {
	approaches := t.targetApproaches(apples, target)
	if len(approaches) == 0 {
		return nil
	}

	type compState struct {
		cell Point
	}

	supports := make(map[Point]TargetApproach, len(approaches))
	for _, approach := range approaches {
		supports[approach.SupportCell] = approach
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
	for _, approach := range approaches {
		compID := componentOf[approach.SupportCell]
		best, ok := bestByComponent[compID]
		if !ok ||
			approach.MinLen < best.MinLen ||
			(approach.MinLen == best.MinLen &&
				ManhattanDistance(approach.SupportCell, target) < ManhattanDistance(best.SupportCell, target)) ||
			(approach.MinLen == best.MinLen &&
				ManhattanDistance(approach.SupportCell, target) == ManhattanDistance(best.SupportCell, target) &&
				(approach.SupportCell.Y < best.SupportCell.Y ||
					(approach.SupportCell.Y == best.SupportCell.Y && approach.SupportCell.X < best.SupportCell.X))) {
			bestByComponent[compID] = approach
		}
	}

	closest := make([]TargetApproach, 0, len(bestByComponent))
	for _, approach := range bestByComponent {
		closest = append(closest, approach)
	}

	sort.Slice(closest, func(i, j int) bool {
		if closest[i].MinLen != closest[j].MinLen {
			return closest[i].MinLen < closest[j].MinLen
		}
		if ManhattanDistance(closest[i].SupportCell, target) != ManhattanDistance(closest[j].SupportCell, target) {
			return ManhattanDistance(closest[i].SupportCell, target) < ManhattanDistance(closest[j].SupportCell, target)
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
