// Check for advisory lock usage — node-local only.
package sql_patterns

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// AdvisoryLocksCheck detects advisory lock usage in pg_stat_statements.
type AdvisoryLocksCheck struct{}

func init() {
	check.Register(AdvisoryLocksCheck{})
}

func (AdvisoryLocksCheck) Name() string        { return "advisory_locks" }
func (AdvisoryLocksCheck) Category() string     { return "sql_patterns" }
func (AdvisoryLocksCheck) Mode() string         { return "scan" }
func (AdvisoryLocksCheck) Description() string {
	return "Advisory lock usage — locks are node-local, not replicated"
}

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
