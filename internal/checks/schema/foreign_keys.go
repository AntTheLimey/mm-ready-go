// Check foreign key relationships for replication ordering awareness.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// ForeignKeysCheck finds foreign key relationships and CASCADE constraints.
type ForeignKeysCheck struct{}

func init() {
	check.Register(ForeignKeysCheck{})
}

func (ForeignKeysCheck) Name() string     { return "foreign_keys" }
func (ForeignKeysCheck) Category() string  { return "schema" }
func (ForeignKeysCheck) Mode() string      { return "scan" }
func (ForeignKeysCheck) Description() string {
	return "Foreign key relationships — replication ordering and cross-node considerations"
}

func (c ForeignKeysCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			con.conname AS constraint_name,
			rn.nspname AS ref_schema,
			rc.relname AS ref_table,
			confdeltype::text AS delete_action,
			confupdtype::text AS update_action
		FROM pg_catalog.pg_constraint con
		JOIN pg_catalog.pg_class c ON c.oid = con.conrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_class rc ON rc.oid = con.confrelid
		JOIN pg_catalog.pg_namespace rn ON rn.oid = rc.relnamespace
		WHERE con.contype = 'f'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname, con.conname;
	`

	actionLabels := map[string]string{
		"a": "NO ACTION",
		"r": "RESTRICT",
		"c": "CASCADE",
		"n": "SET NULL",
		"d": "SET DEFAULT",
	}

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("foreign_keys query failed: %w", err)
	}
	defer rows.Close()

	type fkRow struct {
		fqn, conName, refFQN, delLabel, updLabel, delAct, updAct string
	}
	var allRows []fkRow

	for rows.Next() {
		var schemaName, tableName, conName, refSchema, refTable, delAct, updAct string
		if err := rows.Scan(&schemaName, &tableName, &conName, &refSchema, &refTable, &delAct, &updAct); err != nil {
			return nil, fmt.Errorf("foreign_keys scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		refFQN := refSchema + "." + refTable
		delLabel := actionLabels[delAct]
		if delLabel == "" {
			delLabel = delAct
		}
		updLabel := actionLabels[updAct]
		if updLabel == "" {
			updLabel = updAct
		}
		allRows = append(allRows, fkRow{fqn, conName, refFQN, delLabel, updLabel, delAct, updAct})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("foreign_keys rows iteration failed: %w", err)
	}

	if len(allRows) == 0 {
		return nil, nil
	}

	var findings []models.Finding
	cascadeCount := 0

	// Report CASCADE FKs specifically
	for _, r := range allRows {
		if r.delAct == "c" || r.updAct == "c" {
			cascadeCount++
			findings = append(findings, models.Finding{
				Severity:  models.SeverityWarning,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("CASCADE foreign key '%s' on '%s'", r.conName, r.fqn),
				Detail: fmt.Sprintf(
					"Foreign key '%s' on '%s' references '%s' with "+
						"ON DELETE %s / ON UPDATE %s. CASCADE actions are "+
						"executed locally on each node, meaning the cascaded changes happen "+
						"independently on provider and subscriber, which can lead to conflicts "+
						"in a multi-master setup.",
					r.conName, r.fqn, r.refFQN, r.delLabel, r.updLabel,
				),
				ObjectName: r.fqn,
				Remediation: "Review CASCADE behavior. In multi-master, consider handling cascades " +
					"in application logic or ensuring operations flow through a single node.",
				Metadata: map[string]any{"constraint": r.conName, "references": r.refFQN},
			})
		}
	}

	// Summary finding about FK count
	findings = append(findings, models.Finding{
		Severity:  models.SeverityConsider,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("Database has %d foreign key constraint(s)", len(allRows)),
		Detail: fmt.Sprintf(
			"Found %d foreign key constraints. Ensure all referenced tables "+
				"are included in the replication set, and that replication ordering will "+
				"satisfy referential integrity.",
			len(allRows),
		),
		ObjectName:  "(database)",
		Remediation: "Ensure all FK-related tables are in the same replication set.",
		Metadata:    map[string]any{"fk_count": len(allRows), "cascade_count": cascadeCount},
	})

	return findings, nil
}
