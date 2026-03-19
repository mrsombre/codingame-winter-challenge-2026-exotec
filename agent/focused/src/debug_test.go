//go:build debug

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

var debugOutDir = filepath.Join("..", "..", "..", "debug", "public")

type debugCoord struct {
	X int `json:"x"`
	Y int `json:"y"`
}

func debugWriteJSON(t *testing.T, name string, v any) {
	t.Helper()
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(debugOutDir, name)
	if err := os.WriteFile(outPath, out, 0644); err != nil {
		t.Fatal(err)
	}
	t.Logf("wrote %d bytes to %s", len(out), outPath)
}

// dbgGameDebug builds Game from debug config vars in decision_test.go.
// Uses replay folder or dbgSeed + dbgTurnLines. Builds surface graph.
// Returns game and the actual seed used.
func dbgGameDebug() (*Game, int64) {
	// Resolve seed (replay folder takes priority)
	seed := dbgSeed
	if s, ok := loadReplaySeed(); ok {
		seed = s
	}
	g := dbgGame()
	(&Plan{G: g}).Init()
	return g, seed
}

func TestPrintMap(t *testing.T) {
	g, seed := dbgGameDebug()

	type LinkJSON struct {
		To      int          `json:"to"`
		Landing debugCoord   `json:"landing"`
		Len     int          `json:"len"`
		Path    []debugCoord `json:"path"`
	}

	type AppleLinkJSON struct {
		Apple debugCoord   `json:"apple"`
		Start debugCoord   `json:"start"`
		Len   int          `json:"len"`
		Path  []debugCoord `json:"path"`
	}

	type SurfJSON struct {
		ID         int             `json:"id"`
		Y          int             `json:"y"`
		Left       int             `json:"left"`
		Right      int             `json:"right"`
		Len        int             `json:"len"`
		Type       string          `json:"type"`
		Links      []LinkJSON      `json:"links"`
		AppleLinks []AppleLinkJSON `json:"appleLinks"`
	}

	type SurfReachJSON struct {
		SurfID  int          `json:"surfId"`
		Dist    int          `json:"dist"`
		Landing debugCoord   `json:"landing"`
		Dirs    []string     `json:"dirs"`
		Heads   []debugCoord `json:"heads"`
	}

	type ReachAppleJSON struct {
		Apple debugCoord `json:"apple"`
		Dist  int        `json:"dist"`
		Dir   string     `json:"dir"`
	}

	type PlanJSON struct {
		OnSurface   bool             `json:"onSurface"`
		Apples      []ReachAppleJSON `json:"apples"`
		BestApple   *debugCoord      `json:"bestApple,omitempty"`
		BestDist    int              `json:"bestDist"`
		SurfReaches []SurfReachJSON  `json:"surfReaches"`
	}

	type SnakeJSON struct {
		ID    int          `json:"id"`
		Owner int          `json:"owner"`
		Body  []debugCoord `json:"body"`
		Dir   string       `json:"dir"`
		Sp    int          `json:"sp"`
		Plan  *PlanJSON    `json:"plan,omitempty"`
	}

	type AppleHeatJSON struct {
		X      int `json:"x"`
		Y      int `json:"y"`
		Heat   int `json:"heat"`
		MyDist int `json:"myDist"`
		OpDist int `json:"opDist"`
	}

	type RouteStepJSON struct {
		Dir     string     `json:"dir"`
		ExpHead debugCoord `json:"expHead"`
		Apple   int        `json:"apple"` // -1 = transit
	}

	type RouteJSON struct {
		SnakeID int             `json:"snakeId"`
		Valid   bool            `json:"valid"`
		Apples  []debugCoord    `json:"apples"`
		Steps   []RouteStepJSON `json:"steps"`
	}

	type MapJSON struct {
		Seed     int64           `json:"seed"`
		League   int             `json:"league"`
		W        int             `json:"w"`
		H        int             `json:"h"`
		Walls    []debugCoord    `json:"walls"`
		Apples   []AppleHeatJSON `json:"apples"`
		Snakes   []SnakeJSON     `json:"snakes"`
		Surfaces []SurfJSON      `json:"surfaces"`
		Clusters []struct{}      `json:"clusters"`
		Routes   []RouteJSON     `json:"routes"`
	}

	toCoord := func(cell int) debugCoord {
		x, y := g.XY(cell)
		return debugCoord{x, y}
	}

	var walls []debugCoord
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			if !g.Cell[g.Idx(x, y)] {
				walls = append(walls, debugCoord{x, y})
			}
		}
	}

	// apples built after snake reach is computed (need heat data)

	dirName := func(d int) string {
		if d >= 0 && d < 4 {
			return Dn[d]
		}
		return "?"
	}

	toSurfReachJSON := func(sr SurfReach) SurfReachJSON {
		dirs := make([]string, len(sr.Dirs))
		for k, d := range sr.Dirs {
			dirs[k] = dirName(d)
		}
		heads := make([]debugCoord, len(sr.Heads))
		for k, h := range sr.Heads {
			heads[k] = toCoord(h)
		}
		return SurfReachJSON{
			SurfID:  sr.SurfID,
			Dist:    sr.Dist,
			Landing: toCoord(sr.Landing),
			Dirs:    dirs,
			Heads:   heads,
		}
	}

	// Run real pipeline: phaseBFS + phaseInfluence + phasePartition
	pl := &Plan{G: g}
	pl.Init()
	d := &Decision{G: g, P: pl}
	d.phaseBFS()
	d.phaseInfluence()
	d.phasePartition()

	snakes := make([]SnakeJSON, g.SNum)
	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		body := make([]debugCoord, sn.Len)
		for j := 0; j < sn.Len; j++ {
			body[j] = toCoord(sn.Body[j])
		}

		sj := SnakeJSON{
			ID:    sn.ID,
			Owner: sn.Owner,
			Body:  body,
			Dir:   dirName(sn.Dir),
			Sp:    sn.Sp,
		}

		head := sn.Body[0]
		onSurf := g.IsInGrid(head) && g.SurfAt[head] >= 0 && sn.Sp == 0

		plan := PlanJSON{
			OnSurface: onSurf,
		}

		// Use BFS results from the unified arrays (indexed by snake slot)
		bp := &d.BFS.Plan[i]
		reach := bp.Apples

		if !onSurf {
			surfEntries := d.BFS.SurfBFS[i]
			plan.SurfReaches = make([]SurfReachJSON, len(surfEntries))
			for k, sr := range surfEntries {
				plan.SurfReaches[k] = toSurfReachJSON(sr)
			}
		}

		plan.Apples = make([]ReachAppleJSON, len(reach))
		for k, ri := range reach {
			plan.Apples[k] = ReachAppleJSON{
				Apple: toCoord(ri.Apple),
				Dist:  ri.Dist,
				Dir:   dirName(ri.FirstDir),
			}
		}
		if len(reach) > 0 {
			c := toCoord(reach[0].Apple)
			plan.BestApple = &c
			plan.BestDist = reach[0].Dist
		}

		sj.Plan = &plan
		snakes[i] = sj
	}

	// Build apple heat from pipeline's Influence data
	apples := make([]AppleHeatJSON, g.ANum)
	for i := 0; i < g.ANum; i++ {
		ax, ay := g.XY(g.Ap[i])
		inf := &d.Influence[i]
		apples[i] = AppleHeatJSON{
			X: ax, Y: ay,
			Heat: inf.Heat, MyDist: inf.MyBest, OpDist: inf.OpBest,
		}
	}

	surfs := make([]SurfJSON, len(g.Surfs))
	for i, s := range g.Surfs {
		links := make([]LinkJSON, len(s.Links))
		for j, l := range s.Links {
			path := make([]debugCoord, len(l.Path))
			for k, cell := range l.Path {
				path[k] = toCoord(cell)
			}
			links[j] = LinkJSON{
				To:      l.To,
				Landing: toCoord(l.Landing),
				Len:     l.Len,
				Path:    path,
			}
		}
		appleLinks := make([]AppleLinkJSON, len(s.Apples))
		for j, l := range s.Apples {
			path := make([]debugCoord, len(l.Path))
			for k, cell := range l.Path {
				path[k] = toCoord(cell)
			}
			appleLinks[j] = AppleLinkJSON{
				Apple: toCoord(l.Apple),
				Start: toCoord(l.Start),
				Len:   l.Len,
				Path:  path,
			}
		}
		stype := "solid"
		if s.Type == SurfApple {
			stype = "apple"
		} else if s.Type == SurfNone {
			stype = "none"
		}
		surfs[i] = SurfJSON{
			ID:         s.ID,
			Y:          s.Y,
			Left:       s.Left,
			Right:      s.Right,
			Len:        s.Len,
			Type:       stype,
			Links:      links,
			AppleLinks: appleLinks,
		}
	}

	// Build route data from partition planner
	var routes []RouteJSON
	for si, snIdx := range d.MySnakes {
		route := &pl.Routes[si]
		sn := &g.Sn[snIdx]

		rj := RouteJSON{
			SnakeID: sn.ID,
			Valid:   route.Valid,
		}
		for _, ap := range route.AppleSeq {
			rj.Apples = append(rj.Apples, toCoord(ap))
		}
		for _, step := range route.Steps {
			rj.Steps = append(rj.Steps, RouteStepJSON{
				Dir:     dirName(step.Dir),
				ExpHead: toCoord(step.ExpHead),
				Apple:   step.Apple,
			})
		}
		routes = append(routes, rj)
	}

	debugWriteJSON(t, "map.json", MapJSON{
		Seed:     seed,
		League:   testLeague,
		W:        g.W,
		H:        g.H,
		Walls:    walls,
		Apples:   apples,
		Snakes:   snakes,
		Surfaces: surfs,
		Clusters: nil,
		Routes:   routes,
	})
}

