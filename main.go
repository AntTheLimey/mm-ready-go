// Package main is the entry point for the mm-ready CLI tool.
package main

import (
	"fmt"
	"os"

	_ "github.com/pgEdge/mm-ready-go/internal/checks"
	"github.com/pgEdge/mm-ready-go/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
