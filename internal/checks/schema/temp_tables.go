// Check for temporary table definitions (in functions, not runtime).
package schema

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// TempTablesCheck finds functions/procedures that CREATE TEMP TABLE.
type TempTablesCheck struct{}

func init() {
	check.Register(TempTablesCheck{})
}

// Name returns the unique identifier for this check.
func (TempTablesCheck) Name() string     { return "temp_tables" }
// Category returns the check category.
func (TempTablesCheck) Category() string { return "schema" }
// Mode returns when this check runs (scan, audit, or both).
func (TempTablesCheck) Mode() string     { return "scan" }
// Description returns a human-readable summary of this check.
func (TempTablesCheck) Description() string {
	return "TEMPORARY tables — session-local, never replicated"
}

// Run executes the check against the database connection.
func (c TempTablesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			p.proname AS func_name
		FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND p.prokind IN ('f', 'p')
		  AND p.prosrc ~* 'CREATE\s+(TEMP|TEMPORARY)\s+TABLE'
		ORDER BY n.nspname, p.proname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("temp_tables query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, funcName string
		if err := rows.Scan(&schemaName, &funcName); err != nil {
			return nil, fmt.Errorf("temp_tables scan failed: %w", err)
		}
		fqn := schemaName + "." + funcName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Function '%s' creates temporary tables", fqn),
			Detail: fmt.Sprintf(
				"Function '%s' contains CREATE TEMP/TEMPORARY TABLE statements. "+
					"Temporary tables are session-local and are not replicated. This is "+
					"usually fine, but be aware that temp table data will differ across nodes.",
				fqn,
			),
			ObjectName:  fqn,
			Remediation: "Review to confirm temp table usage is intentional and node-local.",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("temp_tables rows iteration failed: %w", err)
	}
	return findings, nil
}
