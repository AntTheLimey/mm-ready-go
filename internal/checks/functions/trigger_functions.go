// Check triggers that may conflict with replication.
package functions

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// TriggerFunctionsCheck audits triggers for replication firing behaviour.
type TriggerFunctionsCheck struct{}

func init() {
	check.Register(TriggerFunctionsCheck{})
}

func (TriggerFunctionsCheck) Name() string        { return "trigger_functions" }
func (TriggerFunctionsCheck) Category() string     { return "functions" }
func (TriggerFunctionsCheck) Mode() string         { return "scan" }
func (TriggerFunctionsCheck) Description() string {
	return "Triggers — ENABLE REPLICA and ENABLE ALWAYS both fire during Spock apply"
}

var enabledLabels = map[string]string{
	"O": "ORIGIN (fires on non-replica sessions)",
	"D": "DISABLED",
	"R": "REPLICA (fires during replica apply)",
	"A": "ALWAYS (fires in all sessions)",
}

func (c TriggerFunctionsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			t.tgname AS trigger_name,
			CASE t.tgtype & 66
				WHEN 2 THEN 'BEFORE'
				WHEN 64 THEN 'INSTEAD OF'
				ELSE 'AFTER'
			END AS timing,
			CASE
				WHEN t.tgtype & 4 > 0 THEN 'INSERT'
				WHEN t.tgtype & 8 > 0 THEN 'DELETE'
				WHEN t.tgtype & 16 > 0 THEN 'UPDATE'
				WHEN t.tgtype & 32 > 0 THEN 'TRUNCATE'
				ELSE 'UNKNOWN'
			END AS event,
			pn.nspname || '.' || p.proname AS func_name,
			t.tgenabled::text AS enabled
		FROM pg_catalog.pg_trigger t
		JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_proc p ON p.oid = t.tgfoid
		JOIN pg_catalog.pg_namespace pn ON pn.oid = p.pronamespace
		WHERE NOT t.tgisinternal
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname, t.tgname;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("trigger_functions query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, trigName, timing, event, funcName, enabled string
		if err := rows.Scan(&schemaName, &tableName, &trigName, &timing, &event, &funcName, &enabled); err != nil {
			return nil, fmt.Errorf("trigger_functions scan failed: %w", err)
		}

		fqn := fmt.Sprintf("%s.%s", schemaName, tableName)
		enabledLabel := enabledLabels[enabled]
		if enabledLabel == "" {
			enabledLabel = enabled
		}

		var sev models.Severity
		var concern string
		remediation := ""

		// Spock apply workers run with session_replication_role='replica'
		// (confirmed: spock_apply.c:3742). Both ENABLE REPLICA and ENABLE ALWAYS
		// triggers fire during apply. ORIGIN-mode triggers do NOT fire during apply.
		switch enabled {
		case "A":
			sev = models.SeverityWarning
			concern = "This trigger fires ALWAYS — it WILL fire on subscriber nodes when " +
				"Spock applies replicated changes. The trigger function will execute " +
				"on both the originating node and all subscriber nodes, which may " +
				"cause duplicate side effects or conflicts."
			remediation = "For most triggers, ORIGIN mode (default 'O') is correct — it only " +
				"fires on the node where the write originates. Use ENABLE REPLICA or " +
				"ENABLE ALWAYS only when the trigger must also fire during replication apply."
		case "R":
			sev = models.SeverityWarning
			concern = "This trigger fires in REPLICA mode — it WILL fire on subscriber nodes " +
				"when Spock applies replicated changes (Spock apply workers run with " +
				"session_replication_role='replica'). Review the trigger function for " +
				"side effects that should not occur during replication apply."
			remediation = "For most triggers, ORIGIN mode (default 'O') is correct — it only " +
				"fires on the node where the write originates. Use ENABLE REPLICA or " +
				"ENABLE ALWAYS only when the trigger must also fire during replication apply."
		case "O":
			sev = models.SeverityInfo
			concern = "This trigger fires on ORIGIN only (default). It will NOT fire when " +
				"Spock applies replicated changes on subscriber nodes."
		case "D":
			sev = models.SeverityInfo
			concern = "This trigger is DISABLED."
		default:
			sev = models.SeverityInfo
			concern = fmt.Sprintf("Trigger enabled mode: %s.", enabledLabel)
		}

		findings = append(findings, models.Finding{
			Severity:    sev,
			CheckName:   c.Name(),
			Category:    c.Category(),
			Title:       fmt.Sprintf("Trigger '%s' on '%s' (%s %s, %s)", trigName, fqn, timing, event, enabledLabel),
			Detail:      fmt.Sprintf("Trigger '%s' calls %s. %s", trigName, funcName, concern),
			ObjectName:  fmt.Sprintf("%s.%s", fqn, trigName),
			Remediation: remediation,
			Metadata: map[string]any{
				"timing":   timing,
				"event":    event,
				"function": funcName,
				"enabled":  enabled,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("trigger_functions rows error: %w", err)
	}

	return findings, nil
}
