package main

import (
	"bufio"
	"fmt"
	"os"
)

var debug = true

func log(a ...interface{}) {
	if debug {
		fmt.Fprintln(os.Stderr, a...)
	}
}

func main() {
	s := bufio.NewScanner(os.Stdin)
	s.Buffer(make([]byte, 1000000), 1000000)

	g := Init(s)
	p := &Plan{G: g}
	bot := &Decision{G: g, P: p}

	g.Turn(s)
	p.Init()
	bot.Decide()

	for {
		g.Turn(s)
		p.UpdateAppleSurfaces()
		bot.Decide()
	}
}
