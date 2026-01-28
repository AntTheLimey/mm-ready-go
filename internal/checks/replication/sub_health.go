package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// SubscriptionHealthCheck verifies health of Spock subscriptions.
type SubscriptionHealthCheck struct{}

func init() {
	check.Register(&SubscriptionHealthCheck{})
}

func (c *SubscriptionHealthCheck) Name() string     { return "subscription_health" }
func (c *SubscriptionHealthCheck) Category() string  { return "replication" }
func (c *SubscriptionHealthCheck) Description() string {
	return "Check health of Spock subscriptions"
}
func (c *SubscriptionHealthCheck) Mode() string { return "audit" }

func (c *SubscriptionHealthCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
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
			Title:      "Spock schema not found — skipping subscription health check",
			Detail:     "The spock schema does not exist in this database.",
			ObjectName: "spock",
		}}, nil
	}

	// Query subscription status
	query := `
		SELECT
			sub_name,
			sub_enabled,
			sub_slot_name,
			sub_replication_sets,
			sub_forward_origins
		FROM spock.subscription
		ORDER BY sub_name;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return []models.Finding{{
			Severity:   models.SeverityWarning,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Could not query spock.subscription",
			Detail:     fmt.Sprintf("Error querying subscriptions: %v", err),
			ObjectName: "spock.subscription",
		}}, nil
	}
	defer rows.Close()

	type subRow struct {
		name       string
		enabled    bool
		slotName   string
		repsets    []string
		fwdOrigins []string
	}

	var subs []subRow
	for rows.Next() {
		var s subRow
		if err := rows.Scan(&s.name, &s.enabled, &s.slotName, &s.repsets, &s.fwdOrigins); err != nil {
			return nil, fmt.Errorf("scanning subscription row: %w", err)
		}
		subs = append(subs, s)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating subscription rows: %w", err)
	}

	if len(subs) == 0 {
		return []models.Finding{{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "No Spock subscriptions found",
			Detail:     "This node has no Spock subscriptions configured.",
			ObjectName: "spock.subscription",
		}}, nil
	}

	var findings []models.Finding
	for _, s := range subs {
		if !s.enabled {
			findings = append(findings, models.Finding{
				Severity:  models.SeverityCritical,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("Subscription '%s' is DISABLED", s.name),
				Detail: fmt.Sprintf(
					"Subscription '%s' exists but is disabled. This means no data is "+
						"being replicated from the provider node through this subscription.",
					s.name),
				ObjectName: s.name,
				Remediation: fmt.Sprintf(
					"Re-enable the subscription:\n"+
						"  SELECT spock.alter_subscription_enable('%s');", s.name),
				Metadata: map[string]any{"slot_name": s.slotName},
			})
		}

		// Check replication slot health
		var active bool
		var restartLSN, flushLSN *string
		slotErr := conn.QueryRow(ctx, `
			SELECT active, restart_lsn::text, confirmed_flush_lsn::text
			FROM pg_replication_slots
			WHERE slot_name = $1;
		`, s.slotName).Scan(&active, &restartLSN, &flushLSN)

		if slotErr == nil && !active {
			meta := map[string]any{}
			if restartLSN != nil {
				meta["restart_lsn"] = *restartLSN
			}
			if flushLSN != nil {
				meta["flush_lsn"] = *flushLSN
			}
			findings = append(findings, models.Finding{
				Severity:  models.SeverityWarning,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("Replication slot '%s' is inactive", s.slotName),
				Detail: fmt.Sprintf(
					"Replication slot '%s' for subscription '%s' is not active. "+
						"This could indicate a connection issue with the provider node.",
					s.slotName, s.name),
				ObjectName:  s.slotName,
				Remediation: "Check network connectivity and provider node status.",
				Metadata:    meta,
			})
		}
	}

	return findings, nil
}
