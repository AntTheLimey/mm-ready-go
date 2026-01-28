// Package replication contains checks related to WAL, slots, workers, and Spock replication health.
package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// WalLevelCheck verifies that wal_level is set to 'logical'.
type WalLevelCheck struct{}

func init() {
	check.Register(&WalLevelCheck{})
}

func (c *WalLevelCheck) Name() string        { return "wal_level" }
func (c *WalLevelCheck) Category() string     { return "replication" }
func (c *WalLevelCheck) Description() string  { return "wal_level must be 'logical' for Spock replication" }
func (c *WalLevelCheck) Mode() string         { return "scan" }

func (c *WalLevelCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var walLevel string
	err := conn.QueryRow(ctx, "SHOW wal_level;").Scan(&walLevel)
	if err != nil {
		return nil, fmt.Errorf("querying wal_level: %w", err)
	}

	if walLevel != "logical" {
		return []models.Finding{{
			Severity:  models.SeverityCritical,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("wal_level is '%s' — must be 'logical'", walLevel),
			Detail: fmt.Sprintf(
				"Current wal_level is '%s'. Spock requires wal_level = 'logical' to enable "+
					"logical decoding of the write-ahead log. This is a PostgreSQL server "+
					"setting that should be configured before installing Spock.", walLevel),
			ObjectName: "wal_level",
			Remediation: "Configure before installing Spock:\n" +
				"  ALTER SYSTEM SET wal_level = 'logical';\n" +
				"Then restart PostgreSQL. No Spock installation is needed " +
				"for this change — it is a standard PostgreSQL setting.",
			Metadata: map[string]any{"current_value": walLevel},
		}}, nil
	}

	return nil, nil
}
