// Check for foreign key columns without supporting indexes.
package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// MissingFKIndexesCheck finds FK columns on the referencing side that lack a matching index.
type MissingFKIndexesCheck struct{}

func init() {
	check.Register(MissingFKIndexesCheck{})
}

func (MissingFKIndexesCheck) Name() string     { return "missing_fk_indexes" }
func (MissingFKIndexesCheck) Category() string  { return "schema" }
func (MissingFKIndexesCheck) Mode() string      { return "scan" }
func (MissingFKIndexesCheck) Description() string {
	return "Foreign key columns without indexes — slow cascades and lock contention"
}

func (c MissingFKIndexesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			cn.nspname AS schema_name,
			cc.relname AS table_name,
			co.conname AS constraint_name,
			array_agg(a.attname ORDER BY x.ordinality) AS fk_columns
		FROM pg_catalog.pg_constraint co
		JOIN pg_catalog.pg_class cc ON cc.oid = co.conrelid
		JOIN pg_catalog.pg_namespace cn ON cn.oid = cc.relnamespace
		CROSS JOIN LATERAL unnest(co.conkey) WITH ORDINALITY AS x(attnum, ordinality)
		JOIN pg_catalog.pg_attribute a
		  ON a.attrelid = co.conrelid AND a.attnum = x.attnum
		WHERE co.contype = 'f'
		  AND cn.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND NOT EXISTS (
			  SELECT 1
			  FROM pg_catalog.pg_index i
			  WHERE i.indrelid = co.conrelid
				AND (i.indkey::int2[])[0:array_length(co.conkey, 1)-1]
					= co.conkey
		  )
		GROUP BY cn.nspname, cc.relname, co.conname
		ORDER BY cn.nspname, cc.relname, co.conname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("missing_fk_indexes query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, conName string
		var fkCols []string
		if err := rows.Scan(&schemaName, &tableName, &conName, &fkCols); err != nil {
			return nil, fmt.Errorf("missing_fk_indexes scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		colList := strings.Join(fkCols, ", ")
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("No index on FK columns '%s' (%s)", fqn, colList),
			Detail: fmt.Sprintf(
				"Foreign key constraint '%s' on '%s' references "+
					"columns (%s) that have no supporting index. Without "+
					"an index, DELETE and UPDATE on the referenced (parent) table "+
					"require a sequential scan of the child table while holding a "+
					"lock. In multi-master replication, this causes longer lock "+
					"hold times and increases the likelihood of conflicts.",
				conName, fqn, colList,
			),
			ObjectName: fqn,
			Remediation: fmt.Sprintf(
				"Create an index:\n"+
					"  CREATE INDEX ON %s (%s);",
				fqn, colList,
			),
			Metadata: map[string]any{"constraint": conName, "columns": fkCols},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("missing_fk_indexes rows iteration failed: %w", err)
	}
	return findings, nil
}
