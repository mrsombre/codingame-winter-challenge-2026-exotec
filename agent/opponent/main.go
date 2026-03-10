package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

const debug = false

var scanner *bufio.Scanner

func readline() string {
	scanner.Scan()
	line := scanner.Text()
	if debug {
		fmt.Fprintln(os.Stderr, line)
	}
	return line
}

const (
	DirUp    = "UP"
	DirDown  = "DOWN"
	DirLeft  = "LEFT"
	DirRight = "RIGHT"
)

type Point struct{ x, y int }

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}

func dist(a, b Point) int {
	return abs(a.x-b.x) + abs(a.y-b.y)
}

func parseBody(s string) []Point {
	parts := strings.Split(s, ":")
	pts := make([]Point, len(parts))
	for i, p := range parts {
		fmt.Sscanf(p, "%d,%d", &pts[i].x, &pts[i].y)
	}
	return pts
}

func main() {
	scanner = bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1000000), 1000000)

	var myId int
	fmt.Sscan(readline(), &myId)

	var width, height int
	fmt.Sscan(readline(), &width)
	fmt.Sscan(readline(), &height)

	for i := 0; i < height; i++ {
		readline()
	}

	var snakebotsPerPlayer int
	fmt.Sscan(readline(), &snakebotsPerPlayer)

	myBots := make(map[int]bool)
	for i := 0; i < snakebotsPerPlayer; i++ {
		var id int
		fmt.Sscan(readline(), &id)
		myBots[id] = true
	}
	for i := 0; i < snakebotsPerPlayer; i++ {
		readline()
	}

	for {
		var powerSourceCount int
		fmt.Sscan(readline(), &powerSourceCount)

		sources := make([]Point, powerSourceCount)
		for i := 0; i < powerSourceCount; i++ {
			fmt.Sscan(readline(), &sources[i].x, &sources[i].y)
		}

		var snakebotCount int
		fmt.Sscan(readline(), &snakebotCount)

		type Bot struct {
			id   int
			head Point
		}
		var mine []Bot

		for i := 0; i < snakebotCount; i++ {
			var id int
			var body string
			fmt.Sscan(readline(), &id, &body)
			if myBots[id] {
				pts := parseBody(body)
				mine = append(mine, Bot{id, pts[0]})
			}
		}

		var actions []string
		for _, bot := range mine {
			if len(sources) == 0 {
				actions = append(actions, fmt.Sprintf("%d %s", bot.id, DirUp))
				continue
			}
			// find nearest power source
			best := sources[0]
			bestD := dist(bot.head, best)
			for _, s := range sources[1:] {
				if d := dist(bot.head, s); d < bestD {
					best = s
					bestD = d
				}
			}
			dx := best.x - bot.head.x
			dy := best.y - bot.head.y
			var dir string
			if abs(dx) >= abs(dy) {
				if dx > 0 {
					dir = DirRight
				} else {
					dir = DirLeft
				}
			} else {
				if dy > 0 {
					dir = DirDown
				} else {
					dir = DirUp
				}
			}
			actions = append(actions, fmt.Sprintf("%d %s", bot.id, dir))
		}

		var output string
		if len(actions) == 0 {
			output = "WAIT"
		} else {
			output = strings.Join(actions, ";")
		}
		if debug {
			fmt.Fprintln(os.Stderr, output)
		}
		fmt.Println(output)
	}
}
