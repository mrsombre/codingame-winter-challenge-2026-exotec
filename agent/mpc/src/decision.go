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
	MySnakes []int // indices into g.Sn for my alive snakes
	OpSnakes []int // indices into g.Sn for enemy alive snakes

	// Per-snake scoring: best apple cell and direction after assignment.
	Assigned    []int // apple cell per MySnakes slot (-1 = none)
	AssignedDir []int // first direction per MySnakes slot

	BFS        BFSResult
	Influence  [MaxAp]AppleContest   // per-apple contestation data
	HeatByCell [MaxExpandedCells]int // cell → heat value (opDist - myDist)
}

// Decide runs the full pipeline and prints one line of commands.
func (d *Decision) Decide() {
	d.phaseBFS()
	d.phaseInfluence()
	d.phaseMPC()
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

	var parts []string
	for i, c := range cmds {
		parts = append(parts, c.String())
		if debug && i < len(d.MySnakes) {
			si := i
			if d.Assigned[si] >= 0 && g.IsInGrid(d.Assigned[si]) {
				x, y := g.XY(d.Assigned[si])
				parts = append(parts, fmt.Sprintf("MARK %d %d", x, y))
			}
		}
	}

	fmt.Fprintln(os.Stdout, strings.Join(parts, ";"))
}
