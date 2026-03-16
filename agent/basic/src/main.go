package main

import (
	"bufio"
	"fmt"
	"os"
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

	g := Init(s)
	p := &Plan{g: g}
	p.Precompute()
	bot := &Decision{G: g, P: p}

	// first turn: read + precompute within 1s budget
	g.Read(s)
	bot.Decide()

	for {
		g.Read(s)
		bot.Decide()
	}
}
