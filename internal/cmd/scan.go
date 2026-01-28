package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/connection"
	"github.com/AntTheLimey/mm-ready/internal/reporter"
	"github.com/AntTheLimey/mm-ready/internal/scanner"
	"github.com/spf13/cobra"
)

var scanConn connFlags
var scanOut outputFlags
var scanCategories string
var scanVerbose bool

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Pre-Spock readiness scan (target: vanilla PostgreSQL)",
	RunE:  runScan,
}

func init() {
	addConnFlags(scanCmd, &scanConn)
	addOutputFlags(scanCmd, &scanOut)
	scanCmd.Flags().StringVar(&scanCategories, "categories", "", "Comma-separated list of check categories to run")
	scanCmd.Flags().BoolVarP(&scanVerbose, "verbose", "v", false, "Print progress")
}

func runScan(cmd *cobra.Command, args []string) error {
	return runMode(scanConn, scanOut, scanCategories, scanVerbose, "scan")
}

func runMode(cf connFlags, of outputFlags, categories string, verbose bool, mode string) error {
	ctx := context.Background()

	conn, err := connection.Connect(ctx, connection.Config{
		Host:     cf.Host,
		Port:     cf.Port,
		DBName:   cf.DBName,
		User:     cf.User,
		Password: cf.Password,
		DSN:      cf.DSN,
	})
	if err != nil {
		return fmt.Errorf("connection failed: %w", err)
	}
	defer conn.Close(ctx)

	var cats []string
	if categories != "" {
		cats = splitComma(categories)
	}

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host:       cf.Host,
		Port:       cf.Port,
		DBName:     cf.DBName,
		Categories: cats,
		Mode:       mode,
		Verbose:    verbose,
	})
	if err != nil {
		return err
	}

	output, err := reporter.Render(report, of.Format)
	if err != nil {
		return err
	}

	return writeOutput(output, of, report.Database)
}

func writeOutput(output string, of outputFlags, dbname string) error {
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

func splitComma(s string) []string {
	var result []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
