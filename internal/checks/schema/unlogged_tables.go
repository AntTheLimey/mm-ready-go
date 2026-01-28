// Check for UNLOGGED tables — not replicated by Spock.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// UnloggedTablesCheck finds tables with UNLOGGED persistence.
type UnloggedTablesCheck struct{}

func init() {
	check.Register(UnloggedTablesCheck{})
}

func (UnloggedTablesCheck) Name() string     { return "unlogged_tables" }
func (UnloggedTablesCheck) Category() string  { return "schema" }
func (UnloggedTablesCheck) Mode() string      { return "scan" }
func (UnloggedTablesCheck) Description() string {
	return "UNLOGGED tables — not written to WAL and cannot be replicated"
}

func (c UnloggedTablesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relpersistence = 'u'
		  AND c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("unlogged_tables query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName string
		if err := rows.Scan(&schemaName, &tableName); err != nil {
			return nil, fmt.Errorf("unlogged_tables scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("UNLOGGED table '%s'", fqn),
			Detail: fmt.Sprintf(
				"Table '%s' is UNLOGGED. Unlogged tables are not written to the "+
					"write-ahead log and therefore cannot be replicated by Spock. Data in "+
					"this table will exist only on the local node.",
				fqn,
			),
			ObjectName: fqn,
			Remediation: fmt.Sprintf(
				"If this table needs to be replicated, convert it: "+
					"ALTER TABLE %s SET LOGGED;",
				fqn,
			),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("unlogged_tables rows iteration failed: %w", err)
	}
	return findings, nil
}
