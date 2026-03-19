package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

// ----- Global debug config -----
// These are FALLBACKS. If replay/seed.txt and replay/replay.txt exist, they take priority.

var dbgSeed int64 = -721452932309119600

// dbgTurnLines: paste stderr turn input here. nil = use engine turn 0.
var dbgTurnLines []string = nil

// dbgSnakeID: which snake to trace in BFS detail. -1 = all my snakes.
var dbgSnakeID = 0

// ----- Replay folder loader -----

var replayDir = filepath.Join("..", "..", "..", "replay")

// replayDataLine matches lines that are turn data: start with digit(s).
var replayDataLine = regexp.MustCompile(`^\d`)

func loadReplaySeed() (int64, bool) {
	data, err := os.ReadFile(filepath.Join(replayDir, "seed.txt"))
	if err != nil {
		return 0, false
	}
	s := strings.TrimSpace(string(data))
	// Parse "seed=<number>" or just "<number>"
	s = strings.TrimPrefix(s, "seed=")
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func loadReplayTurn() ([]string, bool) {
	data, err := os.ReadFile(filepath.Join(replayDir, "replay.txt"))
	if err != nil {
		return nil, false
	}
	raw := strings.TrimSpace(string(data))
	if raw == "" {
		return nil, false
	}
	// Filter: keep only lines starting with a digit (turn data)
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if replayDataLine.MatchString(line) {
			lines = append(lines, line)
		}
	}
	if len(lines) == 0 {
		return nil, false
	}
	return lines, true
}

// ----- Game builder -----

func dbgGame() *Game {
	// Try replay folder first
	seed := dbgSeed
	if s, ok := loadReplaySeed(); ok {
		seed = s
	}

	turnLines := dbgTurnLines
	if tl, ok := loadReplayTurn(); ok {
		turnLines = tl
	}

	if turnLines != nil {
		g, _ := testGameWithTurn(turnLines, seed, int64(testLeague))
		return g
	}
	return testGameFull(seed, int64(testLeague))
}

// ----- Console trace: full pipeline -----

func TestDbgPipeline(t *testing.T) {
	g := dbgGame()
	p := &Plan{G: g}
	p.Init()
	t.Logf("Grid: %dx%d  Apples: %d  Snakes: %d", g.W, g.H, g.ANum, g.SNum)
	d := &Decision{G: g, P: p}

	d.phaseBFS()
	t.Log("--- phaseBFS ---")
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		hx, hy := g.XY(sn.Body[0])
		t.Logf("[%d] snake %d  head=(%d,%d) dir=%s sp=%d len=%d",
			si, sn.ID, hx, hy, Dn[sn.Dir], sn.Sp, sn.Len)
		plan := &d.BFS.Plan[snIdx]
		if plan.BestApple >= 0 {
			ax, ay := g.XY(plan.BestApple)
			t.Logf("    target=(%d,%d) dist=%d firstDir=%s conflict=%v",
				ax, ay, plan.BestDist, safeDir(plan.TotalFirst), plan.Conflicting)
		} else {
			t.Logf("    target=none")
		}
		for _, ri := range d.BFS.Reach[snIdx] {
			ax, ay := g.XY(ri.Apple)
			t.Logf("    reach (%d,%d) d=%d dir=%s surf=%d", ax, ay, ri.Dist, safeDir(ri.FirstDir), ri.EndSurf)
		}
		t.Logf("    → dir=%s apple=%s", safeDir(d.AssignedDir[si]), cellStr(g, d.Assigned[si]))
	}

	d.phaseInfluence()
	d.phasePartition()
	t.Log("--- phasePartition ---")
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		route := &d.P.Routes[si]
		t.Logf("[%d] snake %d  dir=%s target=%s valid=%v apples=%d",
			si, sn.ID, safeDir(d.AssignedDir[si]), cellStr(g, d.Assigned[si]),
			route.Valid, len(route.AppleSeq))
		for ai, ap := range route.AppleSeq {
			ax, ay := g.XY(ap)
			t.Logf("    route[%d] = (%d,%d)", ai, ax, ay)
		}
	}

	d.phaseSafety()
	t.Log("--- FINAL ---")
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		t.Logf("[%d] snake %d  dir=%s target=%s", si, sn.ID, safeDir(d.AssignedDir[si]), cellStr(g, d.Assigned[si]))
	}
}

// ----- Console trace: BFS detail for one snake -----

