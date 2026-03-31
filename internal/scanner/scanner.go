// Package scanner orchestrates check discovery and execution.
package scanner

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/connection"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// Options configures a scan run.
type Options struct {
	// Host is the database server hostname.
	Host string
	// Port is the database server port.
	Port int
	// DBName is the database name.
	DBName string
	// Categories filters checks by category.
	Categories []string
	// Exclude lists check names to skip.
	Exclude []string
	// IncludeOnly lists check names to run exclusively.
	IncludeOnly []string
	// Mode is the check mode (scan, audit, both).
	Mode string
	// Verbose enables detailed progress output.
	Verbose bool
}

// RunScan executes all discovered checks against the database and returns a ScanReport.
func RunScan(ctx context.Context, conn *pgx.Conn, opts Options) (*models.ScanReport, error) {
	mode := opts.Mode
	if mode == "" {
		mode = "scan"
	}

	modeLabel := "Readiness scan"
	if mode == "audit" {
		modeLabel = "Spock audit"
	}

	pgVersion, err := connection.GetPGVersion(ctx, conn)
	if err != nil {
		return nil, fmt.Errorf("get pg version: %w", err)
	}

	report := &models.ScanReport{
		Database:    opts.DBName,
		Host:        opts.Host,
		Port:        opts.Port,
		Timestamp:   time.Now().UTC(),
		PGVersion:   pgVersion,
		SpockTarget: "5.0",
		ScanMode:    mode,
	}

	checks := check.GetChecks(mode, opts.Categories, opts.Exclude, opts.IncludeOnly)
	total := len(checks)

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "%s: running %d checks against %s...\n", modeLabel, total, opts.DBName)
	}

	for i, c := range checks {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "  [%d/%d] %s/%s: %s\n", i+1, total, c.Category(), c.Name(), c.Description())
		}

		result := models.CheckResult{
			CheckName:   c.Name(),
			Category:    c.Category(),
			Description: c.Description(),
		}

		findings, runErr := c.Run(ctx, conn)
		if runErr != nil {
			result.Error = fmt.Sprintf("%v", runErr)
			if opts.Verbose {
				fmt.Fprintf(os.Stderr, "    ERROR: %s\n", result.Error)
			}
		} else {
			result.Findings = findings
		}

		report.Results = append(report.Results, result)
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Done. %d critical, %d warnings, %d consider, %d info.\n",
			report.CriticalCount(), report.WarningCount(),
			report.ConsiderCount(), report.InfoCount())
	}

	return report, nil
}
