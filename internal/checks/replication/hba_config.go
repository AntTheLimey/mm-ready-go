package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// HbaConfigCheck verifies pg_hba.conf has replication connection entries.
type HbaConfigCheck struct{}

func init() {
	check.Register(&HbaConfigCheck{})
}

func (c *HbaConfigCheck) Name() string        { return "hba_config" }
func (c *HbaConfigCheck) Category() string     { return "replication" }
func (c *HbaConfigCheck) Description() string {
	return "pg_hba.conf must allow replication connections between nodes"
}
func (c *HbaConfigCheck) Mode() string { return "scan" }

func (c *HbaConfigCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	query := `
		SELECT
			line_number, type, database, user_name, address, netmask, auth_method
		FROM pg_catalog.pg_hba_file_rules
		ORDER BY line_number;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		// pg_hba_file_rules requires superuser or pg_read_all_settings; may also
		// not exist on older PostgreSQL versions.
		return []models.Finding{{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "Could not read pg_hba_file_rules",
			Detail: "Unable to query pg_hba_file_rules. This view requires superuser " +
				"or pg_read_all_settings privilege, and is available in PostgreSQL 15+.",
			ObjectName:  "pg_hba.conf",
			Remediation: "Manually verify pg_hba.conf allows replication connections.",
		}}, nil
	}
	defer rows.Close()

	replicationCount := 0
	for rows.Next() {
		var lineNumber int
		var hbaType string
		var databases []string
		var userName []string
		var address, netmask, authMethod *string
		if err := rows.Scan(&lineNumber, &hbaType, &databases, &userName, &address, &netmask, &authMethod); err != nil {
			return nil, fmt.Errorf("scanning pg_hba_file_rules row: %w", err)
		}
		for _, db := range databases {
			if db == "replication" {
				replicationCount++
				break
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pg_hba_file_rules: %w", err)
	}

	if replicationCount == 0 {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "No replication entries found in pg_hba.conf",
			Detail: "No pg_hba.conf rules were found granting access to the 'replication' " +
				"database. Spock requires replication connections between nodes.",
			ObjectName: "pg_hba.conf",
			Remediation: "Add replication entries to pg_hba.conf, e.g.:\n" +
				"host replication spock_user 0.0.0.0/0 scram-sha-256",
		}}, nil
	}

	return []models.Finding{{
		Severity:  models.SeverityConsider,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("Found %d replication entry/entries in pg_hba.conf", replicationCount),
		Detail: fmt.Sprintf(
			"pg_hba.conf has %d replication access rule(s). "+
				"Verify these allow connections from all Spock peer nodes.", replicationCount),
		ObjectName:  "pg_hba.conf",
		Remediation: "Ensure all peer node IPs are covered by replication rules.",
		Metadata:    map[string]any{"entry_count": replicationCount},
	}}, nil
}