func TestDbgBFS(t *testing.T) {
	g := dbgGame()
	p := &Plan{G: g}
	p.Init()
	t.Logf("Grid: %dx%d  Apples: %d  Snakes: %d", g.W, g.H, g.ANum, g.SNum)

	var sn *Snake
	for i := 0; i < g.SNum; i++ {
		if g.Sn[i].ID == dbgSnakeID && g.Sn[i].Alive {
			sn = &g.Sn[i]
			break
		}
	}
	if sn == nil {
		t.Fatalf("Snake %d not found or dead", dbgSnakeID)
	}

	hx, hy := g.XY(sn.Body[0])
	t.Logf("Snake %d: head=(%d,%d) dir=%s sp=%d len=%d", sn.ID, hx, hy, Dn[sn.Dir], sn.Sp, sn.Len)
	for bi, c := range sn.Body {
		bx, by := g.XY(c)
		t.Logf("  body[%d] = (%d,%d)", bi, bx, by)
	}

	head := sn.Body[0]
	onSurface := g.IsInGrid(head) && g.SurfAt[head] >= 0 && sn.Sp == 0
	t.Logf("  onSurface=%v headSurf=%d", onSurface, g.SurfAt[head])

	// Nearby grid
	t.Log("--- Nearby grid ---")
	for dy := -2; dy <= 3; dy++ {
		for dx := -3; dx <= 3; dx++ {
			cx, cy := hx+dx, hy+dy
			c := g.Idx(cx, cy)
			if c < 0 {
				continue
			}
			wall := !g.Cell[c]
			sid := g.SurfAt[c]
			stype := ""
			if sid >= 0 {
				switch g.Surfs[sid].Type {
				case SurfSolid:
					stype = "S"
				case SurfApple:
					stype = "A"
				}
			}
			apple := ""
			for a := 0; a < g.ANum; a++ {
				if g.Ap[a] == c {
					apple = "*"
				}
			}
			if wall || sid >= 0 || apple != "" {
				tag := ""
				if wall {
					tag = "W"
				}
				t.Logf("  (%d,%d) %s%s%s surf=%d", cx, cy, tag, stype, apple, sid)
			}
		}
	}

	// Layer 1: SurfBFS
	sim := NewSim(g)
	sim.RebuildAppleMap()
	surfEntries := sim.SurfBFS(sn)
	t.Log("--- Layer 1: SurfBFS ---")
	for _, sr := range surfEntries {
		lx, ly := g.XY(sr.Landing)
		dirs := make([]string, len(sr.Dirs))
		for k, dd := range sr.Dirs {
			dirs[k] = Dn[dd]
		}
		s := &g.Surfs[sr.SurfID]
		stype := "solid"
		if s.Type == SurfApple {
			stype = "APPLE"
		}
		t.Logf("  surf %d (%s) y=%d x=[%d..%d] land=(%d,%d) d=%d dir=%s path=%v",
			sr.SurfID, stype, s.Y, s.Left, s.Right, lx, ly, sr.Dist, safeDir(sr.FirstDir), dirs)
		for _, al := range s.Apples {
			ax, ay := g.XY(al.Apple)
			sx, sy := g.XY(al.Start)
			t.Logf("      → apple (%d,%d) from (%d,%d) len=%d", ax, ay, sx, sy, al.Len)
		}
		for _, sl := range s.Links {
			tlx, tly := g.XY(sl.Landing)
			ts := &g.Surfs[sl.To]
			tt := "solid"
			if ts.Type == SurfApple {
				tt = "APPLE"
			}
			t.Logf("      → surf %d (%s) land=(%d,%d) len=%d", sl.To, tt, tlx, tly, sl.Len)
		}
	}

	// Layer 2: surfGraphReach
	reach := surfGraphReach(g, surfEntries, sn.Len, head)
	t.Log("--- Layer 2: surfGraphReach ---")
	for _, ri := range reach {
		ax, ay := g.XY(ri.Apple)
		t.Logf("  apple (%d,%d) dist=%d dir=%s endSurf=%d", ax, ay, ri.Dist, safeDir(ri.FirstDir), ri.EndSurf)
	}

	if len(surfEntries) > 0 {
		t.Logf("→ TotalFirst = %s (from surfBFS[0])", safeDir(surfEntries[0].FirstDir))
	}
	if len(reach) > 0 {
		ax, ay := g.XY(reach[0].Apple)
		t.Logf("→ BestApple = (%d,%d) dist=%d dir=%s", ax, ay, reach[0].Dist, safeDir(reach[0].FirstDir))
	}
}

// ----- Helpers -----

func safeDir(d int) string {
	if d >= 0 && d < 4 {
		return Dn[d]
	}
	return "?"
}

func cellStr(g *Game, cell int) string {
	if cell < 0 || !g.IsInGrid(cell) {
		return "none"
	}
	x, y := g.XY(cell)
	return fmt.Sprintf("(%d,%d)", x, y)
}

// ----- Regression tests -----

func TestDebugSeed1074_Crash(t *testing.T) {
	turnLines := []string{
		"11",
		"6 0", "11 4", "5 7",
		"3 0", "27 0", "16 1", "21 1", "11 2", "26 2",
		"3 4", "1 5",
		"7",
		"0 13,4:14,4:15,4:15,5:15,6:15,7:15,8:15,9:14,9:13,9",
		"1 37,7:37,8:37,9",
		"2 26,9:26,10:27,10:27,11:27,12:27,13",
		"3 36,7:36,8:36,9",
		"5 31,13:30,13:30,14:29,14:28,14:27,14",
		"6 40,13:39,13:38,13:37,13:36,13",
		"7 38,6:37,6:36,6:35,6:34,6:34,7",
	}
	g, p := testGameWithTurn(turnLines, 1074984748969286100, 3)
	t.Logf("Grid: %dx%d", g.W, g.H)
	d := &Decision{G: g, P: p}
	d.Decide() // must not panic
	t.Log("OK — no crash")
}
