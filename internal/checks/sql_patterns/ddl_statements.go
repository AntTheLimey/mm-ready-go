// Detect DDL statements in tracked SQL for replication awareness.
package sql_patterns

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// DdlStatementsCheck detects DDL statements in pg_stat_statements.
type DdlStatementsCheck struct{}

func init() {
	check.Register(DdlStatementsCheck{})
}

func (DdlStatementsCheck) Name() string        { return "ddl_statements" }
func (DdlStatementsCheck) Category() string     { return "sql_patterns" }
func (DdlStatementsCheck) Mode() string         { return "scan" }
func (DdlStatementsCheck) Description() string {
	return "DDL statements — must use Spock DDL replication or manual coordination"
}

var ddlPatterns = []string{
	"CREATE TABLE", "ALTER TABLE", "DROP TABLE",
	"CREATE INDEX", "DROP INDEX",
	"CREATE VIEW", "DROP VIEW", "ALTER VIEW",
	"CREATE FUNCTION", "DROP FUNCTION", "ALTER FUNCTION",
	"CREATE PROCEDURE", "DROP PROCEDURE", "ALTER PROCEDURE",
	"CREATE TRIGGER", "DROP TRIGGER",
	"CREATE TYPE", "DROP TYPE", "ALTER TYPE",
	"CREATE SCHEMA", "DROP SCHEMA",
	"CREATE SEQUENCE", "ALTER SEQUENCE", "DROP SEQUENCE",
}

func (c DdlStatementsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	pattern := strings.Join(ddlPatterns, "|")

	rows, err := conn.Query(ctx, `
		SELECT query, calls
		FROM pg_stat_statements
		WHERE query ~* $1
		ORDER BY calls DESC
		LIMIT 50;
	`, pattern)
	if err != nil {
		// pg_stat_statements not available.
		return []models.Finding{{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Cannot check DDL patterns — pg_stat_statements unavailable",
			Detail:     "pg_stat_statements is not available.",
			ObjectName: "pg_stat_statements",
		}}, nil
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
			return nil, fmt.Errorf("ddl_statements scan failed: %w", err)
		}
		matched = append(matched, r)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("ddl_statements rows error: %w", err)
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
		if len(truncated) > 120 {
			truncated = truncated[:120]
		}
		lines = append(lines, fmt.Sprintf("  [%d calls] %s", r.calls, truncated))
	}

	return []models.Finding{{
		Severity:  models.SeverityConsider,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("Found %d DDL statement pattern(s) in pg_stat_statements", len(matched)),
		Detail: "DDL statements are not automatically replicated by default. " +
			"Spock's AutoDDL feature (spock.enable_ddl_replication=on) can " +
			"automatically replicate DDL classified as LOGSTMT_DDL by PostgreSQL.\n\n" +
			"AutoDDL does NOT replicate:\n" +
			"  - TRUNCATE (classified as LOGSTMT_MISC, replicated via replication sets)\n" +
			"  - VACUUM, ANALYZE (classified as LOGSTMT_ALL, must run on each node)\n\n" +
			"AutoDDL DOES replicate (when enabled):\n" +
			"  - CREATE/ALTER/DROP TABLE, INDEX, VIEW, FUNCTION, SEQUENCE, etc.\n" +
			"  - CLUSTER, REINDEX (classified as LOGSTMT_DDL)\n\n" +
			"Top DDL patterns:\n" + strings.Join(lines, "\n"),
		ObjectName: "(queries)",
		Remediation: "Enable Spock AutoDDL for automatic DDL propagation:\n" +
			"  ALTER SYSTEM SET spock.enable_ddl_replication = on;\n" +
			"  ALTER SYSTEM SET spock.include_ddl_repset = on;\n" +
			"Or use spock.replicate_ddl_command() for manual DDL propagation. " +
			"VACUUM and ANALYZE must always be run independently on each node.",
		Metadata: map[string]any{"ddl_count": len(matched)},
	}}, nil
}
