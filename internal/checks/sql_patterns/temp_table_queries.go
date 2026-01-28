// Check for CREATE TEMP TABLE in SQL patterns.
package sql_patterns

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// TempTableQueriesCheck detects CREATE TEMP TABLE in pg_stat_statements.
type TempTableQueriesCheck struct{}

func init() {
	check.Register(TempTableQueriesCheck{})
}

func (TempTableQueriesCheck) Name() string        { return "temp_table_queries" }
func (TempTableQueriesCheck) Category() string     { return "sql_patterns" }
func (TempTableQueriesCheck) Mode() string         { return "scan" }
func (TempTableQueriesCheck) Description() string {
	return "CREATE TEMP TABLE in SQL — session-local, not replicated"
}

func (c TempTableQueriesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT query, calls
		FROM pg_stat_statements
		WHERE query ~* 'CREATE\s+(TEMP|TEMPORARY)\s+TABLE'
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
			return nil, fmt.Errorf("temp_table_queries scan failed: %w", err)
		}
		matched = append(matched, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("temp_table_queries rows error: %w", err)
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
		Severity:  models.SeverityInfo,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("CREATE TEMP TABLE detected (%d pattern(s))", len(matched)),
		Detail: "Temporary tables are session-local and not replicated. This is " +
			"usually expected behavior, but flagged for awareness.\n\n" +
			"Patterns:\n" + strings.Join(lines, "\n"),
		ObjectName: "(queries)",
	}}, nil
}
