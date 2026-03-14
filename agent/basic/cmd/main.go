package main

import (
	"bufio"
	"fmt"
	"os"
)

const debug = true

var scanner *bufio.Scanner

func readline() string {
	if !scanner.Scan() {
		os.Exit(0)
	}
	line := scanner.Text()
	if debug {
		fmt.Fprintln(os.Stderr, line)
	}
	return line
}

func main() {
	scanner = bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	// players count
	readline()
	// width
	var W int
	fmt.Sscan(readline(), &W)
	// height
	var H int
	fmt.Sscan(readline(), &H)

	// grid rows
	for y := 0; y < H; y++ {
		readline()
	}

	// bots per player
	var botsPerPlayer int
	fmt.Sscan(readline(), &botsPerPlayer)
	// my bot ids
	for i := 0; i < botsPerPlayer; i++ {
		readline()
	}
	// opponent bot ids
	for i := 0; i < botsPerPlayer; i++ {
		readline()
	}

	for {
		// sources
		var srcN int
		fmt.Sscan(readline(), &srcN)
		for i := 0; i < srcN; i++ {
			readline()
		}

		// bots
		var botN int
		fmt.Sscan(readline(), &botN)
		for i := 0; i < botN; i++ {
			readline()
		}

		fmt.Println("WAIT")
	}
}
