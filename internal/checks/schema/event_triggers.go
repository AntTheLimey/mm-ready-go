// Check for event triggers — fire on DDL events, interact with Spock DDL replication.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// EventTriggersCheck finds event triggers and evaluates their replication impact.
type EventTriggersCheck struct{}

func init() {
	check.Register(EventTriggersCheck{})
}

func (EventTriggersCheck) Name() string     { return "event_triggers" }
func (EventTriggersCheck) Category() string  { return "schema" }
func (EventTriggersCheck) Mode() string      { return "scan" }
func (EventTriggersCheck) Description() string {
	return "Event triggers — fire on DDL events, may interact with Spock DDL replication"
}

func (c EventTriggersCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			evtname AS trigger_name,
			evtevent AS event,
			evtenabled::text AS enabled
		FROM pg_catalog.pg_event_trigger
		ORDER BY evtname;
	`

	enabledLabels := map[string]string{
		"O": "origin/local",
		"D": "disabled",
		"R": "replica",
		"A": "always",
	}

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("event_triggers query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var triggerName, event, enabled string
		if err := rows.Scan(&triggerName, &event, &enabled); err != nil {
			return nil, fmt.Errorf("event_triggers scan failed: %w", err)
		}

		if enabled == "D" {
			continue // Skip disabled triggers
		}

		label := enabledLabels[enabled]
		if label == "" {
			label = enabled
		}

		var severity models.Severity
		var detail, remediation string

		switch enabled {
		case "A":
			severity = models.SeverityConsider
			detail = fmt.Sprintf(
				"Event trigger '%s' fires on '%s' events "+
					"(enabled mode: %s). Spock's apply worker runs with "+
					"session_replication_role='replica' (spock_apply.c:3742), so "+
					"only ENABLE ALWAYS triggers fire during replication apply.\n\n"+
					"If this trigger is used for DDL automation (e.g. automatically "+
					"adding new tables to replication sets), ENABLE ALWAYS is the "+
					"CORRECT and REQUIRED setting. If this trigger has side effects "+
					"that should only run once (e.g. sending notifications, writing "+
					"audit logs), it should NOT be ENABLE ALWAYS.",
				triggerName, event, label,
			)
			remediation = fmt.Sprintf(
				"If this trigger automates replication set management, ENABLE "+
					"ALWAYS is correct — no change needed. If it has side effects "+
					"that should not fire during replication apply, change to "+
					"'origin' mode: ALTER EVENT TRIGGER %s ENABLE;",
				triggerName,
			)
		case "R":
			severity = models.SeverityWarning
			detail = fmt.Sprintf(
				"Event trigger '%s' fires on '%s' events "+
					"(enabled mode: %s). ENABLE REPLICA triggers fire when "+
					"session_replication_role='replica', which is the mode Spock's "+
					"apply worker uses. This trigger WILL fire during replication apply.",
				triggerName, event, label,
			)
			remediation = fmt.Sprintf(
				"Review whether this trigger should fire during replication "+
					"apply. If not, set to origin mode: ALTER EVENT TRIGGER "+
					"%s ENABLE;",
				triggerName,
			)
		default:
			severity = models.SeverityInfo
			detail = fmt.Sprintf(
				"Event trigger '%s' fires on '%s' events "+
					"(enabled mode: %s). Origin-mode triggers only fire for "+
					"locally-originated DDL, not replicated DDL. This is the default "+
					"and generally correct setting.",
				triggerName, event, label,
			)
			remediation = ""
		}

		findings = append(findings, models.Finding{
			Severity:    severity,
			CheckName:   c.Name(),
			Category:    c.Category(),
			Title:       fmt.Sprintf("Event trigger '%s' on %s (enabled: %s)", triggerName, event, label),
			Detail:      detail,
			ObjectName:  triggerName,
			Remediation: remediation,
			Metadata:    map[string]any{"event": event, "enabled": enabled},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("event_triggers rows iteration failed: %w", err)
	}
	return findings, nil
}
