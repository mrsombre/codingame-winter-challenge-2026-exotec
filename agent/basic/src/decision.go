package main

import (
	"fmt"
	"os"
	"strings"
)

// Command represents a single bot move instruction.
type Command struct {
	ID  int    // snake ID
	Dir int    // direction constant (DU, DR, DD, DL) or -1 for WAIT
	Msg string // debug message, printed if non-empty
}

// String formats the command: "<id> <direction> [msg]" or "WAIT".
func (c Command) String() string {
	if c.Msg != "" {
		return fmt.Sprintf("%d %s %s", c.ID, Dn[c.Dir], c.Msg)
	}
	return fmt.Sprintf("%d %s", c.ID, Dn[c.Dir])
}

// Decision is the top-level decision maker, initialized once with game state.
type Decision struct {
	G *Game
	P *Plan
}

// Decide computes commands and prints a single semicolon-separated line.
func (d *Decision) Decide() {
	var cmds []Command

	if len(cmds) == 0 {
		fmt.Println("WAIT")
		return
	}

	// decide here (heuristics)

	cmdo := make([]string, 0, len(cmds))
	for _, c := range cmds {
		if c.Msg != "" {
			cmdo = append(cmdo, fmt.Sprintf("%d %s %s", c.ID, Dn[c.Dir], c.Msg))
		} else {
			cmdo = append(cmdo, fmt.Sprintf("%d %s", c.ID, Dn[c.Dir]))
		}
	}

	cmd := strings.Join(cmdo, ";")
	fmt.Fprintln(os.Stdout, cmd)
}
