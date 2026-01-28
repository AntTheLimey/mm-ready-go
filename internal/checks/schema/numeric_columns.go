// Check for numeric SUM/COUNT columns that may be Delta-Apply candidates.
package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// NumericColumnsCheck finds numeric columns whose names suggest accumulator/counter patterns.
type NumericColumnsCheck struct{}

func init() {
	check.Register(NumericColumnsCheck{})
}

func (NumericColumnsCheck) Name() string     { return "numeric_columns" }
func (NumericColumnsCheck) Category() string  { return "schema" }
func (NumericColumnsCheck) Mode() string      { return "scan" }
func (NumericColumnsCheck) Description() string {
	return "Numeric columns that may be Delta-Apply candidates (counters, balances, etc.)"
}

// suspectPatterns are column name substrings that suggest accumulator/counter patterns.
var suspectPatterns = []string{
	"count", "total", "sum", "balance", "quantity", "qty",
	"amount", "tally", "counter", "num_", "cnt", "running_",
	"cumulative", "aggregate", "accrued", "inventory",
}

func (c NumericColumnsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			a.attname AS column_name,
			format_type(a.atttypid, a.atttypmod) AS data_type,
			a.attnotnull AS is_not_null
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND a.atttypid IN (
			  'integer'::regtype, 'bigint'::regtype, 'smallint'::regtype,
			  'numeric'::regtype, 'real'::regtype, 'double precision'::regtype
		  )
		ORDER BY n.nspname, c.relname, a.attname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("numeric_columns query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, colName, dataType string
		var isNotNull bool
		if err := rows.Scan(&schemaName, &tableName, &colName, &dataType, &isNotNull); err != nil {
			return nil, fmt.Errorf("numeric_columns scan failed: %w", err)
		}

		colLower := strings.ToLower(colName)
		matched := false
		for _, p := range suspectPatterns {
			if strings.Contains(colLower, p) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		fqn := schemaName + "." + tableName

		if !isNotNull {
			// Delta-apply requires NOT NULL (spock_apply_heap.c:613-627)
			findings = append(findings, models.Finding{
				Severity:  models.SeverityWarning,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("Delta-Apply candidate '%s.%s' allows NULL", fqn, colName),
				Detail: fmt.Sprintf(
					"Column '%s' on table '%s' is numeric (%s) "+
						"and its name suggests it may be an accumulator or counter. "+
						"If configured for Delta-Apply in Spock, the column MUST have a "+
						"NOT NULL constraint. The Spock apply worker "+
						"(spock_apply_heap.c:613-627) checks this and will reject "+
						"delta-apply on nullable columns.",
					colName, fqn, dataType,
				),
				ObjectName: fmt.Sprintf("%s.%s", fqn, colName),
				Remediation: fmt.Sprintf(
					"If this column will use Delta-Apply, add a NOT NULL constraint:\n"+
						"  ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;\n"+
						"Ensure existing rows have no NULL values first.",
					fqn, colName,
				),
				Metadata: map[string]any{"column": colName, "data_type": dataType, "nullable": true},
			})
		} else {
			findings = append(findings, models.Finding{
				Severity:  models.SeverityConsider,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("Potential Delta-Apply column: '%s.%s' (%s)", fqn, colName, dataType),
				Detail: fmt.Sprintf(
					"Column '%s' on table '%s' is numeric (%s) "+
						"and its name suggests it may be an accumulator or counter. In "+
						"multi-master replication, concurrent updates to such columns can "+
						"cause conflicts. Delta-Apply can resolve this by applying the "+
						"delta (change) rather than the absolute value. This column has a "+
						"NOT NULL constraint, so it meets the Delta-Apply prerequisite.",
					colName, fqn, dataType,
				),
				ObjectName: fmt.Sprintf("%s.%s", fqn, colName),
				Remediation: "Investigate whether this column receives concurrent " +
					"increment/decrement updates from multiple nodes. If so, " +
					"configure it for Delta-Apply in Spock.",
				Metadata: map[string]any{"column": colName, "data_type": dataType, "nullable": false},
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("numeric_columns rows iteration failed: %w", err)
	}
	return findings, nil
}
