package main

import (
	"bufio"
	"fmt"
	"os"
)

var debug = false

func log(a ...interface{}) {
	if debug {
		fmt.Fprintln(os.Stderr, a...)
	}
}

func main() {
	s := bufio.NewScanner(os.Stdin)
	s.Buffer(make([]byte, 1_000_000), 1_000_000)

	g := Init(s)
	bot := NewDecision(g)
	for g.Turn(s) {
		bot.Decide()
	}
}
