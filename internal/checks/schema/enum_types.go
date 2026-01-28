// Check for ENUM types — DDL changes to enums require coordination.
package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// EnumTypesCheck finds ENUM types that require multi-node coordination for DDL changes.
type EnumTypesCheck struct{}

func init() {
	check.Register(EnumTypesCheck{})
}

func (EnumTypesCheck) Name() string     { return "enum_types" }
func (EnumTypesCheck) Category() string  { return "schema" }
func (EnumTypesCheck) Mode() string      { return "scan" }
func (EnumTypesCheck) Description() string {
	return "ENUM types — DDL changes to enums require multi-node coordination"
}

func (c EnumTypesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			t.typname AS type_name,
			array_agg(e.enumlabel ORDER BY e.enumsortorder) AS labels
		FROM pg_catalog.pg_type t
		JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
		JOIN pg_catalog.pg_enum e ON e.enumtypid = t.oid
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		GROUP BY n.nspname, t.typname
		ORDER BY n.nspname, t.typname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("enum_types query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, typeName string
		var labels []string
		if err := rows.Scan(&schemaName, &typeName, &labels); err != nil {
			return nil, fmt.Errorf("enum_types scan failed: %w", err)
		}
		fqn := schemaName + "." + typeName
		labelCount := len(labels)

		displayLabels := labels
		suffix := ""
		if len(displayLabels) > 10 {
			displayLabels = displayLabels[:10]
			suffix = "..."
		}

		// Keep up to 20 labels in metadata
		metaLabels := labels
		if len(metaLabels) > 20 {
			metaLabels = metaLabels[:20]
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("ENUM type '%s' (%d values)", fqn, labelCount),
			Detail: fmt.Sprintf(
				"ENUM type '%s' has %d values: %s%s. "+
					"In multi-master replication, ALTER TYPE ... ADD VALUE is a DDL "+
					"change that must be applied on all nodes. Spock can replicate DDL "+
					"through the ddl_sql replication set, but ENUM modifications must "+
					"be coordinated carefully to avoid type mismatches during apply.",
				fqn, labelCount, strings.Join(displayLabels, ", "), suffix,
			),
			ObjectName: fqn,
			Remediation: "Plan ENUM modifications to be applied through Spock's DDL " +
				"replication (spock.replicate_ddl) to ensure all nodes stay in sync. " +
				"Alternatively, consider using a lookup table instead of ENUMs for " +
				"values that change frequently.",
			Metadata: map[string]any{"label_count": labelCount, "labels": metaLabels},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("enum_types rows iteration failed: %w", err)
	}
	return findings, nil
}
