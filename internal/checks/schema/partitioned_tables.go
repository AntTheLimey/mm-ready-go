// Check for partitioned tables and their partition strategies.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// PartitionedTablesCheck finds partitioned tables and reviews their strategy.
type PartitionedTablesCheck struct{}

func init() {
	check.Register(PartitionedTablesCheck{})
}

func (PartitionedTablesCheck) Name() string     { return "partitioned_tables" }
func (PartitionedTablesCheck) Category() string  { return "schema" }
func (PartitionedTablesCheck) Mode() string      { return "scan" }
func (PartitionedTablesCheck) Description() string {
	return "Partitioned tables — review partition strategy for Spock compatibility"
}

func (c PartitionedTablesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			pt.partstrat::text AS strategy,
			(
				SELECT count(*)
				FROM pg_catalog.pg_inherits i
				WHERE i.inhparent = c.oid
			) AS partition_count
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_partitioned_table pt ON pt.partrelid = c.oid
		WHERE c.relkind = 'p'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname;
	`

	strategyLabels := map[string]string{
		"r": "RANGE",
		"l": "LIST",
		"h": "HASH",
	}

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("partitioned_tables query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, strategy string
		var partCount int
		if err := rows.Scan(&schemaName, &tableName, &strategy, &partCount); err != nil {
			return nil, fmt.Errorf("partitioned_tables scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		stratLabel := strategyLabels[strategy]
		if stratLabel == "" {
			stratLabel = strategy
		}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Partitioned table '%s' (%s, %d partitions)", fqn, stratLabel, partCount),
			Detail: fmt.Sprintf(
				"Table '%s' uses %s partitioning with %d "+
					"partition(s). Spock 5 supports partition replication, but the partition "+
					"structure must be identical on all nodes. Adding/removing partitions "+
					"must be coordinated across the cluster.",
				fqn, stratLabel, partCount,
			),
			ObjectName: fqn,
			Remediation: "Ensure partition definitions are identical across all nodes. " +
				"Plan partition maintenance (add/drop) as a coordinated cluster " +
				"operation.\n\n" +
				"Important: detaching a partition (ALTER TABLE ... DETACH PARTITION) " +
				"does NOT automatically remove it from the replication set. The " +
				"Spock AutoDDL code handles AT_AttachPartition but not " +
				"AT_DetachPartition. After detaching, manually remove the " +
				"orphaned table if replication is no longer needed:\n" +
				"  SELECT spock.repset_remove_table('default', 'schema.partition_name');",
			Metadata: map[string]any{"strategy": stratLabel, "partition_count": partCount},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("partitioned_tables rows iteration failed: %w", err)
	}
	return findings, nil
}
