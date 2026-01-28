// Check for tables missing primary keys.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// PrimaryKeysCheck finds tables without primary keys.
type PrimaryKeysCheck struct{}

func init() {
	check.Register(PrimaryKeysCheck{})
}

func (PrimaryKeysCheck) Name() string     { return "primary_keys" }
func (PrimaryKeysCheck) Category() string  { return "schema" }
func (PrimaryKeysCheck) Mode() string      { return "scan" }
func (PrimaryKeysCheck) Description() string {
	return "Tables without primary keys — affects Spock replication behaviour"
}

func (c PrimaryKeysCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND NOT EXISTS (
			  SELECT 1
			  FROM pg_catalog.pg_constraint con
			  WHERE con.conrelid = c.oid
				AND con.contype = 'p'
		  )
		ORDER BY n.nspname, c.relname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("primary_keys query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName string
		if err := rows.Scan(&schemaName, &tableName); err != nil {
			return nil, fmt.Errorf("primary_keys scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Table '%s' has no primary key", fqn),
			Detail: fmt.Sprintf(
				"Table '%s' lacks a primary key. Spock automatically places "+
					"tables without primary keys into the 'default_insert_only' "+
					"replication set. In this set, only INSERT and TRUNCATE operations "+
					"are replicated — UPDATE and DELETE operations are silently filtered "+
					"out by the Spock output plugin and never sent to subscribers.",
				fqn,
			),
			ObjectName: fqn,
			Remediation: fmt.Sprintf(
				"Add a primary key to '%s' if UPDATE/DELETE replication is "+
					"needed. If the table is genuinely insert-only (e.g. an event log), "+
					"no action is required — it will replicate correctly in the "+
					"default_insert_only replication set.",
				fqn,
			),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("primary_keys rows iteration failed: %w", err)
	}
	return findings, nil
}
