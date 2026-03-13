package engine

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"codingame/internal/agentkit"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSupportPlannerLetsShortBotGroundLongerOne(t *testing.T) {
	layout := []string{
		".......",
		"..#....",
		"..#....",
		"..#....",
		"..#....",
		"#######",
	}
	targetApple := Coord{X: 3, Y: 0}
	game := newSupportAcceptanceGame(layout,
		[]Coord{targetApple},
		[][]Coord{
			{{X: 4, Y: 3}, {X: 4, Y: 4}, {X: 4, Y: 5}},               // short supporter
			{{X: 0, Y: 2}, {X: 0, Y: 3}, {X: 0, Y: 4}, {X: 0, Y: 5}}, // longer climber
		},
		[][]Coord{
			{{X: 6, Y: 3}, {X: 6, Y: 4}, {X: 6, Y: 5}},
			{{X: 6, Y: 0}, {X: 6, Y: 1}, {X: 6, Y: 2}},
		},
	)

	state, mine, _, sources := supportAcceptanceState(game)
	require.Len(t, mine, 2)
	require.Equal(t, []agentkit.Point{{X: 3, Y: 0}}, sources)

	climber := mine[1]
	path := state.Terr.SupPathBFS(climber.Body[0], state.Terr.BodyInitRun(climber.Body), sources[0], &state.Apples)
	require.NotNil(t, path, "expected climber path analysis to exist")
	assert.Equal(t, 5, path.MinLen, "climber should be exactly one segment short before support")
	assert.Equal(t, 4, len(climber.Body))

	preferred, botDists := supportAcceptanceAssignments(state, mine)
	jobs := state.PlanSupportJobs(mine, preferred, sources, botDists, time.Now().Add(time.Second))
	job, ok := jobs[mine[0].ID]
	require.True(t, ok, "expected the short bot to receive a support job")
	assert.Equal(t, mine[1].ID, job.ClimberID)
	assert.Equal(t, agentkit.Point{X: 3, Y: 0}, job.Apple)
	assert.Equal(t, agentkit.Point{X: 3, Y: 3}, job.Cell)

	moves, ok := findSequenceToSupportCell(layout,
		[]Coord{targetApple},
		[][]Coord{
			{{X: 4, Y: 3}, {X: 4, Y: 4}, {X: 4, Y: 5}},
			{{X: 0, Y: 2}, {X: 0, Y: 3}, {X: 0, Y: 4}, {X: 0, Y: 5}},
		},
		[][]Coord{
			{{X: 6, Y: 3}, {X: 6, Y: 4}, {X: 6, Y: 5}},
			{{X: 6, Y: 0}, {X: 6, Y: 1}, {X: 6, Y: 2}},
		},
		Coord{X: job.Cell.X, Y: job.Cell.Y},
		5,
	)
	require.True(t, ok, "expected to move the short bot onto the planned support cell")

	sawSupportCell := false
	for turn, move := range moves {
		game.Players[0].birds[0].Direction = move[0]
		game.Players[0].birds[1].Direction = move[1]

		for _, opp := range game.Players[1].birds {
			if !opp.Alive || len(opp.Body) == 0 {
				continue
			}
			opp.Direction = DirNorth
			if opp.HeadPos().Y == 0 {
				opp.Direction = DirSouth
			}
		}

		game.PerformGameUpdate(turn)

		if bodyContainsCoord(game.Players[0].birds[0].Body, Coord{X: job.Cell.X, Y: job.Cell.Y}) {
			sawSupportCell = true
		}

		assert.True(t, game.Grid.HasApple(targetApple), "apple should still exist while support is being built")
		assert.True(t, game.Players[1].birds[0].Alive, "opponent bird should stay alive while support is being built")
	}

	assert.True(t, sawSupportCell, "supporter never reached the planned support cell")

	postState, _, _, _ := supportAcceptanceState(game)
	minLenFromSupport, climbDist := postState.Terr.MinImmLen(job.Cell, job.Apple, &postState.Apples)
	assert.Equal(t, 4, len(climber.Body))
	assert.LessOrEqual(t, minLenFromSupport, len(climber.Body), "planned support cell should make the apple reachable for the longer bot")
	assert.Greater(t, climbDist, 0, "support cell should give a real climb route to the apple")
}

