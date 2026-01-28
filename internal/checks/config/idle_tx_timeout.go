// Check idle-in-transaction session timeout configuration.
package config

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// IdleTransactionTimeoutCheck checks whether idle transaction timeouts are configured.
type IdleTransactionTimeoutCheck struct{}

func init() {
	check.Register(IdleTransactionTimeoutCheck{})
}

func (IdleTransactionTimeoutCheck) Name() string     { return "idle_transaction_timeout" }
func (IdleTransactionTimeoutCheck) Category() string  { return "config" }
func (IdleTransactionTimeoutCheck) Description() string {
	return "Idle-in-transaction timeout — long idle transactions block VACUUM and cause bloat"
}
func (IdleTransactionTimeoutCheck) Mode() string { return "scan" }

func (c IdleTransactionTimeoutCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var findings []models.Finding

	// Check idle_in_transaction_session_timeout
	var idleTxTimeout string
	err := conn.QueryRow(ctx, "SHOW idle_in_transaction_session_timeout;").Scan(&idleTxTimeout)
	if err != nil {
		return nil, fmt.Errorf("idle_in_transaction_session_timeout query failed: %w", err)
	}

	if idleTxTimeout == "0" {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "idle_in_transaction_session_timeout is not set",
			Detail: "idle_in_transaction_session_timeout is disabled (0). " +
				"Connections that remain idle in an open transaction hold " +
				"transaction IDs (XIDs) that prevent VACUUM from reclaiming " +
				"dead tuples, leading to table bloat. In replication " +
				"environments this is amplified because bloat on any node " +
				"affects replication performance.",
			ObjectName: "idle_in_transaction_session_timeout",
			Remediation: "Set a reasonable timeout (e.g. 5 minutes):\n" +
				"  ALTER SYSTEM SET idle_in_transaction_session_timeout = '5min';\n" +
				"  SELECT pg_reload_conf();",
			Metadata: map[string]any{"current": idleTxTimeout},
		})
	}

	// idle_session_timeout added in PG14 — may not exist on older versions
	var idleSessionTimeout string
	err = conn.QueryRow(ctx, "SHOW idle_session_timeout;").Scan(&idleSessionTimeout)
	if err != nil {
		// Parameter does not exist on this PG version; skip gracefully
		return findings, nil
	}

	if idleSessionTimeout == "0" {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "idle_session_timeout is not set",
			Detail: "idle_session_timeout is disabled (0). Idle connections " +
				"consume backend slots and shared memory. In a multi-master " +
				"cluster, connection pool exhaustion on any node can cause " +
				"replication apply workers to stall.",
			ObjectName: "idle_session_timeout",
			Remediation: "Consider setting a session timeout for non-interactive connections:\n" +
				"  ALTER SYSTEM SET idle_session_timeout = '30min';\n" +
				"  SELECT pg_reload_conf();",
			Metadata: map[string]any{"current": idleSessionTimeout},
		})
	}

	return findings, nil
}
