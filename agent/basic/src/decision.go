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
}

// Decide runs the full pipeline and prints one line of commands.
func (d *Decision) Decide() {
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
	fmt.Fprintln(os.Stdout, strings.Join(parts, ";"))
}
