package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/analyzer"
	"github.com/AntTheLimey/mm-ready/internal/parser"
	"github.com/AntTheLimey/mm-ready/internal/reporter"
	"github.com/spf13/cobra"
)

var analyzeFile string
var analyzeOut outputFlags
var analyzeCategories string
var analyzeVerbose bool

var analyzeCmd = &cobra.Command{
	Use:   "analyze",
	Short: "Analyze a pg_dump schema file offline",
	Long:  "Parse a pg_dump --schema-only SQL file and run schema-structural checks without a database connection.",
	RunE:  runAnalyze,
}

func init() {
	analyzeCmd.Flags().StringVar(&analyzeFile, "file", "", "Path to pg_dump SQL file (required)")
	analyzeCmd.MarkFlagRequired("file")
	addOutputFlags(analyzeCmd, &analyzeOut)
	analyzeCmd.Flags().StringVar(&analyzeCategories, "categories", "", "Comma-separated list of check categories to run")
	analyzeCmd.Flags().BoolVarP(&analyzeVerbose, "verbose", "v", false, "Print progress")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// Check if file exists
	if _, err := os.Stat(analyzeFile); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", analyzeFile)
	}

	// Parse the dump file
	schema, err := parser.ParseDump(analyzeFile)
	if err != nil {
		return fmt.Errorf("parse dump: %w", err)
	}

	// Parse categories
	var cats []string
	if analyzeCategories != "" {
		for _, c := range strings.Split(analyzeCategories, ",") {
			c = strings.TrimSpace(c)
			if c != "" {
				cats = append(cats, c)
			}
		}
	}

	// Run analysis
	report, err := analyzer.RunAnalyze(schema, analyzeFile, cats, analyzeVerbose)
	if err != nil {
		return fmt.Errorf("analyze: %w", err)
	}

	// Render report
	output, err := reporter.Render(report, analyzeOut.Format)
	if err != nil {
		return fmt.Errorf("render report: %w", err)
	}

	// Write output
	return writeAnalyzeOutput(output, analyzeOut, report.Database)
}

func writeAnalyzeOutput(output string, of outputFlags, dbname string) error {
	var path string
	if of.Output != "" {
		path = MakeOutputPath(of.Output, of.Format, dbname)
	} else {
		path = MakeDefaultOutputPath(of.Format, dbname)
	}

	dir := filepath.Dir(path)
	if dir != "" {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	if err := os.WriteFile(path, []byte(output), 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	fmt.Fprintf(os.Stderr, "Report written to %s\n", path)
	return nil
}