func TestPrintSurfaces(t *testing.T) {
	g, _ := dbgGameDebug()

	type CellJSON struct {
		X    int `json:"x"`
		Y    int `json:"y"`
		Surf int `json:"surf"`
	}

	type SurfJSON struct {
		ID    int `json:"id"`
		Y     int `json:"y"`
		Left  int `json:"left"`
		Right int `json:"right"`
		Len   int `json:"len"`
	}

	type LinkJSON struct {
		From int          `json:"from"`
		To   int          `json:"to"`
		Path []debugCoord `json:"path"`
	}

	type AppleLinkJSON struct {
		From  int          `json:"from"`
		Apple debugCoord   `json:"apple"`
		Start debugCoord   `json:"start"`
		Path  []debugCoord `json:"path"`
	}

	type SurfMapJSON struct {
		W          int             `json:"w"`
		H          int             `json:"h"`
		Walls      []debugCoord    `json:"walls"`
		Cells      []CellJSON      `json:"cells"`
		Surfaces   []SurfJSON      `json:"surfaces"`
		Links      []LinkJSON      `json:"links"`
		AppleLinks []AppleLinkJSON `json:"appleLinks"`
	}

	var walls []debugCoord
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			if !g.Cell[g.Idx(x, y)] {
				walls = append(walls, debugCoord{x, y})
			}
		}
	}

	var cells []CellJSON
	for y := 0; y < g.H; y++ {
		for x := 0; x < g.W; x++ {
			idx := g.Idx(x, y)
			sid := g.SurfAt[idx]
			if sid >= 0 {
				cells = append(cells, CellJSON{X: x, Y: y, Surf: sid})
			}
		}
	}

	surfs := make([]SurfJSON, len(g.Surfs))
	for i, s := range g.Surfs {
		surfs[i] = SurfJSON{ID: s.ID, Y: s.Y, Left: s.Left, Right: s.Right, Len: s.Len}
	}

	var links []LinkJSON
	var appleLinks []AppleLinkJSON
	for _, s := range g.Surfs {
		for _, l := range s.Links {
			path := make([]debugCoord, len(l.Path))
			for k, cell := range l.Path {
				x, y := g.XY(cell)
				path[k] = debugCoord{x, y}
			}
			links = append(links, LinkJSON{From: s.ID, To: l.To, Path: path})
		}
		for _, l := range s.Apples {
			path := make([]debugCoord, len(l.Path))
			for k, cell := range l.Path {
				x, y := g.XY(cell)
				path[k] = debugCoord{x, y}
			}
			ax, ay := g.XY(l.Apple)
			sx, sy := g.XY(l.Start)
			appleLinks = append(appleLinks, AppleLinkJSON{
				From:  s.ID,
				Apple: debugCoord{ax, ay},
				Start: debugCoord{sx, sy},
				Path:  path,
			})
		}
	}

	debugWriteJSON(t, "surfaces.json", SurfMapJSON{
		W:          g.W,
		H:          g.H,
		Walls:      walls,
		Cells:      cells,
		Surfaces:   surfs,
		Links:      links,
		AppleLinks: appleLinks,
	})
}
