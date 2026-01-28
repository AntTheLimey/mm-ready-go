// Check for primary keys backed by standard sequences (need snowflake migration).
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// SequencePKsCheck finds PK columns backed by standard sequences that need snowflake migration.
type SequencePKsCheck struct{}

func init() {
	check.Register(SequencePKsCheck{})
}

func (SequencePKsCheck) Name() string     { return "sequence_pks" }
func (SequencePKsCheck) Category() string  { return "schema" }
func (SequencePKsCheck) Mode() string      { return "scan" }
func (SequencePKsCheck) Description() string {
	return "Primary keys using standard sequences — must migrate to pgEdge snowflake"
}

func (c SequencePKsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			a.attname AS column_name,
			pg_get_serial_sequence(quote_ident(n.nspname) || '.' || quote_ident(c.relname), a.attname) AS seq_name
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_constraint con ON con.conrelid = c.oid AND con.contype = 'p'
		JOIN pg_catalog.pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
		WHERE c.relkind = 'r'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND (
			  pg_get_serial_sequence(quote_ident(n.nspname) || '.' || quote_ident(c.relname), a.attname) IS NOT NULL
			  OR a.attidentity != ''
		  )
		ORDER BY n.nspname, c.relname, a.attname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("sequence_pks query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, colName string
		var seqName *string
		if err := rows.Scan(&schemaName, &tableName, &colName, &seqName); err != nil {
			return nil, fmt.Errorf("sequence_pks scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		seqDisplay := "identity column"
		if seqName != nil {
			seqDisplay = *seqName
		}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityCritical,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("PK column '%s.%s' uses a standard sequence", fqn, colName),
			Detail: fmt.Sprintf(
				"Primary key column '%s' on table '%s' is backed by "+
					"sequence '%s'. In a multi-master setup, "+
					"standard sequences will produce conflicting values across nodes. "+
					"Must migrate to pgEdge snowflake sequences.",
				colName, fqn, seqDisplay,
			),
			ObjectName: fqn,
			Remediation: fmt.Sprintf(
				"Convert '%s.%s' to use the pgEdge snowflake extension "+
					"for globally unique ID generation. See: pgEdge snowflake documentation.",
				fqn, colName,
			),
			Metadata: map[string]any{"column": colName, "sequence": seqName},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sequence_pks rows iteration failed: %w", err)
	}
	return findings, nil
}
