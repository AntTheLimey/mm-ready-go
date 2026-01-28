package monitor

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"time"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/connection"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// Options configures a monitor run.
type Options struct {
	Host     string
	Port     int
	DBName   string
	Duration int
	LogFile  string
	Verbose  bool
}

// RunMonitor runs a full scan plus time-based observation.
func RunMonitor(ctx context.Context, conn *pgx.Conn, opts Options) (*models.ScanReport, error) {
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
		ScanMode:    "monitor",
	}

	// Phase 1: standard checks (scan-mode only)
	checks := check.GetChecks("scan", nil)
	total := len(checks)
	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "Phase 1: Running %d standard checks...\n", total)
	}

	for i, c := range checks {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "  [%d/%d] %s/%s\n", i+1, total, c.Category(), c.Name())
		}
		result := models.CheckResult{
			CheckName:   c.Name(),
			Category:    c.Category(),
			Description: c.Description(),
		}
		findings, err := c.Run(ctx, conn)
		if err != nil {
			result.Error = fmt.Sprintf("%v", err)
		} else {
			result.Findings = findings
		}
		report.Results = append(report.Results, result)
	}

	// Phase 2: pg_stat_statements observation
	if IsAvailable(ctx, conn) {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "\nPhase 2: Observing pg_stat_statements for %ds...\n", opts.Duration)
		}
		delta, err := CollectOverDuration(ctx, conn, opts.Duration, opts.Verbose)
		if err != nil {
			report.Results = append(report.Results, models.CheckResult{
				CheckName:   "pgstat_observation",
				Category:    "monitor",
				Description: "pg_stat_statements observation",
				Error:       fmt.Sprintf("%v", err),
			})
		} else {
			report.Results = append(report.Results, buildPgstatResult(delta))
		}
	} else {
		if opts.Verbose {
			fmt.Fprintln(os.Stderr, "\nPhase 2: pg_stat_statements not available, skipping.")
		}
		report.Results = append(report.Results, models.CheckResult{
			CheckName:   "pgstat_observation",
			Category:    "monitor",
			Description: "pg_stat_statements observation",
			Skipped:     true,
			SkipReason:  "pg_stat_statements not available",
		})
	}

	// Phase 3: log file analysis
	if opts.LogFile != "" {
		if opts.Verbose {
			fmt.Fprintf(os.Stderr, "\nPhase 3: Parsing log file: %s\n", opts.LogFile)
		}
		analysis, err := ParseLogFile(opts.LogFile)
		if err != nil {
			report.Results = append(report.Results, models.CheckResult{
				CheckName:   "log_analysis",
				Category:    "monitor",
				Description: "PostgreSQL log file analysis",
				Error:       fmt.Sprintf("%v", err),
			})
		} else {
			report.Results = append(report.Results, buildLogResult(analysis))
		}
	} else if opts.Verbose {
		fmt.Fprintln(os.Stderr, "\nPhase 3: No log file specified, skipping log analysis.")
	}

	if opts.Verbose {
		fmt.Fprintf(os.Stderr, "\nDone. %d critical, %d warnings, %d info.\n",
			report.CriticalCount(), report.WarningCount(), report.InfoCount())
	}

	return report, nil
}

var (
	truncateCascadeRe  = regexp.MustCompile(`(?i)TRUNCATE.*CASCADE`)
	concurrentIndexRe  = regexp.MustCompile(`(?i)CREATE\s+INDEX\s+CONCURRENTLY`)
)

func buildPgstatResult(delta *StatsDelta) models.CheckResult {
	result := models.CheckResult{
		CheckName:   "pgstat_observation",
		Category:    "monitor",
		Description: fmt.Sprintf("SQL activity observed over %.0f seconds", delta.DurationSecs),
	}

	// Report new queries
	if len(delta.NewQueries) > 0 {
		detail := "New queries detected:\n"
		limit := len(delta.NewQueries)
		if limit > 20 {
			limit = 20
		}
		for _, q := range delta.NewQueries[:limit] {
			truncated := q.Query
			if len(truncated) > 150 {
				truncated = truncated[:150]
			}
			detail += fmt.Sprintf("  [%d calls] %s\n", q.Calls, truncated)
		}
		result.Findings = append(result.Findings, models.Finding{
			Severity:   models.SeverityInfo,
			CheckName:  "pgstat_observation",
			Category:   "monitor",
			Title:      fmt.Sprintf("%d new query pattern(s) appeared during observation", len(delta.NewQueries)),
			Detail:     detail,
			ObjectName: "(queries)",
			Metadata:   map[string]any{"new_query_count": len(delta.NewQueries)},
		})
	}

	// Check observed queries for replication-relevant patterns
	limit := len(delta.ChangedQueries)
	if limit > 50 {
		limit = 50
	}
	for _, entry := range delta.ChangedQueries[:limit] {
		truncated := entry.Query
		if len(truncated) > 200 {
			truncated = truncated[:200]
		}

		if truncateCascadeRe.MatchString(entry.Query) {
			result.Findings = append(result.Findings, models.Finding{
				Severity:    models.SeverityWarning,
				CheckName:   "pgstat_observation",
				Category:    "monitor",
				Title:       fmt.Sprintf("TRUNCATE CASCADE observed live (%d calls)", entry.DeltaCalls),
				Detail:      fmt.Sprintf("Query: %s", truncated),
				ObjectName:  "(observed)",
				Remediation: "TRUNCATE CASCADE only applies on provider side.",
			})
		}

		if concurrentIndexRe.MatchString(entry.Query) {
			result.Findings = append(result.Findings, models.Finding{
				Severity:    models.SeverityWarning,
				CheckName:   "pgstat_observation",
				Category:    "monitor",
				Title:       fmt.Sprintf("CREATE INDEX CONCURRENTLY observed live (%d calls)", entry.DeltaCalls),
				Detail:      fmt.Sprintf("Query: %s", truncated),
				ObjectName:  "(observed)",
				Remediation: "Must be done manually on each node.",
			})
		}
	}

	// Summary
	active := len(delta.ChangedQueries)
	result.Findings = append(result.Findings, models.Finding{
		Severity:   models.SeverityInfo,
		CheckName:  "pgstat_observation",
		Category:   "monitor",
		Title:      fmt.Sprintf("Observation summary: %d active query patterns over %.0fs", active, delta.DurationSecs),
		Detail:     fmt.Sprintf("Observed %d query patterns with activity. New patterns: %d.", active, len(delta.NewQueries)),
		ObjectName: "(monitor)",
		Metadata: map[string]any{
			"duration":        delta.DurationSecs,
			"active_patterns": active,
			"new_patterns":    len(delta.NewQueries),
		},
	})

	return result
}

