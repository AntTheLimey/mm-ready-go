// Check for LISTEN/NOTIFY usage — not replicated by logical replication.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// NotifyListenCheck finds NOTIFY/pg_notify usage in functions and pg_stat_statements.
type NotifyListenCheck struct{}

func init() {
	check.Register(NotifyListenCheck{})
}

func (NotifyListenCheck) Name() string     { return "notify_listen" }
func (NotifyListenCheck) Category() string  { return "schema" }
func (NotifyListenCheck) Mode() string      { return "scan" }
func (NotifyListenCheck) Description() string {
	return "LISTEN/NOTIFY usage — notifications are not replicated by Spock"
}

func (c NotifyListenCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var findings []models.Finding

	// Check functions that use pg_notify or NOTIFY
	const funcQuery = `
		SELECT
			n.nspname AS schema_name,
			p.proname AS func_name,
			pg_get_functiondef(p.oid) AS func_def
		FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND (
			  prosrc ~* 'pg_notify' OR
			  prosrc ~* '\bNOTIFY\b'
		  )
		ORDER BY n.nspname, p.proname;
	`

	rows, err := conn.Query(ctx, funcQuery)
	if err != nil {
		return nil, fmt.Errorf("notify_listen func query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, funcName, funcDef string
		if err := rows.Scan(&schemaName, &funcName, &funcDef); err != nil {
			return nil, fmt.Errorf("notify_listen func scan failed: %w", err)
		}
		fqn := schemaName + "." + funcName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Function '%s' uses NOTIFY/pg_notify", fqn),
			Detail: fmt.Sprintf(
				"Function '%s' contains NOTIFY or pg_notify() calls. "+
					"LISTEN/NOTIFY is a PostgreSQL inter-process communication "+
					"mechanism that is NOT replicated by logical replication. "+
					"If application components rely on notifications triggered by "+
					"data changes, those notifications will only fire on the node "+
					"where the change originates — not on subscriber nodes.",
				fqn,
			),
			ObjectName: fqn,
			Remediation: "If notifications are used as part of the application architecture, " +
				"ensure that listeners connect to all nodes, or implement an " +
				"application-level notification mechanism that works across nodes.",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("notify_listen func rows iteration failed: %w", err)
	}

	// Check pg_stat_statements for NOTIFY usage — fail gracefully if not available
	const stmtQuery = `
		SELECT query, calls
		FROM pg_stat_statements
		WHERE query ~* '\bNOTIFY\b'
		   OR query ~* 'pg_notify'
		ORDER BY calls DESC;
	`

	stmtRows, err := conn.Query(ctx, stmtQuery)
	if err != nil {
		// pg_stat_statements not available — skip silently
		return findings, nil
	}
	defer stmtRows.Close()

	for stmtRows.Next() {
		var queryText string
		var calls int64
		if err := stmtRows.Scan(&queryText, &calls); err != nil {
			// Skip on scan error
			break
		}
		truncated := queryText
		if len(truncated) > 200 {
			truncated = truncated[:200]
		}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("NOTIFY pattern in queries (%d call(s))", calls),
			Detail: fmt.Sprintf(
				"Query executed %d time(s): %s...\n\n"+
					"NOTIFY calls are not replicated by Spock. Subscribers will "+
					"not receive these notifications.",
				calls, truncated,
			),
			ObjectName: "(query)",
			Metadata:   map[string]any{"calls": calls},
		})
	}
	// Ignore stmtRows.Err() — pg_stat_statements may not be fully available

	return findings, nil
}
