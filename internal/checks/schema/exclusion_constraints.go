// Check for exclusion constraints — not supported by logical replication.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// ExclusionConstraintsCheck finds exclusion constraints that cannot be enforced across nodes.
type ExclusionConstraintsCheck struct{}

func init() {
	check.Register(ExclusionConstraintsCheck{})
}

func (ExclusionConstraintsCheck) Name() string     { return "exclusion_constraints" }
func (ExclusionConstraintsCheck) Category() string  { return "schema" }
func (ExclusionConstraintsCheck) Mode() string      { return "scan" }
func (ExclusionConstraintsCheck) Description() string {
	return "Exclusion constraints — not enforceable across Spock nodes"
}

func (c ExclusionConstraintsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			con.conname AS constraint_name
		FROM pg_catalog.pg_constraint con
		JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE con.contype = 'x'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname, con.conname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("exclusion_constraints query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, constraintName string
		if err := rows.Scan(&schemaName, &tableName, &constraintName); err != nil {
			return nil, fmt.Errorf("exclusion_constraints scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Exclusion constraint '%s' on '%s'", constraintName, fqn),
			Detail: fmt.Sprintf(
				"Table '%s' has exclusion constraint '%s'. "+
					"Exclusion constraints are evaluated locally on each node. In a "+
					"multi-master topology, two nodes could independently accept rows "+
					"that would violate the exclusion constraint if evaluated globally, "+
					"leading to replication conflicts or data inconsistencies.",
				fqn, constraintName,
			),
			ObjectName: fmt.Sprintf("%s.%s", fqn, constraintName),
			Remediation: "Review whether this exclusion constraint can be replaced with " +
				"application-level logic, or ensure that only one node writes data " +
				"that could conflict under this constraint.",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("exclusion_constraints rows iteration failed: %w", err)
	}
	return findings, nil
}
