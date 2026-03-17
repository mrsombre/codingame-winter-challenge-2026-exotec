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
	g.InitAppleSurfaces()

	type LinkJSON struct {
		To      int          `json:"to"`
		Landing debugCoord   `json:"landing"`
		Len     int          `json:"len"`
		Path    []debugCoord `json:"path"`
	}

	type SurfJSON struct {
		ID    int        `json:"id"`
		Y     int        `json:"y"`
		Left  int        `json:"left"`
		Right int        `json:"right"`
		Len   int        `json:"len"`
		Type  string     `json:"type"`
		Links []LinkJSON `json:"links"`
	}

	type SnakeJSON struct {
		ID    int          `json:"id"`
		Owner int          `json:"owner"`
		Body  []debugCoord `json:"body"`
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

	snakes := make([]SnakeJSON, g.SNum)
	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		body := make([]debugCoord, sn.Len)
		for j := 0; j < sn.Len; j++ {
			body[j] = toCoord(sn.Body[j])
		}
		snakes[i] = SnakeJSON{ID: sn.ID, Owner: sn.Owner, Body: body}
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

	type SurfMapJSON struct {
		W        int        `json:"w"`
		H        int        `json:"h"`
		Walls    []debugCoord `json:"walls"`
		Cells    []CellJSON `json:"cells"`    // all surface cells with their surface ID
		Surfaces []SurfJSON `json:"surfaces"`
		Links    []LinkJSON `json:"links"`    // all links flattened
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
	for _, s := range g.Surfs {
		for _, l := range s.Links {
			path := make([]debugCoord, len(l.Path))
			for k, cell := range l.Path {
				x, y := g.XY(cell)
				path[k] = debugCoord{x, y}
			}
			links = append(links, LinkJSON{From: s.ID, To: l.To, Path: path})
		}
	}

	debugWriteJSON(t, "surfaces.json", SurfMapJSON{
		W:        g.W,
		H:        g.H,
		Walls:    walls,
		Cells:    cells,
		Surfaces: surfs,
		Links:    links,
	})
}
