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

	st := &State{}
	SFI(s, st)

	bot := &Decision{State: st}
	// first turn: read + precompute within 1s budget
	RT(s, st)
	// TODO: precompute here
	bot.Decide()

	for {
		RT(s, st)
		bot.Decide()
	}
}
