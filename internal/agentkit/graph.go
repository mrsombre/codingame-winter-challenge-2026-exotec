package agentkit

import "sort"

type GraphNodeType uint8

const (
	GraphNodeTypeNone GraphNodeType = iota
	GraphNodeTypeEdge
	GraphNodeTypeApple
	GraphNodeTypeEdgeApple
)

func (t GraphNodeType) HasEdge() bool {
	return t == GraphNodeTypeEdge || t == GraphNodeTypeEdgeApple
}

func (t GraphNodeType) HasApple() bool {
	return t == GraphNodeTypeApple || t == GraphNodeTypeEdgeApple
}

type GraphArc struct {
	To   int
	Dir  Direction
	Dist int
}

type GraphClimb struct {
	From   int
	Dir    Direction
	Dist   int
	MinLen int
}

type GraphNode struct {
	Pos     Point
	Type    GraphNodeType
	Arcs    []GraphArc
	ClimbIn []GraphClimb
}

type Graph struct {
	Width   int
	Height  int
	NodeIDs []int
	Nodes   []GraphNode
}

func NewGraph(grid *AGrid, apples *BitGrid) *Graph {
	if grid == nil {
		return &Graph{}
	}

	g := &Graph{
		Width:   grid.Width,
		Height:  grid.Height,
		NodeIDs: make([]int, grid.Width*grid.Height),
	}
	for i := range g.NodeIDs {
		g.NodeIDs[i] = -1
	}

	for y := 0; y < grid.Height; y++ {
		for x := 0; x < grid.Width; x++ {
			p := Point{X: x, Y: y}
			if grid.IsWall(p) {
				continue
			}

			isApple := apples != nil && apples.Has(p)
			if !g.isPassableEdge(grid, p) && !isApple {
				continue
			}

			id := len(g.Nodes)
			g.NodeIDs[grid.CIdx(p)] = id
			g.Nodes = append(g.Nodes, GraphNode{
				Pos:  p,
				Type: graphNodeType(g.isPassableEdge(grid, p), isApple),
			})
		}
	}

	for id := range g.Nodes {
		from := g.Nodes[id].Pos
		for dir := DirUp; dir <= DirLeft; dir++ {
			next := Add(from, DirDelta[dir])
			if grid.IsWall(next) {
				continue
			}

			dist := 1
			for !grid.IsWall(next) {
				if nid := g.NodeIDAt(next); nid != -1 {
					if nid != id {
						g.Nodes[id].Arcs = append(g.Nodes[id].Arcs, GraphArc{
							To:   nid,
							Dir:  dir,
							Dist: dist,
						})
					}
					break
				}
				next = Add(next, DirDelta[dir])
				dist++
			}
		}
	}

	return g
}

func (g *Graph) NodeIDAt(p Point) int {
	if g == nil || p.X < 0 || p.X >= g.Width || p.Y < 0 || p.Y >= g.Height {
		return -1
	}
	return g.NodeIDs[p.Y*g.Width+p.X]
}

func (g *Graph) NodeAt(p Point) *GraphNode {
	id := g.NodeIDAt(p)
	if id == -1 {
		return nil
	}
	return &g.Nodes[id]
}

// PrecalcHigherClimbs stores direct incoming climb costs for higher-ground
// nodes. It only considers immediate graph neighbors with a smaller Y target;
// downhill/flat movement is left to ordinary BFS + falling logic.
// startRun is the initial unsupported run at the source node.
func (g *Graph) PrecalcHigherClimbs(grid *AGrid, apples *BitGrid, startRun int) {
	if g == nil || grid == nil {
		return
	}
	if startRun < 1 {
		startRun = 1
	}

	for i := range g.Nodes {
		g.Nodes[i].ClimbIn = g.Nodes[i].ClimbIn[:0]
	}

	for fromID, from := range g.Nodes {
		for _, arc := range from.Arcs {
			to := g.Nodes[arc.To]
			if to.Pos.Y >= from.Pos.Y {
				continue
			}

			minLen := g.arcMinLen(grid, apples, fromID, arc, startRun)
			if minLen == Unreachable {
				continue
			}

			g.Nodes[arc.To].ClimbIn = append(g.Nodes[arc.To].ClimbIn, GraphClimb{
				From:   fromID,
				Dir:    arc.Dir,
				Dist:   arc.Dist,
				MinLen: minLen,
			})
		}
	}

	for i := range g.Nodes {
		sort.Slice(g.Nodes[i].ClimbIn, func(a, b int) bool {
			left := g.Nodes[i].ClimbIn[a]
			right := g.Nodes[i].ClimbIn[b]
			if left.MinLen != right.MinLen {
				return left.MinLen < right.MinLen
			}
			if left.Dist != right.Dist {
				return left.Dist < right.Dist
			}
			return left.From < right.From
		})
	}
}

func (g *Graph) isPassableEdge(grid *AGrid, p Point) bool {
	below := Point{X: p.X, Y: p.Y + 1}
	if !grid.IsWall(below) {
		return false
	}

	leftBelow := Point{X: below.X - 1, Y: below.Y}
	rightBelow := Point{X: below.X + 1, Y: below.Y}
	return !grid.IsWall(leftBelow) || !grid.IsWall(rightBelow)
}

func (g *Graph) arcMinLen(grid *AGrid, apples *BitGrid, fromID int, arc GraphArc, startRun int) int {
	if g == nil || grid == nil || fromID < 0 || fromID >= len(g.Nodes) {
		return Unreachable
	}
	if arc.To < 0 || arc.To >= len(g.Nodes) || startRun < 1 {
		return Unreachable
	}

	run := startRun
	maxRun := startRun
	cur := g.Nodes[fromID].Pos

	for step := 1; step <= arc.Dist; step++ {
		cur = Add(cur, DirDelta[arc.Dir])
		if grid.IsWall(cur) {
			return Unreachable
		}

		run++
		if graphHasSupport(grid, apples, cur) {
			run = 1
		}
		if run > maxRun {
			maxRun = run
		}
	}

	if cur != g.Nodes[arc.To].Pos {
		return Unreachable
	}

	return maxRun
}

func graphHasSupport(grid *AGrid, apples *BitGrid, p Point) bool {
	if grid.WBelow(p) {
		return true
	}
	if apples == nil {
		return false
	}
	return apples.Has(Point{X: p.X, Y: p.Y + 1})
}

func graphNodeType(isEdge, isApple bool) GraphNodeType {
	switch {
	case isEdge && isApple:
		return GraphNodeTypeEdgeApple
	case isEdge:
		return GraphNodeTypeEdge
	case isApple:
		return GraphNodeTypeApple
	default:
		return GraphNodeTypeNone
	}
}
