// Check for tables without PKs that have UPDATE/DELETE activity.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// UpdateDeleteNoPkCheck finds tables without primary keys that have UPDATE/DELETE activity.
type UpdateDeleteNoPkCheck struct{}

func init() {
	check.Register(UpdateDeleteNoPkCheck{})
}

func (UpdateDeleteNoPkCheck) Name() string     { return "tables_update_delete_no_pk" }
func (UpdateDeleteNoPkCheck) Category() string  { return "schema" }
func (UpdateDeleteNoPkCheck) Mode() string      { return "scan" }
func (UpdateDeleteNoPkCheck) Description() string {
	return "Tables without primary keys that have UPDATE/DELETE activity — " +
		"these operations are silently dropped by Spock"
}

func (c UpdateDeleteNoPkCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			s.n_tup_upd AS updates,
			s.n_tup_del AS deletes,
			s.n_tup_ins AS inserts
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_stat_user_tables s
			ON s.schemaname = n.nspname AND s.relname = c.relname
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND NOT EXISTS (
			  SELECT 1
			  FROM pg_catalog.pg_constraint con
			  WHERE con.conrelid = c.oid AND con.contype = 'p'
		  )
		ORDER BY n.nspname, c.relname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("tables_update_delete_no_pk query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName string
		var updates, deletes, inserts int64
		if err := rows.Scan(&schemaName, &tableName, &updates, &deletes, &inserts); err != nil {
			return nil, fmt.Errorf("tables_update_delete_no_pk scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName

		if updates > 0 || deletes > 0 {
			findings = append(findings, models.Finding{
				Severity:  models.SeverityCritical,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("Table '%s' has UPDATE/DELETE activity but no primary key", fqn),
				Detail: fmt.Sprintf(
					"Table '%s' has no primary key and shows "+
						"%d UPDATE(s) and %d DELETE(s) "+
						"(plus %d INSERT(s)) since the last stats reset. "+
						"Spock places tables without primary keys into the "+
						"'default_insert_only' replication set, where UPDATE and "+
						"DELETE operations are silently filtered out by the output "+
						"plugin (spock_output_plugin.c). This means changes would "+
						"be LOST on subscriber nodes.",
					fqn, updates, deletes, inserts,
				),
				ObjectName: fqn,
				Remediation: fmt.Sprintf(
					"Add a primary key to '%s' so it can be placed in the "+
						"'default' replication set and replicate all DML operations. "+
						"Note: REPLICA IDENTITY FULL is NOT a substitute — Spock's "+
						"get_replication_identity() returns InvalidOid for FULL "+
						"without a PK.",
					fqn,
				),
				Metadata: map[string]any{
					"updates": updates,
					"deletes": deletes,
					"inserts": inserts,
				},
			})
		} else if inserts > 0 {
			findings = append(findings, models.Finding{
				Severity:  models.SeverityInfo,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("Table '%s' is insert-only with no PK (OK for replication)", fqn),
				Detail: fmt.Sprintf(
					"Table '%s' has no primary key but only shows INSERT "+
						"activity (%d inserts, 0 updates, 0 deletes). "+
						"This table will be placed in the 'default_insert_only' "+
						"replication set, which correctly replicates INSERT and "+
						"TRUNCATE operations.",
					fqn, inserts,
				),
				ObjectName: fqn,
				Metadata:   map[string]any{"inserts": inserts},
			})
		}
		// Tables with zero activity are skipped — no findings needed.
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tables_update_delete_no_pk rows iteration failed: %w", err)
	}
	return findings, nil
}
