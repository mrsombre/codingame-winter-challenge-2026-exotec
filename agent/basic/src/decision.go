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
	if c.Dir < 0 || c.Dir > 3 {
		return "WAIT"
	}
	if c.Msg != "" {
		return fmt.Sprintf("%d %s %s", c.ID, DirName[c.Dir], c.Msg)
	}
	return fmt.Sprintf("%d %s", c.ID, DirName[c.Dir])
}

// Decision is the top-level decision maker, initialized once with game state.
type Decision struct {
	State *State
	Cmds  []Command
	buf   strings.Builder
}

// Decide computes commands and prints a single semicolon-separated line.
func (d *Decision) Decide() {
	d.Cmds = d.buildCommands()
	if len(d.Cmds) == 0 {
		fmt.Println("WAIT")
		return
	}
	d.buf.Reset()
	for i, c := range d.Cmds {
		if i > 0 {
			d.buf.WriteByte(';')
		}
		d.buf.WriteString(c.String())
	}
	d.buf.WriteByte('\n')
	os.Stdout.WriteString(d.buf.String())
}

func (d *Decision) buildCommands() []Command {
	cmds := make([]Command, 0, d.State.MyN)
	for i := 0; i < d.State.SNum; i++ {
		s := &d.State.Sn[i]
		if s.Owner != 0 || !s.Alive {
			continue
		}
		cmds = append(cmds, Command{s.ID, -1, ""})
	}
	return cmds
}
