package main

import "fmt"

func bodyFacing(body []Point) Direction {
	if len(body) < 2 {
		return DirUp
	}
	return FacingPts(body[0], body[1])
}

func actionString(id int, dir Direction, reason string) string {
	if debug && reason != "" {
		return fmt.Sprintf("%d %s %s", id, DirName[dir], reason)
	}
	return fmt.Sprintf("%d %s", id, DirName[dir])
}
