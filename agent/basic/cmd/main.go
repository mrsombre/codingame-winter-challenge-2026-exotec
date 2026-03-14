package main

import (
	"bufio"
	"fmt"
	"os"

	"codingame/agent/basic/src"
)

const debug = true

func log(a ...interface{}) {
	if debug {
		fmt.Fprintln(os.Stderr, a...)
	}
}

func main() {
	s := bufio.NewScanner(os.Stdin)
	s.Buffer(make([]byte, 1000000), 1000000)

	st := rInit(s)

	// first turn: read + precompute within 1s budget
	rTurn(s, st)
	// TODO: precompute here

	fmt.Println("WAIT")

	for {
		rTurn(s, st)

		fmt.Println("WAIT")
	}
}

func rInit(s *bufio.Scanner) *src.State {
	st := &src.State{}

	// player id
	s.Scan()
	fmt.Sscan(s.Text(), &st.ID)
	log(st.ID)

	// grid
	var w, h int
	s.Scan()
	fmt.Sscan(s.Text(), &w)
	s.Scan()
	fmt.Sscan(s.Text(), &h)
	rows := make([]string, h)
	for i := 0; i < h; i++ {
		s.Scan()
		rows[i] = s.Text()
		log(rows[i])
	}
	st.G = src.NewGrid(w, h, rows)

	// snake ids
	var sp int
	s.Scan()
	fmt.Sscan(s.Text(), &sp)
	st.MyN = sp
	for i := 0; i < sp; i++ {
		s.Scan()
		fmt.Sscan(s.Text(), &st.MyIDs[i])
	}
	st.OppN = sp
	for i := 0; i < sp; i++ {
		s.Scan()
		fmt.Sscan(s.Text(), &st.OppIDs[i])
	}
	log(st.MyIDs[:st.MyN], st.OppIDs[:st.OppN])

	return st
}

func rTurn(s *bufio.Scanner, st *src.State) {
	// apples
	s.Scan()
	fmt.Sscan(s.Text(), &st.AppleN)
	for i := 0; i < st.AppleN; i++ {
		var x, y int
		s.Scan()
		fmt.Sscan(s.Text(), &x, &y)
		st.SetApple(i, x, y)
	}

	// snakes
	s.Scan()
	fmt.Sscan(s.Text(), &st.SnakeN)
	for i := 0; i < st.SnakeN; i++ {
		var id int
		var body string
		s.Scan()
		fmt.Sscan(s.Text(), &id, &body)
		st.SetSnake(i, id, body)
		log(id, body)
	}
}
