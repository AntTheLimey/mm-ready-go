package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// RepsetMembershipCheck verifies all user tables are in a Spock replication set.
type RepsetMembershipCheck struct{}

func init() {
	check.Register(&RepsetMembershipCheck{})
}

func (c *RepsetMembershipCheck) Name() string     { return "repset_membership" }
func (c *RepsetMembershipCheck) Category() string  { return "replication" }
func (c *RepsetMembershipCheck) Description() string {
	return "Verify all user tables are in a Spock replication set"
}
func (c *RepsetMembershipCheck) Mode() string { return "audit" }

func (c *RepsetMembershipCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	// Check if spock schema exists
	var hasSpock bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_namespace WHERE nspname = 'spock'
		);
	`).Scan(&hasSpock)
	if err != nil {
		hasSpock = false
	}

	if !hasSpock {
		return []models.Finding{{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Spock schema not found — skipping repset membership check",
			Detail:     "The spock schema does not exist in this database.",
			ObjectName: "spock",
		}}, nil
	}

	// Find user tables not in any replication set
	query := `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND NOT EXISTS (
			  SELECT 1
			  FROM spock.repset_table rt
			  WHERE rt.set_reloid = c.oid
		  )
		ORDER BY n.nspname, c.relname;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return []models.Finding{{
			Severity:   models.SeverityWarning,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Could not query spock.repset_table",
			Detail:     fmt.Sprintf("Error querying replication set membership: %v", err),
			ObjectName: "spock.repset_table",
		}}, nil
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName string
		if err := rows.Scan(&schemaName, &tableName); err != nil {
			return nil, fmt.Errorf("scanning repset membership row: %w", err)
		}
		fqn := fmt.Sprintf("%s.%s", schemaName, tableName)
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Table '%s' is not in any replication set", fqn),
			Detail: fmt.Sprintf(
				"Table '%s' exists but is not a member of any Spock replication set. "+
					"This table will NOT be replicated to other nodes. If this is "+
					"intentional (e.g. node-local temp/staging data), no action is needed.", fqn),
			ObjectName: fqn,
			Remediation: fmt.Sprintf(
				"Add the table to a replication set:\n"+
					"  SELECT spock.repset_add_table('default', '%s');\n"+
					"Or for insert-only tables:\n"+
					"  SELECT spock.repset_add_table('default_insert_only', '%s');", fqn, fqn),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating repset membership rows: %w", err)
	}

	return findings, nil
}
