// Check for generated (computed/stored) columns.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// GeneratedColumnsCheck finds generated/stored columns with replication behavior differences.
type GeneratedColumnsCheck struct{}

func init() {
	check.Register(GeneratedColumnsCheck{})
}

func (GeneratedColumnsCheck) Name() string     { return "generated_columns" }
func (GeneratedColumnsCheck) Category() string  { return "schema" }
func (GeneratedColumnsCheck) Mode() string      { return "scan" }
func (GeneratedColumnsCheck) Description() string {
	return "Generated/stored columns — replication behavior differences"
}

func (c GeneratedColumnsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			a.attname AS column_name,
			a.attgenerated::text AS gen_type,
			pg_get_expr(d.adbin, d.adrelid) AS expression
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
		WHERE c.relkind = 'r'
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		  AND a.attgenerated != ''
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname, a.attname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("generated_columns query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, colName, genType string
		var expression *string
		if err := rows.Scan(&schemaName, &tableName, &colName, &genType, &expression); err != nil {
			return nil, fmt.Errorf("generated_columns scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		genLabel := "VIRTUAL"
		if genType == "s" {
			genLabel = "STORED"
		}
		exprStr := ""
		if expression != nil {
			exprStr = *expression
		}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Generated column '%s.%s' (%s)", fqn, colName, genLabel),
			Detail: fmt.Sprintf(
				"Column '%s' on table '%s' is a %s generated column "+
					"with expression: %s. Generated columns are recomputed on the "+
					"subscriber side. If the expression depends on functions or data that "+
					"differs across nodes, values may diverge.",
				colName, fqn, genLabel, exprStr,
			),
			ObjectName: fmt.Sprintf("%s.%s", fqn, colName),
			Remediation: "Verify the generation expression produces identical results on all nodes. " +
				"Avoid expressions that depend on volatile functions or node-local state.",
			Metadata: map[string]any{"gen_type": genLabel, "expression": exprStr},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("generated_columns rows iteration failed: %w", err)
	}
	return findings, nil
}
