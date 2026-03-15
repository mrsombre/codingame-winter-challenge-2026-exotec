package main

import (
	"bufio"
	"fmt"
)

const (
	MaxASn = 8   // total snakes both players
	MaxPSn = 4   // max snakes per player
	MaxSeg = 256 // max body parts per snake
)

// Snake holds one snake's current body as flat cell indices, head-first.
type Snake struct {
	ID    int
	Owner int // 0 = mine, 1 = enemy
	Body  [MaxSeg]int
	Len   int
	Alive bool
}

// Head returns the head cell index.
func (s *Snake) Head() int { return s.Body[0] }

// State holds full game state: immutable init data + mutable per-turn data.
type State struct {
	G *Grid // immutable, set once

	// Init data (immutable after init)
	ID    int         // my player id (0 or 1)
	MyIDs [MaxPSn]int // my snake IDs
	MyN   int
	OpIDs [MaxPSn]int // enemy snake IDs
	OpN   int

	// Turn data (overwritten each turn)
	Sn   [MaxASn]Snake
	SNum int           // alive snakes this turn
	Ap   [MaxCells]int // flat cell indices
	ANum int
}

// SFI reads init data from scanner: player id, grid, snake ids.
func SFI(s *bufio.Scanner, st *State) {
	s.Scan()
	fmt.Sscan(s.Text(), &st.ID)
	log(st.ID)

	st.G = GFI(s)

	var sp int
	s.Scan()
	fmt.Sscan(s.Text(), &sp)
	log(sp)
	st.MyN = sp
	for i := 0; i < sp; i++ {
		s.Scan()
		fmt.Sscan(s.Text(), &st.MyIDs[i])
		log(st.MyIDs[i])
	}
	st.OpN = sp
	for i := 0; i < sp; i++ {
		s.Scan()
		fmt.Sscan(s.Text(), &st.OpIDs[i])
		log(st.OpIDs[i])
	}
}

// RT reads per-turn data from scanner: apples and snakes.
func RT(s *bufio.Scanner, st *State) {
	// clear previous turn
	for i := 0; i < st.ANum; i++ {
		st.Ap[i] = 0
	}
	for i := 0; i < st.SNum; i++ {
		st.Sn[i] = Snake{}
	}

	// apples
	s.Scan()
	fmt.Sscan(s.Text(), &st.ANum)
	log(st.ANum)
	for i := 0; i < st.ANum; i++ {
		var x, y int
		s.Scan()
		fmt.Sscan(s.Text(), &x, &y)
		log(x, y)
		st.SetApple(i, x, y)
	}

	// snakes
	s.Scan()
	fmt.Sscan(s.Text(), &st.SNum)
	log(st.SNum)
	for i := 0; i < st.SNum; i++ {
		var id int
		var body string
		s.Scan()
		fmt.Sscan(s.Text(), &id, &body)
		log(id, body)
		st.SetSnake(i, id, body)
	}
}

// IsMyID returns true if id belongs to my snakes.
func (st *State) IsMyID(id int) bool {
	for i := 0; i < st.MyN; i++ {
		if st.MyIDs[i] == id {
			return true
		}
	}
	return false
}

// SetApple stores an apple at position x,y.
func (st *State) SetApple(i, x, y int) {
	st.Ap[i] = st.G.Idx(x, y)
}

// SetSnake parses a body string and stores the snake at slot i.
func (st *State) SetSnake(i, id int, body string) {
	s := &st.Sn[i]
	s.ID = id
	s.Alive = true
	if st.IsMyID(id) {
		s.Owner = 0
	} else {
		s.Owner = 1
	}
	s.Len = ParseBody(body, &s.Body, st.G)
}

// ParseBody parses "x,y:x,y:x,y" into flat indices, returns body length.
func ParseBody(s string, dst *[MaxSeg]int, g *Grid) int {
	n := 0
	i := 0
	for i < len(s) && n < MaxSeg {
		x := 0
		for i < len(s) && s[i] != ',' {
			x = x*10 + int(s[i]-'0')
			i++
		}
		i++ // skip ','
		y := 0
		for i < len(s) && s[i] != ':' {
			y = y*10 + int(s[i]-'0')
			i++
		}
		i++ // skip ':'
		dst[n] = g.Idx(x, y)
		n++
	}
	return n
}
