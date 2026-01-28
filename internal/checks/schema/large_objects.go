// Check for large object usage — recommend LOLOR extension.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// LargeObjectsCheck finds large objects and OID columns.
type LargeObjectsCheck struct{}

func init() {
	check.Register(LargeObjectsCheck{})
}

func (LargeObjectsCheck) Name() string     { return "large_objects" }
func (LargeObjectsCheck) Category() string  { return "schema" }
func (LargeObjectsCheck) Mode() string      { return "scan" }
func (LargeObjectsCheck) Description() string {
	return "Large object (LOB) usage — logical decoding does not support them"
}

func (c LargeObjectsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var findings []models.Finding

	// Check if any large objects exist
	var lobCount int
	err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_catalog.pg_largeobject_metadata;").Scan(&lobCount)
	if err != nil {
		return nil, fmt.Errorf("large_objects count query failed: %w", err)
	}

	if lobCount > 0 {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Database contains %d large object(s)", lobCount),
			Detail: fmt.Sprintf(
				"Found %d large object(s) in pg_largeobject_metadata. "+
					"PostgreSQL's logical decoding facility does not support decoding "+
					"changes to large objects. These will not be replicated by Spock.",
				lobCount,
			),
			ObjectName: "pg_largeobject",
			Remediation: "Migrate large objects to use the LOLOR extension for replication-safe " +
				"large object management, or store binary data in BYTEA columns.\n\n" +
				"To use LOLOR:\n" +
				"  CREATE EXTENSION lolor;\n" +
				"  ALTER SYSTEM SET lolor.node = <unique_node_id>;  -- unique per node, 1 to 2^28\n" +
				"  -- Restart PostgreSQL\n" +
				"  SELECT spock.repset_add_table('default', 'lolor.pg_largeobject');\n" +
				"  SELECT spock.repset_add_table('default', 'lolor.pg_largeobject_metadata');",
			Metadata: map[string]any{"lob_count": lobCount},
		})
	}

	// Also check for columns using OID type (commonly used with large objects)
	const oidQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			a.attname AS column_name
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND a.atttypid = 'oid'::regtype
		ORDER BY n.nspname, c.relname, a.attname;
	`

	rows, err := conn.Query(ctx, oidQuery)
	if err != nil {
		return nil, fmt.Errorf("large_objects oid query failed: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var schemaName, tableName, colName string
		if err := rows.Scan(&schemaName, &tableName, &colName); err != nil {
			return nil, fmt.Errorf("large_objects oid scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("OID column '%s.%s' may reference large objects", fqn, colName),
			Detail: fmt.Sprintf(
				"Column '%s' on table '%s' uses the OID data type, "+
					"which is commonly used to reference large objects. If used for LOB "+
					"references, these will not replicate through logical decoding.",
				colName, fqn,
			),
			ObjectName: fmt.Sprintf("%s.%s", fqn, colName),
			Remediation: "If this column references large objects, migrate to LOLOR or " +
				"BYTEA. LOLOR requires lolor.node to be set uniquely per node " +
				"and its tables added to a replication set. " +
				"If the column is used for other purposes, this finding can be ignored.",
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("large_objects oid rows iteration failed: %w", err)
	}
	return findings, nil
}
