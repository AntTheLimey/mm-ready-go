// Check sequence data types — smallint/integer sequences may overflow in multi-master.
package sequences

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// SequenceDataTypesCheck flags smallint/integer sequences that may overflow faster in multi-master.
type SequenceDataTypesCheck struct{}

func init() {
	check.Register(SequenceDataTypesCheck{})
}

func (SequenceDataTypesCheck) Name() string        { return "sequence_data_types" }
func (SequenceDataTypesCheck) Category() string     { return "sequences" }
func (SequenceDataTypesCheck) Mode() string         { return "scan" }
func (SequenceDataTypesCheck) Description() string {
	return "Sequence data types — smallint/integer may overflow faster in multi-master"
}

func (c SequenceDataTypesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS seq_name,
			s.seqtypid::regtype::text AS data_type,
			s.seqmax AS max_value,
			s.seqstart AS start_value,
			s.seqincrement AS increment
		FROM pg_catalog.pg_sequence s
		JOIN pg_catalog.pg_class c ON c.oid = s.seqrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("sequence_data_types query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var (
			schemaName, seqName, dataType string
			maxValue, startValue          int64
			increment                     int64
		)
		if err := rows.Scan(&schemaName, &seqName, &dataType, &maxValue, &startValue, &increment); err != nil {
			return nil, fmt.Errorf("sequence_data_types scan failed: %w", err)
		}

		if dataType != "smallint" && dataType != "integer" {
			continue
		}

		fqn := fmt.Sprintf("%s.%s", schemaName, seqName)
		var typeMax int64
		if dataType == "smallint" {
			typeMax = 32767
		} else {
			typeMax = 2147483647
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Sequence '%s' uses %s (max %d)", fqn, dataType, typeMax),
			Detail: fmt.Sprintf(
				"Sequence '%s' is defined as %s with max value %d. "+
					"In a multi-master setup with pgEdge Snowflake sequences, the ID space "+
					"is partitioned across nodes and includes a node identifier component. "+
					"Smaller integer types can exhaust their range much faster. "+
					"Consider upgrading to bigint.",
				fqn, dataType, maxValue,
			),
			ObjectName: fqn,
			Remediation: "Alter the column and sequence to use bigint:\n" +
				"  ALTER TABLE ... ALTER COLUMN ... TYPE bigint;\n" +
				"This allows room for Snowflake-style globally unique IDs.",
			Metadata: map[string]any{
				"data_type": dataType,
				"max_value": maxValue,
				"increment": increment,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("sequence_data_types rows error: %w", err)
	}

	return findings, nil
}
