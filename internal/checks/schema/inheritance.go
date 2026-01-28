// Check for table inheritance — poorly supported in logical replication.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// InheritanceCheck finds traditional (non-partition) table inheritance.
type InheritanceCheck struct{}

func init() {
	check.Register(InheritanceCheck{})
}

func (InheritanceCheck) Name() string     { return "inheritance" }
func (InheritanceCheck) Category() string  { return "schema" }
func (InheritanceCheck) Mode() string      { return "scan" }
func (InheritanceCheck) Description() string {
	return "Table inheritance (non-partition) — not well supported in logical replication"
}

func (c InheritanceCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			pn.nspname AS parent_schema,
			pc.relname AS parent_table,
			cn.nspname AS child_schema,
			cc.relname AS child_table
		FROM pg_catalog.pg_inherits i
		JOIN pg_catalog.pg_class pc ON pc.oid = i.inhparent
		JOIN pg_catalog.pg_namespace pn ON pn.oid = pc.relnamespace
		JOIN pg_catalog.pg_class cc ON cc.oid = i.inhrelid
		JOIN pg_catalog.pg_namespace cn ON cn.oid = cc.relnamespace
		WHERE pc.relkind = 'r'  -- exclude partitioned tables (relkind='p')
		  AND cc.relkind = 'r'
		  AND pn.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY pn.nspname, pc.relname, cn.nspname, cc.relname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("inheritance query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var parentSchema, parentTable, childSchema, childTable string
		if err := rows.Scan(&parentSchema, &parentTable, &childSchema, &childTable); err != nil {
			return nil, fmt.Errorf("inheritance scan failed: %w", err)
		}
		parentFQN := parentSchema + "." + parentTable
		childFQN := childSchema + "." + childTable
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Table inheritance: '%s' inherits from '%s'", childFQN, parentFQN),
			Detail: fmt.Sprintf(
				"Table '%s' uses traditional table inheritance from "+
					"'%s'. Logical replication does not replicate through "+
					"inheritance hierarchies — each table is replicated independently. "+
					"Queries against the parent that include child data via inheritance "+
					"may behave differently across nodes.",
				childFQN, parentFQN,
			),
			ObjectName: childFQN,
			Remediation: "Consider migrating from table inheritance to declarative partitioning " +
				"(if appropriate) or separate standalone tables.",
			Metadata: map[string]any{"parent": parentFQN},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("inheritance rows iteration failed: %w", err)
	}
	return findings, nil
}