func newSupportAcceptanceGame(layout []string, apples []Coord, p0Bodies, p1Bodies [][]Coord) *Game {
	grid := NewGrid(len(layout[0]), len(layout))
	for y, row := range layout {
		for x, ch := range row {
			if ch == '#' {
				grid.GetXY(x, y).Type = TileWall
			}
		}
	}
	grid.Apples = append(grid.Apples, apples...)
	grid.Spawns = []Coord{{X: 0, Y: 0}, {X: len(layout[0]) - 1, Y: 0}}

	p0 := NewPlayer(0)
	p1 := NewPlayer(1)
	game := &Game{Grid: grid}
	game.Init([]*Player{p0, p1})

	for i, body := range p0Bodies {
		p0.birds[i].Alive = true
		p0.birds[i].Body = append([]Coord(nil), body...)
	}
	for i, body := range p1Bodies {
		p1.birds[i].Alive = true
		p1.birds[i].Body = append([]Coord(nil), body...)
	}
	game.Players = []*Player{p0, p1}
	return game
}

func supportAcceptanceState(game *Game) (*agentkit.State, []agentkit.MyBotInfo, []agentkit.EnemyInfo, []agentkit.Point) {
	walls := make(map[agentkit.Point]bool)
	for y := 0; y < game.Grid.Height; y++ {
		for x := 0; x < game.Grid.Width; x++ {
			if game.Grid.GetXY(x, y).Type == TileWall {
				walls[agentkit.Point{X: x, Y: y}] = true
			}
		}
	}

	ag := agentkit.NewAG(game.Grid.Width, game.Grid.Height, walls)
	state := agentkit.NewState(ag)

	sources := make([]agentkit.Point, len(game.Grid.Apples))
	for i, apple := range game.Grid.Apples {
		p := agentkit.Point{X: apple.X, Y: apple.Y}
		sources[i] = p
		state.Apples.Set(p)
	}

	var mine []agentkit.MyBotInfo
	var enemies []agentkit.EnemyInfo
	for _, bird := range game.LiveBirds() {
		body := make([]agentkit.Point, len(bird.Body))
		for i, c := range bird.Body {
			body[i] = agentkit.Point{X: c.X, Y: c.Y}
		}
		if bird.Owner.GetIndex() == 0 {
			mine = append(mine, agentkit.MyBotInfo{ID: bird.ID, Body: body})
			continue
		}
		facing := agentkit.DirUp
		if len(body) >= 2 {
			facing = agentkit.FacingPts(body[0], body[1])
		}
		enemies = append(enemies, agentkit.EnemyInfo{
			Head:    body[0],
			Facing:  facing,
			BodyLen: len(body),
			Body:    body,
		})
	}

	return &state, mine, enemies, sources
}

func supportAcceptanceAssignments(state *agentkit.State, mine []agentkit.MyBotInfo) ([][]agentkit.Point, [][]int) {
	preferred := make([][]agentkit.Point, len(mine))
	botDists := make([][]int, len(mine))

	allOcc := agentkit.NewBG(state.Width(), state.Height())
	for _, bot := range mine {
		for _, p := range bot.Body {
			allOcc.Set(p)
		}
	}

	for i, bot := range mine {
		occ := agentkit.OccExcept(&allOcc, bot.Body)
		_, botDists[i] = state.FloodDist(bot.Body[0], &occ)
	}

	for y := 0; y < state.Height(); y++ {
		for x := 0; x < state.Width(); x++ {
			p := agentkit.Point{X: x, Y: y}
			if !state.Apples.Has(p) {
				continue
			}
			bestBot := -1
			bestDist := agentkit.Unreachable
			for i := range mine {
				d := botDists[i][p.Y*state.Width()+p.X]
				if d < bestDist {
					bestDist = d
					bestBot = i
				}
			}
			if bestBot >= 0 {
				preferred[bestBot] = append(preferred[bestBot], p)
			}
		}
	}

	return preferred, botDists
}

