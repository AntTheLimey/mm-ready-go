package replication

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// MultipleDatabasesCheck flags when more than one user database exists.
type MultipleDatabasesCheck struct{}

func init() {
	check.Register(&MultipleDatabasesCheck{})
}

func (c *MultipleDatabasesCheck) Name() string     { return "multiple_databases" }
func (c *MultipleDatabasesCheck) Category() string  { return "replication" }
func (c *MultipleDatabasesCheck) Description() string {
	return "More than one user database in the instance — Spock supports one DB per instance"
}
func (c *MultipleDatabasesCheck) Mode() string { return "scan" }

func (c *MultipleDatabasesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	query := `
		SELECT datname
		FROM pg_catalog.pg_database
		WHERE datistemplate = false
		  AND datname NOT IN ('postgres')
		ORDER BY datname;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying pg_database: %w", err)
	}
	defer rows.Close()

	var dbNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("scanning database name: %w", err)
		}
		dbNames = append(dbNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating databases: %w", err)
	}

	if len(dbNames) > 1 {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Instance has %d user database(s): %s", len(dbNames), strings.Join(dbNames, ", ")),
			Detail: fmt.Sprintf(
				"Found %d non-template databases (excluding 'postgres'): %s. "+
					"pgEdge Spock officially supports one database per PostgreSQL instance. "+
					"Multiple databases may require separate instances for multi-master replication.",
				len(dbNames), strings.Join(dbNames, ", ")),
			ObjectName: "(instance)",
			Remediation: "Plan to separate databases into individual PostgreSQL instances, " +
				"one per database, for Spock multi-master replication.",
			Metadata: map[string]any{"databases": dbNames},
		}}, nil
	}

	return nil, nil
}
