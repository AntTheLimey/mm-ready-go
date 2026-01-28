// Check for deferrable unique/PK constraints — Spock skips them for conflict resolution.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// DeferrableConstraintsCheck finds deferrable unique/PK constraints.
type DeferrableConstraintsCheck struct{}

func init() {
	check.Register(DeferrableConstraintsCheck{})
}

func (DeferrableConstraintsCheck) Name() string     { return "deferrable_constraints" }
func (DeferrableConstraintsCheck) Category() string  { return "schema" }
func (DeferrableConstraintsCheck) Mode() string      { return "scan" }
func (DeferrableConstraintsCheck) Description() string {
	return "Deferrable unique/PK constraints — silently skipped by Spock conflict resolution"
}

func (c DeferrableConstraintsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			con.conname AS constraint_name,
			con.contype::text AS constraint_type,
			con.condeferrable AS is_deferrable,
			con.condeferred AS is_deferred
		FROM pg_catalog.pg_constraint con
		JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE con.contype IN ('p', 'u')
		  AND con.condeferrable = true
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname, con.conname;
	`

	typeLabels := map[string]string{
		"p": "PRIMARY KEY",
		"u": "UNIQUE",
	}

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("deferrable_constraints query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, conName, conType string
		var isDeferrable, isDeferred bool
		if err := rows.Scan(&schemaName, &tableName, &conName, &conType, &isDeferrable, &isDeferred); err != nil {
			return nil, fmt.Errorf("deferrable_constraints scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		conLabel := typeLabels[conType]
		if conLabel == "" {
			conLabel = conType
		}

		severity := models.SeverityWarning
		if conType == "p" {
			severity = models.SeverityCritical
		}

		initiallyStr := "IMMEDIATE"
		if isDeferred {
			initiallyStr = "DEFERRED"
		}

		findings = append(findings, models.Finding{
			Severity:  severity,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Deferrable %s '%s' on '%s'", conLabel, conName, fqn),
			Detail: fmt.Sprintf(
				"Table '%s' has a DEFERRABLE %s constraint "+
					"'%s' (initially %s). "+
					"Spock's conflict resolution checks indimmediate on indexes via "+
					"IsIndexUsableForInsertConflict() and silently SKIPS deferrable "+
					"indexes. This means conflicts on this constraint will NOT be "+
					"detected during replication apply, potentially causing "+
					"duplicate key violations or data inconsistencies.",
				fqn, conLabel, conName, initiallyStr,
			),
			ObjectName: fmt.Sprintf("%s.%s", fqn, conName),
			Remediation: fmt.Sprintf(
				"If possible, make the constraint non-deferrable:\n"+
					"  ALTER TABLE %s ALTER CONSTRAINT %s NOT DEFERRABLE;\n"+
					"If deferral is required by the application, be aware that Spock "+
					"will not use this constraint for conflict detection.",
				fqn, conName,
			),
			Metadata: map[string]any{
				"constraint_type":    conLabel,
				"initially_deferred": isDeferred,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("deferrable_constraints rows iteration failed: %w", err)
	}
	return findings, nil
}
