// Check pg_stat_statements availability for SQL pattern analysis.
package extensions

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// PgStatStatementsCheck checks whether pg_stat_statements is installed and queryable.
type PgStatStatementsCheck struct{}

func init() {
	check.Register(PgStatStatementsCheck{})
}

// Name returns the unique identifier for this check.
func (PgStatStatementsCheck) Name() string     { return "pg_stat_statements_check" }
// Category returns the check category.
func (PgStatStatementsCheck) Category() string { return "extensions" }
// Mode returns when this check runs (scan, audit, or both).
func (PgStatStatementsCheck) Mode() string     { return "scan" }
// Description returns a human-readable summary of this check.
func (PgStatStatementsCheck) Description() string {
	return "pg_stat_statements availability for SQL pattern observation"
}

// Run executes the check against the database connection.
func (c PgStatStatementsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var extversion *string
	err := conn.QueryRow(ctx,
		`SELECT extversion FROM pg_catalog.pg_extension WHERE extname = 'pg_stat_statements';`,
	).Scan(&extversion)

	if err != nil || extversion == nil {
		// Not installed.
		return []models.Finding{{
			Severity:   models.SeverityConsider,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "pg_stat_statements is NOT installed",
			Detail:     "pg_stat_statements is not installed. SQL pattern checks will be limited. Installing it enables richer analysis of executed SQL patterns.",
			ObjectName: "pg_stat_statements",
			Remediation: "Install pg_stat_statements:\n" +
				"1. Add to shared_preload_libraries in postgresql.conf\n" +
				"2. Restart PostgreSQL\n" +
				"3. Run: CREATE EXTENSION pg_stat_statements;",
		}}, nil
	}

	// Installed — check if queryable.
	var stmtCount int64
	err = conn.QueryRow(ctx, "SELECT count(*) FROM pg_stat_statements;").Scan(&stmtCount)
	if err != nil {
		return []models.Finding{{
			Severity:    models.SeverityWarning,
			CheckName:   c.Name(),
			Category:    c.Category(),
			Title:       "pg_stat_statements installed but not queryable",
			Detail:      fmt.Sprintf("pg_stat_statements is installed but could not be queried: %v. Ensure the current user has access to this view.", err),
			ObjectName:  "pg_stat_statements",
			Remediation: "Grant access to pg_stat_statements for the scanning user.",
		}}, nil
	}

	return []models.Finding{{
		Severity:   models.SeverityInfo,
		CheckName:  c.Name(),
		Category:   c.Category(),
		Title:      fmt.Sprintf("pg_stat_statements available (%d statements tracked)", stmtCount),
		Detail:     fmt.Sprintf("pg_stat_statements v%s is installed with %d statements tracked. SQL pattern checks will use this data.", *extversion, stmtCount),
		ObjectName: "pg_stat_statements",
		Metadata:   map[string]any{"version": *extversion, "statement_count": stmtCount},
	}}, nil
}
