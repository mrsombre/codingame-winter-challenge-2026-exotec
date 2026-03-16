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

// --- Phase 1: Gravity-aware BFS ---

func (d *Decision) phaseBFS() {
	g := d.G
	p := d.P

	d.MySnakes = d.MySnakes[:0]
	d.BFS = d.BFS[:0]
	d.OpSnakes = d.OpSnakes[:0]
	d.OpBFS = d.OpBFS[:0]

	for i := 0; i < g.SNum; i++ {
		sn := &g.Sn[i]
		if !sn.Alive || sn.Body[0] < 0 {
			continue
		}
		results := p.BFSFindAll(sn.Body)
		if sn.Owner == 0 {
			d.MySnakes = append(d.MySnakes, i)
			d.BFS = append(d.BFS, results)
		} else {
			d.OpSnakes = append(d.OpSnakes, i)
			d.OpBFS = append(d.OpBFS, results)
		}
	}
}

// --- Phase 2: Influence mapping ---

func (d *Decision) phaseInfluence() {
	g := d.G
	n := g.W * g.H

	if len(d.Influence) < n {
		d.Influence = make([]int, n)
	}

	for c := 0; c < n; c++ {
		myBest := MaxCells
		for _, bfs := range d.BFS {
			if bfs != nil && bfs[c].Dist >= 0 && bfs[c].Dist < myBest {
				myBest = bfs[c].Dist
			}
		}
		opBest := MaxCells
		for _, bfs := range d.OpBFS {
			if bfs != nil && bfs[c].Dist >= 0 && bfs[c].Dist < opBest {
				opBest = bfs[c].Dist
			}
		}
		d.Influence[c] = opBest - myBest
	}
}

// --- Phase 3: Resource scoring ---

func (d *Decision) phaseScoring() {
	// TODO: for each agent-resource pair compute:
	// score = base_value / (1 + bfs_distance) × reachability_gate
	//       × safety_factor × clustering_bonus - opponent_penalty
	// Height bonus for elevated resources reachable only by longer snakes.
	// Clustering bonus for resources near other uncollected resources.
}

// --- Phase 4: Assignment ---

func (d *Decision) phaseAssignment() {
	g := d.G

	n := len(d.MySnakes)
	d.Assigned = make([]int, n)
	d.AssignedDir = make([]int, n)
	for i := range d.Assigned {
		d.Assigned[i] = -1
		d.AssignedDir[i] = DU // fallback
	}

	// Greedy global: pick best (snake, apple) pair each round.
	// Score = BFS distance + penalty for apples the enemy reaches first.
	claimed := make(map[int]bool)
	for round := 0; round < n; round++ {
		bestSI := -1
		bestApple := -1
		bestScore := MaxCells
		bestDir := -1

		for si := 0; si < n; si++ {
			if d.Assigned[si] != -1 {
				continue
			}
			results := d.BFS[si]
			if results == nil {
				continue
			}
			for j := 0; j < g.ANum; j++ {
				ap := g.Ap[j]
				if claimed[ap] {
					continue
				}
				r := results[ap]
				if r.Dist < 0 {
					continue
				}
				score := r.Dist
				if inf := d.Influence[ap]; inf < 0 {
					score -= inf * 2 // penalize: enemy closer by |inf| turns
				}
				if score < bestScore {
					bestSI = si
					bestApple = ap
					bestScore = score
					bestDir = r.FirstDir
				}
			}
		}

		if bestSI == -1 {
			break
		}
		d.Assigned[bestSI] = bestApple
		d.AssignedDir[bestSI] = bestDir
		claimed[bestApple] = true
	}

	// Fallback for unassigned snakes: nearest reachable higher surface.
	for si := 0; si < n; si++ {
		if d.Assigned[si] != -1 {
			continue
		}
		results := d.BFS[si]
		if results == nil {
			continue
		}
		snIdx := d.MySnakes[si]
		head := d.G.Sn[snIdx].Body[0]
		_, hy := g.XY(head)

		bestDist := MaxCells
		for _, surf := range d.P.Surfs {
			if surf.Y >= hy {
				continue // not above head
			}
			// Try left and right edges of the surface as targets
			for _, x := range []int{surf.Left, surf.Right} {
				target := g.Idx(x, surf.Y)
				r := results[target]
				if r.Dist >= 0 && r.Dist < bestDist {
					bestDist = r.Dist
					d.AssignedDir[si] = r.FirstDir
				}
			}
		}
	}
}

// --- Phase 5: Safety check ---

func (d *Decision) phaseSafety() {
	// TODO: for each planned move, verify agent can still reach its own tail
	// (escape route). If not, switch to survival mode: follow the longest
	// path toward the tail.
}

// --- Emit ---

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
