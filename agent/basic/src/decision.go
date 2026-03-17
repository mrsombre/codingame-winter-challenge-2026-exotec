package main

import (
	"fmt"
	"os"
	"strings"
)

// Command represents a single bot move instruction.
type Command struct {
	ID  int    // snake ID
	Dir int    // direction constant (DU, DR, DD, DL)
	Msg string // debug message, printed if non-empty
}

func (c Command) String() string {
	if c.Msg != "" {
		return fmt.Sprintf("%d %s %s", c.ID, Dn[c.Dir], c.Msg)
	}
	return fmt.Sprintf("%d %s", c.ID, Dn[c.Dir])
}

// Decision is the top-level decision maker.
type Decision struct {
	G *Game
	P *Plan

	// Per-turn pipeline data, recomputed each Decide() call.
	MySnakes []int          // indices into g.Sn for my alive snakes
	BFS      [][]PathResult // BFS results per my snake (indexed same as MySnakes)
	OpSnakes []int          // indices into g.Sn for enemy alive snakes
	OpBFS    [][]PathResult // BFS results per enemy snake

	Influence []int // per-cell Voronoi: positive = my lead in turns, negative = enemy lead

	// Per-apple scoring: Scores[si][j] for my snake si → apple index j.
	// Higher = better target. -1 = unreachable.
	Scores [][]int

	// Per-snake scoring: best apple cell and direction after assignment.
	Assigned    []int // apple cell per MySnakes slot (-1 = none)
	AssignedDir []int // first direction per MySnakes slot

	// SimBFS: physically reachable apple targets per my snake.
	SimTargets [][]SimTarget

	// Body bitmap: true for cells occupied by any alive snake body.
	BodyMap []bool
}

// buildBodyMap builds a bitmap of all cells occupied by alive snake bodies.
func (d *Decision) buildBodyMap() {
	g := d.G
	n := g.NCells
	if len(d.BodyMap) < n {
		d.BodyMap = make([]bool, n)
	} else {
		for i := range d.BodyMap[:n] {
			d.BodyMap[i] = false
		}
	}
	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive {
			continue
		}
		for _, c := range sn.Body {
			if c >= 0 && c < n {
				d.BodyMap[c] = true
			}
		}
	}
}

// ValidDirs returns valid move directions for a snake: not wall, not neck,
// not occupied by any body (except own tail which retracts).
func (d *Decision) ValidDirs(body []int) [4]bool {
	g := d.G
	n := g.NCells
	head := body[0]
	neck := neckOf(body)
	tail := body[len(body)-1]
	hasBodyMap := len(d.BodyMap) >= n
	var ok [4]bool
	for dir := 0; dir < 4; dir++ {
		nc := g.Nbm[head][dir]
		if nc == -1 || nc == neck {
			continue
		}
		if nc >= 0 && nc < n && !g.Cell[nc] {
			continue // wall
		}
		if hasBodyMap && nc >= 0 && nc < n && d.BodyMap[nc] && nc != tail {
			continue // body collision
		}
		ok[dir] = true
	}
	return ok
}

// Decide runs the full pipeline and prints one line of commands.
func (d *Decision) Decide() {
	d.buildBodyMap()
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseScoring()
	d.phaseAssignment()
	d.phaseSafety()
	d.command()
}

// command emits the final move commands to stdout.
func (d *Decision) command() {
	g := d.G

	var cmds []Command
	for si, snIdx := range d.MySnakes {
		sn := &g.Sn[snIdx]
		cmds = append(cmds, Command{ID: sn.ID, Dir: d.AssignedDir[si]})
	}

	if len(cmds) == 0 {
		fmt.Println("WAIT")
		return
	}

	parts := make([]string, len(cmds))
	for i, c := range cmds {
		parts[i] = c.String()
	}

	if debug {
		for si := range d.MySnakes {
			if d.Assigned[si] >= 0 && g.IsInGrid(d.Assigned[si]) {
				x, y := g.XY(d.Assigned[si])
				parts = append(parts, fmt.Sprintf("MARK %d %d", x, y))
			}
		}
	}

	fmt.Fprintln(os.Stdout, strings.Join(parts, ";"))
}