func bodyContainsCoord(body []Coord, target Coord) bool {
	for _, c := range body {
		if c == target {
			return true
		}
	}
	return false
}

type supportSnapshot struct {
	p0Bodies [][]Coord
	p1Bodies [][]Coord
	apples   []Coord
}

func findSequenceToSupportCell(layout []string, apples []Coord, p0Bodies, p1Bodies [][]Coord, supportCell Coord, maxDepth int) ([][2]Direction, bool) {
	start := supportSnapshot{
		p0Bodies: cloneBodies(p0Bodies),
		p1Bodies: cloneBodies(p1Bodies),
		apples:   append([]Coord(nil), apples...),
	}

	type node struct {
		snap    supportSnapshot
		path    [][2]Direction
		sawCell bool
	}

	dirs := []Direction{DirNorth, DirEast, DirSouth, DirWest}
	queue := []node{{snap: start}}
	seen := map[string]bool{supportSnapshotKey(start): true}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if len(cur.path) >= maxDepth {
			continue
		}

		for _, d0 := range dirs {
			for _, d1 := range dirs {
				game := newSupportAcceptanceGame(layout, cur.snap.apples, cur.snap.p0Bodies, cur.snap.p1Bodies)
				game.Players[0].birds[0].Direction = d0
				game.Players[0].birds[1].Direction = d1
				for _, bird := range game.Players[1].birds {
					if !bird.Alive || len(bird.Body) == 0 {
						continue
					}
					bird.Direction = DirNorth
					if bird.HeadPos().Y == 0 {
						bird.Direction = DirSouth
					}
				}

				game.PerformGameUpdate(len(cur.path))
				next := supportSnapshotFromGame(game)
				nextSaw := cur.sawCell || bodyContainsCoord(next.p0Bodies[0], supportCell)
				if nextSaw {
					return append(cur.path, [2]Direction{d0, d1}), true
				}

				key := supportSnapshotKey(next)
				if seen[key] {
					continue
				}
				seen[key] = true
				path := append(append([][2]Direction(nil), cur.path...), [2]Direction{d0, d1})
				queue = append(queue, node{snap: next, path: path, sawCell: nextSaw})
			}
		}
	}

	return nil, false
}

func supportSnapshotFromGame(game *Game) supportSnapshot {
	s := supportSnapshot{
		p0Bodies: make([][]Coord, len(game.Players[0].birds)),
		p1Bodies: make([][]Coord, len(game.Players[1].birds)),
		apples:   append([]Coord(nil), game.Grid.Apples...),
	}
	for i, bird := range game.Players[0].birds {
		if len(bird.Body) > 0 {
			s.p0Bodies[i] = append([]Coord(nil), bird.Body...)
		}
	}
	for i, bird := range game.Players[1].birds {
		if len(bird.Body) > 0 {
			s.p1Bodies[i] = append([]Coord(nil), bird.Body...)
		}
	}
	return s
}

func supportSnapshotKey(s supportSnapshot) string {
	var b strings.Builder
	writeBirds := func(birds [][]Coord) {
		for _, body := range birds {
			for _, c := range body {
				fmt.Fprintf(&b, "%d,%d;", c.X, c.Y)
			}
			b.WriteByte('|')
		}
		b.WriteByte('/')
	}
	writeBirds(s.p0Bodies)
	writeBirds(s.p1Bodies)
	for _, a := range s.apples {
		fmt.Fprintf(&b, "%d,%d;", a.X, a.Y)
	}
	return b.String()
}

func cloneBodies(src [][]Coord) [][]Coord {
	out := make([][]Coord, len(src))
	for i, body := range src {
		out[i] = append([]Coord(nil), body...)
	}
	return out
}
