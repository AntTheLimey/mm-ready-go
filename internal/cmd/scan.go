package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pgEdge/mm-ready-go/internal/config"
	"github.com/pgEdge/mm-ready-go/internal/connection"
	"github.com/pgEdge/mm-ready-go/internal/reporter"
	"github.com/pgEdge/mm-ready-go/internal/scanner"
	"github.com/spf13/cobra"
)

var scanConn connFlags
var scanOut outputFlags
var scanCategories string
var scanExclude string
var scanIncludeOnly string
var scanVerbose bool

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Pre-Spock readiness scan (target: vanilla PostgreSQL)",
	RunE:  runScan,
}

func init() {
	addConnFlags(scanCmd, &scanConn)
	addOutputFlags(scanCmd, &scanOut)
	addConfigFlags(scanCmd)
	addReportFlags(scanCmd)
	scanCmd.Flags().StringVar(&scanCategories, "categories", "", "Comma-separated list of check categories to run")
	scanCmd.Flags().StringVar(&scanExclude, "exclude", "", "Comma-separated list of check names to skip")
	scanCmd.Flags().StringVar(&scanIncludeOnly, "include-only", "", "Comma-separated list of check names to run (whitelist)")
	scanCmd.Flags().BoolVarP(&scanVerbose, "verbose", "v", false, "Print progress")
}

func runScan(cmd *cobra.Command, args []string) error {
	return runMode(scanConn, scanOut, scanCategories, scanExclude, scanIncludeOnly, scanVerbose, "scan")
}

func runMode(cf connFlags, of outputFlags, categories string, exclude string, includeOnly string, verbose bool, mode string) error {
	ctx := context.Background()

	// Load config
	var cfg config.Config
	if !noConfig {
		if configPath != "" {
			var cfgErr error
			cfg, cfgErr = config.LoadFile(configPath)
			if cfgErr != nil {
				return cfgErr
			}
		} else {
			path := config.DiscoverConfigFile()
			if path != "" {
				var cfgErr error
				cfg, cfgErr = config.LoadFile(path)
				if cfgErr != nil {
					return cfgErr
				}
				if verbose {
					fmt.Fprintf(os.Stderr, "Using config file: %s\n", path)
				}
			} else {
				cfg = config.Default()
			}
		}
	} else {
		cfg = config.Default()
	}

	checkCfg, reportCfg := config.MergeCLI(cfg, mode, splitComma(exclude), splitComma(includeOnly), noTodo, todoIncludeConsider)

	conn, err := connection.Connect(ctx, connection.Config{
		Host:        cf.Host,
		Port:        cf.Port,
		DBName:      cf.DBName,
		User:        cf.User,
		Password:    cf.Password,
		DSN:         cf.DSN,
		SSLMode:     cf.SSLMode,
		SSLCert:     cf.SSLCert,
		SSLKey:      cf.SSLKey,
		SSLRootCert: cf.SSLRootCert,
	})
	if err != nil {
		return formatConnError(err, cf)
	}
	defer conn.Close(ctx)

	var cats []string
	if categories != "" {
		cats = splitComma(categories)
	}

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host:        cf.Host,
		Port:        cf.Port,
		DBName:      cf.DBName,
		Categories:  cats,
		Exclude:     checkCfg.Exclude,
		IncludeOnly: checkCfg.IncludeOnly,
		Mode:        mode,
		Verbose:     verbose,
	})
	if err != nil {
		return err
	}

	reportOpts := reporter.ReportOptions{
		TodoList:            reportCfg.TodoList,
		TodoIncludeConsider: reportCfg.TodoIncludeConsider,
	}
	output, err := reporter.Render(report, of.Format, reportOpts)
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

// formatConnError returns a user-friendly error message for connection failures.
func formatConnError(err error, cf connFlags) error {
	errMsg := err.Error()
	var hint string

	switch {
	case strings.Contains(errMsg, "no password supplied") || strings.Contains(errMsg, "password authentication failed"):
		hint = "Hint: Use --password to provide a password, or set PGPASSWORD environment variable."
	case strings.Contains(errMsg, "does not exist"):
		hint = "Hint: Check that the database name is correct."
	case strings.Contains(strings.ToLower(errMsg), "connection refused") || strings.Contains(strings.ToLower(errMsg), "could not connect"):
		host := cf.Host
		if host == "" {
			host = "localhost"
		}
		port := cf.Port
		if port == 0 {
			port = 5432
		}
		hint = fmt.Sprintf("Hint: Check that PostgreSQL is running on %s:%d.", host, port)
	}

	if hint != "" {
		return fmt.Errorf("could not connect to database.\n       %s\n\n%s", errMsg, hint)
	}
	return fmt.Errorf("could not connect to database: %s", errMsg)
}
