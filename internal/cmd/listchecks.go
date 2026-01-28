package cmd

import (
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/spf13/cobra"
)

var listCategories string
var listMode string

var listChecksCmd = &cobra.Command{
	Use:   "list-checks",
	Short: "List all available checks",
	Run:   runListChecks,
}

func init() {
	listChecksCmd.Flags().StringVar(&listCategories, "categories", "", "Comma-separated list of categories to filter")
	listChecksCmd.Flags().StringVar(&listMode, "mode", "all", "Filter checks by mode (scan, audit, all)")
}

func runListChecks(cmd *cobra.Command, args []string) {
	var cats []string
	if listCategories != "" {
		cats = splitComma(listCategories)
	}

	mode := listMode
	if mode == "all" {
		mode = ""
	}

	checks := check.GetChecks(mode, cats)

	if len(checks) == 0 {
		fmt.Println("No checks found.")
		return
	}

	currentCat := ""
	for _, c := range checks {
		if c.Category() != currentCat {
			currentCat = c.Category()
			fmt.Printf("\n[%s]\n", currentCat)
		}
		modeTag := ""
		if c.Mode() != "scan" {
			modeTag = fmt.Sprintf("[%s]", c.Mode())
		}
		fmt.Printf("  %-30s %-8s %s\n", c.Name(), modeTag, c.Description())
	}
}
