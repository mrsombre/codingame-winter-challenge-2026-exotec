package main

import (
	"fmt"
	"os"

	"codingame/internal/match"
)

func main() {
	if err := match.Run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
