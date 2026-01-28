// Audit all sequences for multi-master migration planning.
package sequences

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// SequenceAuditCheck audits all sequences, types, and ownership for snowflake migration.
type SequenceAuditCheck struct{}

func init() {
	check.Register(SequenceAuditCheck{})
}

func (SequenceAuditCheck) Name() string        { return "sequence_audit" }
func (SequenceAuditCheck) Category() string     { return "sequences" }
func (SequenceAuditCheck) Mode() string         { return "scan" }
func (SequenceAuditCheck) Description() string {
	return "All sequences, types, and ownership — need snowflake migration plan"
}

func (c SequenceAuditCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS sequence_name,
			s.seqtypid::regtype::text AS data_type,
			s.seqstart AS start_value,
			s.seqincrement AS increment,
			s.seqmin AS min_value,
			s.seqmax AS max_value,
			s.seqcycle AS is_cycle,
			d.refobjid IS NOT NULL AS is_owned,
			CASE WHEN d.refobjid IS NOT NULL THEN
				(SELECT relname FROM pg_class WHERE oid = d.refobjid)
			ELSE NULL END AS owner_table,
			CASE WHEN d.refobjid IS NOT NULL THEN
				(SELECT attname FROM pg_attribute
				 WHERE attrelid = d.refobjid AND attnum = d.refobjsubid)
			ELSE NULL END AS owner_column
		FROM pg_catalog.pg_sequence s
		JOIN pg_catalog.pg_class c ON c.oid = s.seqrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		LEFT JOIN pg_catalog.pg_depend d
			ON d.objid = s.seqrelid
			AND d.deptype = 'a'
			AND d.classid = 'pg_class'::regclass
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sequence_audit query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var (
			schemaName, seqName, dataType string
			startVal, increment           int64
			minVal, maxVal                int64
			isCycle, isOwned              bool
			ownerTable, ownerColumn       *string
		)
		if err := rows.Scan(
			&schemaName, &seqName, &dataType,
			&startVal, &increment, &minVal, &maxVal,
			&isCycle, &isOwned, &ownerTable, &ownerColumn,
		); err != nil {
			return nil, fmt.Errorf("sequence_audit scan failed: %w", err)
		}

		fqn := fmt.Sprintf("%s.%s", schemaName, seqName)
		ownership := "not owned by any column"
		if isOwned && ownerTable != nil && ownerColumn != nil {
			ownership = fmt.Sprintf("owned by %s.%s", *ownerTable, *ownerColumn)
		}

		cycleStr := "no"
		if isCycle {
			cycleStr = "yes"
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Sequence '%s' (%s, %s)", fqn, dataType, ownership),
			Detail: fmt.Sprintf(
				"Sequence '%s': type=%s, start=%d, increment=%d, cycle=%s, %s. "+
					"Standard sequences produce overlapping values in multi-master setups. "+
					"Must migrate to pgEdge snowflake sequences or implement another "+
					"globally-unique ID strategy.",
				fqn, dataType, startVal, increment, cycleStr, ownership,
			),
			ObjectName: fqn,
			Remediation: fmt.Sprintf(
				"Migrate sequence '%s' to use pgEdge snowflake for globally "+
					"unique ID generation across all cluster nodes.", fqn,
			),
			Metadata: map[string]any{
				"data_type":    dataType,
				"start":        startVal,
				"increment":    increment,
				"cycle":        isCycle,
				"owner_table":  ownerTable,
				"owner_column": ownerColumn,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sequence_audit rows error: %w", err)
	}

	return findings, nil
}
