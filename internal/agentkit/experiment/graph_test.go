package experiment

import (
	"strings"
	"testing"

	enginegrid "codingame/internal/engine/grid"
	"codingame/internal/agentkit/game"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const graphSeedLayout = `............................
............................
............................
.##......................##.
.###........#..#........###.
...#........#..#........#...
.........##......##.........
.....#................#.....
......#.....#..#.....#......
...........#....#...........
...##........##........##...
##.#.....#...##...#.....#.##
#..#....####....####....#..#
#.....######....######.....#
############################`

const graphSeed = int64(-6499872768487446000)

func TestGraphNodesUsePassableSideEdgesAndApples(t *testing.T) {
	graph := graphFromLayout(t, graphSeedRows(), []game.Point{
		{X: 11, Y: 6},
		{X: 13, Y: 6},
	})

	requireGraphNodeType(t, graph, game.Point{X: 3, Y: 9}, GraphNodeTypeEdge)
	requireGraphNodeType(t, graph, game.Point{X: 24, Y: 9}, GraphNodeTypeEdge)
	requireGraphNodeType(t, graph, game.Point{X: 11, Y: 6}, GraphNodeTypeApple)
	requireGraphNodeType(t, graph, game.Point{X: 13, Y: 6}, GraphNodeTypeApple)
	assert.Nil(t, graph.NodeAt(game.Point{X: 12, Y: 6}), "interior non-apple floor should not be a node")
	assert.Nil(t, graph.NodeAt(game.Point{X: 20, Y: 9}), "open area without support edge below should not be a node")
}

func TestGraphBuildsStraightArcsBetweenNodes(t *testing.T) {
	graph := graphFromLayout(t, graphSeedRows(), []game.Point{
		{X: 11, Y: 6},
		{X: 13, Y: 6},
	})

	requireGraphArc(t, graph, game.Point{X: 24, Y: 9}, game.DirLeft, game.Point{X: 23, Y: 9}, 1)
	requireGraphArc(t, graph, game.Point{X: 23, Y: 9}, game.DirRight, game.Point{X: 24, Y: 9}, 1)
	requireGraphArc(t, graph, game.Point{X: 3, Y: 9}, game.DirRight, game.Point{X: 4, Y: 9}, 1)
	requireGraphArc(t, graph, game.Point{X: 11, Y: 6}, game.DirRight, game.Point{X: 13, Y: 6}, 2)
	requireGraphArc(t, graph, game.Point{X: 13, Y: 6}, game.DirLeft, game.Point{X: 11, Y: 6}, 2)
}

func TestGraphPrecalcHigherClimbs(t *testing.T) {
	layout := []string{
		".....",
		".....",
		".....",
		"#####",
	}

	grid, graph := graphAndGridFromLayout(t, layout, []game.Point{
		{X: 1, Y: 2},
		{X: 1, Y: 1},
		{X: 1, Y: 0},
	})
	graph.PrecalcHigherClimbs(grid, nil, 1)

	midID := mustGraphNode(t, graph, game.Point{X: 1, Y: 1})
	topID := mustGraphNode(t, graph, game.Point{X: 1, Y: 0})
	lowID := mustGraphNode(t, graph, game.Point{X: 1, Y: 2})

	requireGraphClimb(t, graph, midID, game.Point{X: 1, Y: 2}, 2, 1)
	requireGraphClimb(t, graph, topID, game.Point{X: 1, Y: 1}, 2, 1)
	assert.Empty(t, graph.Nodes[lowID].ClimbIn, "lowest node should not need incoming climbs from below")
}

func graphSeedRows() []string {
	return strings.Split(graphSeedLayout, "\n")
}

func graphFromLayout(t *testing.T, layout []string, apples []game.Point) *Graph {
	t.Helper()
	_, graph := graphAndGridFromLayout(t, layout, apples)
	return graph
}

func graphAndGridFromLayout(t *testing.T, layout []string, apples []game.Point) (*game.AGrid, *Graph) {
	t.Helper()

	walls := make(map[game.Point]bool)
	for y, row := range layout {
		for x, ch := range row {
			if ch == '#' {
				walls[game.Point{X: x, Y: y}] = true
			}
		}
	}

	grid := game.NewAG(len(layout[0]), len(layout), walls)
	appleGrid := game.NewBG(grid.Width, grid.Height)
	for _, apple := range apples {
		require.Falsef(t, grid.IsWall(apple), "apple must be on a passable cell: %+v", apple)
		appleGrid.Set(apple)
	}

	return grid, NewGraph(grid, &appleGrid)
}

func requireGraphNodeType(t *testing.T, graph *Graph, pos game.Point, want GraphNodeType) {
	t.Helper()
	node := graph.NodeAt(pos)
	require.NotNilf(t, node, "expected node at %+v", pos)
	assert.Equal(t, want, node.Type)
}

func requireGraphArc(t *testing.T, graph *Graph, from game.Point, dir game.Direction, to game.Point, dist int) {
	t.Helper()
	node := graph.NodeAt(from)
	require.NotNilf(t, node, "expected node at %+v", from)

	targetID := graph.NodeIDAt(to)
	require.NotEqualf(t, -1, targetID, "expected node at %+v", to)

	for _, arc := range node.Arcs {
		if arc.Dir == dir && arc.To == targetID && arc.Dist == dist {
			return
		}
	}

	require.Failf(t, "missing arc", "from=%+v dir=%v to=%+v dist=%d arcs=%+v", from, dir, to, dist, node.Arcs)
}

func requireGraphClimb(t *testing.T, graph *Graph, toID int, from game.Point, minLen, dist int) {
	t.Helper()

	fromID := graph.NodeIDAt(from)
	require.NotEqualf(t, -1, fromID, "expected source node at %+v", from)

	for _, climb := range graph.Nodes[toID].ClimbIn {
		if climb.From == fromID && climb.MinLen == minLen && climb.Dist == dist {
			return
		}
	}

	require.Failf(t, "missing climb", "to=%+v from=%+v wantMinLen=%d wantDist=%d climbs=%+v",
		graph.Nodes[toID].Pos, from, minLen, dist, graph.Nodes[toID].ClimbIn)
}

func mustGraphNode(t *testing.T, graph *Graph, pos game.Point) int {
	t.Helper()
	id := graph.NodeIDAt(pos)
	require.NotEqualf(t, -1, id, "expected node at %+v", pos)
	return id
}

func TestGraphSeedMatchesGridMaker(t *testing.T) {
	rng := enginegrid.NewSHA1PRNG(graphSeed)
	gm := enginegrid.NewGridMaker(rng, 1)
	g := gm.Make()

	var got []string
	for y := 0; y < g.Height; y++ {
		row := make([]byte, g.Width)
		for x := 0; x < g.Width; x++ {
			row[x] = '.'
			if g.GetXY(x, y).Type == enginegrid.TileWall {
				row[x] = '#'
			}
		}
		got = append(got, string(row))
	}

	assert.Equal(t, graphSeedRows(), got)
}