func buildLogResult(analysis *LogAnalysis) models.CheckResult {
	result := models.CheckResult{
		CheckName:   "log_analysis",
		Category:    "monitor",
		Description: "PostgreSQL log file analysis",
	}

	if len(analysis.TruncateCascade) > 0 {
		detail := ""
		limit := len(analysis.TruncateCascade)
		if limit > 10 {
			limit = 10
		}
		for _, s := range analysis.TruncateCascade[:limit] {
			truncated := s.Statement
			if len(truncated) > 150 {
				truncated = truncated[:150]
			}
			detail += fmt.Sprintf("  Line %d: %s\n", s.LineNumber, truncated)
		}
		result.Findings = append(result.Findings, models.Finding{
			Severity:    models.SeverityWarning,
			CheckName:   "log_analysis",
			Category:    "monitor",
			Title:       fmt.Sprintf("TRUNCATE CASCADE in logs (%d occurrences)", len(analysis.TruncateCascade)),
			Detail:      detail,
			ObjectName:  "(log)",
			Remediation: "TRUNCATE CASCADE only applies on provider side.",
		})
	}

	if len(analysis.ConcurrentIndexes) > 0 {
		detail := ""
		limit := len(analysis.ConcurrentIndexes)
		if limit > 10 {
			limit = 10
		}
		for _, s := range analysis.ConcurrentIndexes[:limit] {
			truncated := s.Statement
			if len(truncated) > 150 {
				truncated = truncated[:150]
			}
			detail += fmt.Sprintf("  Line %d: %s\n", s.LineNumber, truncated)
		}
		result.Findings = append(result.Findings, models.Finding{
			Severity:    models.SeverityWarning,
			CheckName:   "log_analysis",
			Category:    "monitor",
			Title:       fmt.Sprintf("CREATE INDEX CONCURRENTLY in logs (%d occurrences)", len(analysis.ConcurrentIndexes)),
			Detail:      detail,
			ObjectName:  "(log)",
			Remediation: "Must be done manually on each node.",
		})
	}

	if len(analysis.DDLStatements) > 0 {
		detail := ""
		limit := len(analysis.DDLStatements)
		if limit > 20 {
			limit = 20
		}
		for _, s := range analysis.DDLStatements[:limit] {
			truncated := s.Statement
			if len(truncated) > 150 {
				truncated = truncated[:150]
			}
			detail += fmt.Sprintf("  Line %d: %s\n", s.LineNumber, truncated)
		}
		result.Findings = append(result.Findings, models.Finding{
			Severity:    models.SeverityInfo,
			CheckName:   "log_analysis",
			Category:    "monitor",
			Title:       fmt.Sprintf("DDL statements in logs (%d occurrences)", len(analysis.DDLStatements)),
			Detail:      detail,
			ObjectName:  "(log)",
			Remediation: "DDL must be coordinated across nodes or use Spock DDL replication.",
		})
	}

	if len(analysis.AdvisoryLocks) > 0 {
		result.Findings = append(result.Findings, models.Finding{
			Severity:   models.SeverityInfo,
			CheckName:  "log_analysis",
			Category:   "monitor",
			Title:      fmt.Sprintf("Advisory locks in logs (%d occurrences)", len(analysis.AdvisoryLocks)),
			Detail:     "Advisory locks are node-local and not replicated.",
			ObjectName: "(log)",
		})
	}

	if len(analysis.CreateTempTable) > 0 {
		result.Findings = append(result.Findings, models.Finding{
			Severity:   models.SeverityInfo,
			CheckName:  "log_analysis",
			Category:   "monitor",
			Title:      fmt.Sprintf("CREATE TEMP TABLE in logs (%d occurrences)", len(analysis.CreateTempTable)),
			Detail:     "Temporary tables are session-local and not replicated.",
			ObjectName: "(log)",
		})
	}

	// Summary
	result.Findings = append(result.Findings, models.Finding{
		Severity:  models.SeverityInfo,
		CheckName: "log_analysis",
		Category:  "monitor",
		Title:     fmt.Sprintf("Log analysis: %d statements parsed", analysis.TotalStatements),
		Detail: fmt.Sprintf(
			"Parsed %d statements from log file.\n"+
				"DDL: %d, TRUNCATE CASCADE: %d, Concurrent indexes: %d, "+
				"Advisory locks: %d, Temp tables: %d",
			analysis.TotalStatements,
			len(analysis.DDLStatements),
			len(analysis.TruncateCascade),
			len(analysis.ConcurrentIndexes),
			len(analysis.AdvisoryLocks),
			len(analysis.CreateTempTable),
		),
		ObjectName: "(log)",
	})

	return result
}
