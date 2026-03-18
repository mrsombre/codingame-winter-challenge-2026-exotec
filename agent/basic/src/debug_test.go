//go:build debug

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

const debugSeed int64 = 2633570716462326000
const debugLeague = 3

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

func TestPrintMap(t *testing.T) {
	g := testGameFull(debugSeed, int64(debugLeague))
	g.BuildSurfaceGraph()

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
		OnSurface    bool             `json:"onSurface"`
		Apples       []ReachAppleJSON `json:"apples"`
		BestApple    *debugCoord      `json:"bestApple,omitempty"`
		BestDist     int              `json:"bestDist"`
		Conflicting  bool             `json:"conflicting"`
		ConflictWith int              `json:"conflictWith"`
		SurfReaches  []SurfReachJSON  `json:"surfReaches"`
	}

	type SnakeJSON struct {
		ID    int          `json:"id"`
		Owner int          `json:"owner"`
		Body  []debugCoord `json:"body"`
		Dir   string       `json:"dir"`
		Sp    int          `json:"sp"`
		Plan  *PlanJSON    `json:"plan,omitempty"`
	}

	type MapJSON struct {
		W        int         `json:"w"`
		H        int         `json:"h"`
		Walls    []debugCoord `json:"walls"`
		Apples   []debugCoord `json:"apples"`
		Snakes   []SnakeJSON `json:"snakes"`
		Surfaces []SurfJSON  `json:"surfaces"`
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

	apples := make([]debugCoord, g.ANum)
	for i := 0; i < g.ANum; i++ {
		apples[i] = toCoord(g.Ap[i])
	}

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

	// Compute BFS plans
	sim := NewSim(g)
	sim.RebuildAppleMap()

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

		// Compute plan
		head := sn.Body[0]
		onSurf := g.IsInGrid(head) && g.SurfAt[head] >= 0 && sn.Sp == 0

		plan := PlanJSON{
			OnSurface:    onSurf,
			ConflictWith: -1,
		}

		var entries []SurfReach
		if onSurf {
			sid := g.SurfAt[head]
			entries = []SurfReach{{SurfID: sid, Dist: 0, FirstDir: -1, Landing: head}}
		} else {
			entries = sim.SurfBFS(sn)
			plan.SurfReaches = make([]SurfReachJSON, len(entries))
			for k, sr := range entries {
				plan.SurfReaches[k] = toSurfReachJSON(sr)
			}
		}
		reach := surfGraphReach(g, entries, sn.Len, head)
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
			ID:    s.ID,
			Y:     s.Y,
			Left:  s.Left,
			Right: s.Right,
			Len:   s.Len,
			Type:  stype,
			Links: links,
			AppleLinks: appleLinks,
		}
	}

	debugWriteJSON(t, "map.json", MapJSON{
		W:        g.W,
		H:        g.H,
		Walls:    walls,
		Apples:   apples,
		Snakes:   snakes,
		Surfaces: surfs,
	})
}

func TestPrintSurfaces(t *testing.T) {
	g := testGameFull(debugSeed, int64(debugLeague))

	type CellJSON struct {
		X    int `json:"x"`
		Y    int `json:"y"`
		Surf int `json:"surf"` // surface ID or -1
	}

	type SurfJSON struct {
		ID    int `json:"id"`
		Y     int `json:"y"`
		Left  int `json:"left"`
		Right int `json:"right"`
		Len   int `json:"len"`
	}

	type LinkJSON struct {
		From int          `json:"from"` // source surface ID
		To   int          `json:"to"`   // target surface ID
		Path []debugCoord `json:"path"`
	}

	type AppleLinkJSON struct {
		From  int          `json:"from"`  // source surface ID
		Apple debugCoord   `json:"apple"` // target apple coord
		Start debugCoord   `json:"start"` // source surface coord
		Path  []debugCoord `json:"path"`
	}

	type SurfMapJSON struct {
		W          int             `json:"w"`
		H          int             `json:"h"`
		Walls      []debugCoord    `json:"walls"`
		Cells      []CellJSON      `json:"cells"`      // all surface cells with their surface ID
		Surfaces   []SurfJSON      `json:"surfaces"`
		Links      []LinkJSON      `json:"links"`      // all links flattened
		AppleLinks []AppleLinkJSON `json:"appleLinks"` // all apple links flattened
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
