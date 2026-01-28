// Check for CREATE INDEX CONCURRENTLY usage — must be done manually per node.
package sql_patterns

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// ConcurrentIndexesCheck detects CREATE INDEX CONCURRENTLY in pg_stat_statements.
type ConcurrentIndexesCheck struct{}

func init() {
	check.Register(ConcurrentIndexesCheck{})
}

func (ConcurrentIndexesCheck) Name() string        { return "concurrent_indexes" }
func (ConcurrentIndexesCheck) Category() string     { return "sql_patterns" }
func (ConcurrentIndexesCheck) Mode() string         { return "scan" }
func (ConcurrentIndexesCheck) Description() string {
	return "CREATE INDEX CONCURRENTLY — must be created manually on each node"
}

func (c ConcurrentIndexesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT query, calls
		FROM pg_stat_statements
		WHERE query ~* 'CREATE\s+INDEX\s+CONCURRENTLY'
		ORDER BY calls DESC;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		// pg_stat_statements not available — return empty.
		return nil, nil
	}
	defer rows.Close()

	type row struct {
		query string
		calls int64
	}
	var matched []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.query, &r.calls); err != nil {
			return nil, fmt.Errorf("concurrent_indexes scan failed: %w", err)
		}
		matched = append(matched, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("concurrent_indexes rows error: %w", err)
	}

	if len(matched) == 0 {
		return nil, nil
	}

	// Build pattern list (max 10).
	limit := len(matched)
	if limit > 10 {
		limit = 10
	}
	var lines []string
	for _, r := range matched[:limit] {
		truncated := r.query
		if len(truncated) > 150 {
			truncated = truncated[:150]
		}
		lines = append(lines, fmt.Sprintf("  [%d calls] %s", r.calls, truncated))
	}

	return []models.Finding{{
		Severity:  models.SeverityWarning,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("CREATE INDEX CONCURRENTLY detected (%d pattern(s))", len(matched)),
		Detail: "CREATE INDEX CONCURRENTLY statements were found in SQL history. " +
			"Concurrent indexes must be created by hand on each node in a " +
			"Spock cluster — they cannot be replicated via DDL replication.\n\n" +
			"Patterns found:\n" + strings.Join(lines, "\n"),
		ObjectName: "(queries)",
		Remediation: "Plan to execute CREATE INDEX CONCURRENTLY manually on each node. " +
			"Do not rely on DDL replication for these operations.",
		Metadata: map[string]any{"pattern_count": len(matched)},
	}}, nil
}
