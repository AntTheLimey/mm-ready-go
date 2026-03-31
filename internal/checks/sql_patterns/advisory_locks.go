// Package sql_patterns contains checks that scan pg_stat_statements for SQL
// patterns relevant to Spock 5 replication (DDL, TRUNCATE CASCADE, etc.).
package sql_patterns

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// AdvisoryLocksCheck detects advisory lock usage in pg_stat_statements.
type AdvisoryLocksCheck struct{}

func init() {
	check.Register(AdvisoryLocksCheck{})
}

// Name returns the unique identifier for this check.
func (AdvisoryLocksCheck) Name() string     { return "advisory_locks" }
// Category returns the check category.
func (AdvisoryLocksCheck) Category() string { return "sql_patterns" }
// Mode returns when this check runs (scan, audit, or both).
func (AdvisoryLocksCheck) Mode() string     { return "scan" }
// Description returns a human-readable summary of this check.
func (AdvisoryLocksCheck) Description() string {
	return "Advisory lock usage — locks are node-local, not replicated"
}

// Run executes the check against the database connection.
func (c AdvisoryLocksCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT query, calls
		FROM pg_stat_statements
		WHERE query ~* 'pg_advisory_lock|pg_try_advisory_lock'
		ORDER BY calls DESC;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		// pg_stat_statements not available — return empty.
		return nil, nil
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var queryText string
		var calls int64
		if err := rows.Scan(&queryText, &calls); err != nil {
			return nil, fmt.Errorf("advisory_locks scan failed: %w", err)
		}

		truncated := queryText
		if len(truncated) > 200 {
			truncated = truncated[:200]
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Advisory lock usage detected (%d call(s))", calls),
			Detail: fmt.Sprintf(
				"Query: %s\n\n"+
					"Advisory locks are node-local in PostgreSQL. They are not replicated "+
					"and provide no cross-node coordination. If your application uses advisory "+
					"locks for mutual exclusion, this will not work across a multi-master cluster.",
				truncated,
			),
			ObjectName: "(query)",
			Remediation: "If advisory locks are used for application-level coordination, " +
				"implement a distributed locking mechanism instead.",
			Metadata: map[string]any{"calls": calls},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("advisory_locks rows error: %w", err)
	}

	return findings, nil
}
