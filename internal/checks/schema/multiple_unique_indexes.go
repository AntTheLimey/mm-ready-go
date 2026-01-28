// Check for tables with multiple unique indexes — conflict resolution implications.
package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// MultipleUniqueIndexesCheck finds tables with more than one unique index.
type MultipleUniqueIndexesCheck struct{}

func init() {
	check.Register(MultipleUniqueIndexesCheck{})
}

func (MultipleUniqueIndexesCheck) Name() string     { return "multiple_unique_indexes" }
func (MultipleUniqueIndexesCheck) Category() string  { return "schema" }
func (MultipleUniqueIndexesCheck) Mode() string      { return "scan" }
func (MultipleUniqueIndexesCheck) Description() string {
	return "Tables with multiple unique indexes — affects Spock conflict resolution"
}

func (c MultipleUniqueIndexesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			count(*) AS unique_idx_count,
			array_agg(i.relname ORDER BY i.relname) AS index_names
		FROM pg_catalog.pg_index ix
		JOIN pg_catalog.pg_class c ON c.oid = ix.indrelid
		JOIN pg_catalog.pg_class i ON i.oid = ix.indexrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE ix.indisunique
		  AND c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		GROUP BY n.nspname, c.relname
		HAVING count(*) > 1
		ORDER BY count(*) DESC, n.nspname, c.relname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("multiple_unique_indexes query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName string
		var idxCount int
		var indexNames []string
		if err := rows.Scan(&schemaName, &tableName, &idxCount, &indexNames); err != nil {
			return nil, fmt.Errorf("multiple_unique_indexes scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Table '%s' has %d unique indexes", fqn, idxCount),
			Detail: fmt.Sprintf(
				"Table '%s' has %d unique indexes: %s. "+
					"When check_all_uc_indexes is enabled in Spock, the apply worker "+
					"iterates all unique indexes for conflict detection and uses the "+
					"first match it finds (spock_apply_heap.c). With multiple unique "+
					"constraints, conflicts may be detected on different indexes on "+
					"different nodes, which could lead to unexpected resolution behaviour.",
				fqn, idxCount, strings.Join(indexNames, ", "),
			),
			ObjectName: fqn,
			Remediation: "Review whether all unique indexes are necessary for replication " +
				"conflict detection. Consider whether check_all_uc_indexes should " +
				"be enabled, and ensure the application can tolerate conflict " +
				"resolution on any of the unique constraints.",
			Metadata: map[string]any{"unique_index_count": idxCount, "indexes": indexNames},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("multiple_unique_indexes rows iteration failed: %w", err)
	}
	return findings, nil
}
