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

	var mId int
	s.Scan()
	fmt.Sscan(s.Text(), &mId)
	log(mId)

	// grid
	var w int
	s.Scan()
	fmt.Sscan(s.Text(), &w)
	log(w)
	var h int
	s.Scan()
	fmt.Sscan(s.Text(), &h)
	log(h)
	rows := make([]string, h)
	for i := 0; i < h; i++ {
		s.Scan()
		rows[i] = s.Text()
		log(rows[i])
	}
	_ = src.NewGrid(w, h, rows)

	// snakes
	var sp int
	s.Scan()
	fmt.Sscan(s.Text(), &sp)
	// my
	for i := 0; i < sp; i++ {
		var id int
		s.Scan()
		fmt.Sscan(s.Text(), &id)
	}
	// enemy
	for i := 0; i < sp; i++ {
		var id int
		s.Scan()
		fmt.Sscan(s.Text(), &id)
	}

	for {
		var pc int
		s.Scan()
		fmt.Sscan(s.Text(), &pc)

		for i := 0; i < pc; i++ {
			var x, y int
			s.Scan()
			fmt.Sscan(s.Text(), &x, &y)
			log(x, y)
		}
		var cnt int
		s.Scan()
		fmt.Sscan(s.Text(), &cnt)

		for i := 0; i < cnt; i++ {
			var id int
			var b string
			s.Scan()
			fmt.Sscan(s.Text(), &id, &b)
			log(id, b)
		}

		// fmt.Fprintln(os.Stderr, "Debug messages...")
		fmt.Println("WAIT") // Write action to stdout
	}
}
