package main

import (
	"fmt"
	"os"

	_ "github.com/AntTheLimey/mm-ready/internal/checks"
	"github.com/AntTheLimey/mm-ready/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
