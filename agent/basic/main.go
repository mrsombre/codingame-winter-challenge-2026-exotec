package main

import (
	"bufio"
	"fmt"
	"os"
)

var scanner *bufio.Scanner

func readline() string {
	scanner.Scan()
	line := scanner.Text()
	fmt.Fprintln(os.Stderr, line)
	return line
}

func main() {
	scanner = bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1000000), 1000000)

	var myId int
	fmt.Sscan(readline(), &myId)

	var width int
	fmt.Sscan(readline(), &width)

	var height int
	fmt.Sscan(readline(), &height)

	for i := 0; i < height; i++ {
		readline()
	}

	var snakebotsPerPlayer int
	fmt.Sscan(readline(), &snakebotsPerPlayer)

	for i := 0; i < snakebotsPerPlayer; i++ {
		var mySnakebotId int
		fmt.Sscan(readline(), &mySnakebotId)
	}
	for i := 0; i < snakebotsPerPlayer; i++ {
		var oppSnakebotId int
		fmt.Sscan(readline(), &oppSnakebotId)
	}

	for {
		var powerSourceCount int
		fmt.Sscan(readline(), &powerSourceCount)

		for i := 0; i < powerSourceCount; i++ {
			var x, y int
			fmt.Sscan(readline(), &x, &y)
		}

		var snakebotCount int
		fmt.Sscan(readline(), &snakebotCount)

		for i := 0; i < snakebotCount; i++ {
			var snakebotId int
			var body string
			fmt.Sscan(readline(), &snakebotId, &body)
		}

		fmt.Println("WAIT")
	}
}
