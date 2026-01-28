package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// MaxWalSendersCheck verifies sufficient WAL senders for Spock replication.
type MaxWalSendersCheck struct{}

func init() {
	check.Register(&MaxWalSendersCheck{})
}

func (c *MaxWalSendersCheck) Name() string     { return "max_wal_senders" }
func (c *MaxWalSendersCheck) Category() string  { return "replication" }
func (c *MaxWalSendersCheck) Description() string {
	return "Sufficient max_wal_senders for Spock logical replication"
}
func (c *MaxWalSendersCheck) Mode() string { return "scan" }

func (c *MaxWalSendersCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	query := `
		SELECT
			current_setting('max_wal_senders')::int AS max_senders,
			(SELECT count(*) FROM pg_stat_replication) AS active_senders;
	`
	var maxSenders, activeSenders int
	err := conn.QueryRow(ctx, query).Scan(&maxSenders, &activeSenders)
	if err != nil {
		return nil, fmt.Errorf("querying max_wal_senders: %w", err)
	}

	if maxSenders < 10 {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("max_wal_senders is %d (recommend >= 10)", maxSenders),
			Detail: fmt.Sprintf(
				"max_wal_senders is set to %d with %d currently active. Each Spock "+
					"subscription requires a WAL sender process. In a multi-master "+
					"topology with N nodes, each node needs at least N-1 senders plus "+
					"headroom for initial sync and backups.", maxSenders, activeSenders),
			ObjectName: "max_wal_senders",
			Remediation: "Increase max_wal_senders to at least 10:\n" +
				"  ALTER SYSTEM SET max_wal_senders = 10;\n" +
				"Requires a PostgreSQL restart.",
			Metadata: map[string]any{
				"current": maxSenders,
				"active":  activeSenders,
			},
		}}, nil
	}

	return nil, nil
}
