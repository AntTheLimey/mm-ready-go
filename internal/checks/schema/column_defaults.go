// Check for volatile column defaults that may produce different values per node.
package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// ColumnDefaultsCheck finds volatile column defaults (now(), random(), etc.).
type ColumnDefaultsCheck struct{}

func init() {
	check.Register(ColumnDefaultsCheck{})
}

func (ColumnDefaultsCheck) Name() string     { return "column_defaults" }
func (ColumnDefaultsCheck) Category() string  { return "schema" }
func (ColumnDefaultsCheck) Mode() string      { return "scan" }
func (ColumnDefaultsCheck) Description() string {
	return "Volatile column defaults (now(), random(), etc.) — may differ across nodes"
}

// volatilePatterns are substrings that indicate volatile defaults.
var volatilePatterns = []string{
	"now()", "current_timestamp", "current_date", "current_time",
	"clock_timestamp()", "statement_timestamp()", "transaction_timestamp()",
	"timeofday()", "random()", "gen_random_uuid()", "uuid_generate_",
	"pg_current_xact_id()",
}

func (c ColumnDefaultsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			a.attname AS column_name,
			pg_get_expr(d.adbin, d.adrelid) AS default_expr
		FROM pg_catalog.pg_attrdef d
		JOIN pg_catalog.pg_attribute a ON a.attrelid = d.adrelid AND a.attnum = d.adnum
		JOIN pg_catalog.pg_class c ON c.oid = d.adrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND NOT a.attisdropped
		  AND a.attgenerated = ''
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname, a.attname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("column_defaults query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, colName string
		var defaultExpr *string
		if err := rows.Scan(&schemaName, &tableName, &colName, &defaultExpr); err != nil {
			return nil, fmt.Errorf("column_defaults scan failed: %w", err)
		}
		if defaultExpr == nil {
			continue
		}
		exprLower := strings.ToLower(*defaultExpr)

		// Skip nextval — handled by sequence_pks check
		if strings.Contains(exprLower, "nextval(") {
			continue
		}

		matched := false
		for _, p := range volatilePatterns {
			if strings.Contains(exprLower, p) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		fqn := schemaName + "." + tableName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Volatile default on '%s.%s'", fqn, colName),
			Detail: fmt.Sprintf(
				"Column '%s' on table '%s' has a volatile default: "+
					"%s. In multi-master replication, if a row is inserted "+
					"without specifying this column, each node could compute a different "+
					"default value. However, Spock replicates the actual inserted value, "+
					"so this is only an issue if the same row is independently inserted "+
					"on multiple nodes.",
				colName, fqn, *defaultExpr,
			),
			ObjectName: fmt.Sprintf("%s.%s", fqn, colName),
			Remediation: "Ensure the application always provides an explicit value for this column, " +
				"or accept that conflict resolution may be needed for concurrent inserts.",
			Metadata: map[string]any{"default_expr": *defaultExpr},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("column_defaults rows iteration failed: %w", err)
	}
	return findings, nil
}
