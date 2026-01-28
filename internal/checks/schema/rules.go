// Check for rules on tables — rules interact poorly with logical replication.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// RulesCheck finds rules on tables that may cause unexpected behaviour with replication.
type RulesCheck struct{}

func init() {
	check.Register(RulesCheck{})
}

func (RulesCheck) Name() string     { return "rules" }
func (RulesCheck) Category() string  { return "schema" }
func (RulesCheck) Mode() string      { return "scan" }
func (RulesCheck) Description() string {
	return "Rules on tables — can cause unexpected behaviour with logical replication"
}

func (c RulesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			r.rulename AS rule_name,
			r.ev_type::text AS event_type,
			r.is_instead AS is_instead
		FROM pg_catalog.pg_rewrite r
		JOIN pg_catalog.pg_class c ON c.oid = r.ev_class
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND r.rulename != '_RETURN'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname, r.rulename;
	`

	eventLabels := map[string]string{
		"1": "SELECT",
		"2": "UPDATE",
		"3": "INSERT",
		"4": "DELETE",
	}

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("rules query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName, ruleName, eventType string
		var isInstead bool
		if err := rows.Scan(&schemaName, &tableName, &ruleName, &eventType, &isInstead); err != nil {
			return nil, fmt.Errorf("rules scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		event := eventLabels[eventType]
		if event == "" {
			event = eventType
		}

		severity := models.SeverityConsider
		if isInstead {
			severity = models.SeverityWarning
		}

		insteadPrefix := ""
		if isInstead {
			insteadPrefix = "INSTEAD "
		}

		articleStr := "a"
		if isInstead {
			articleStr = "an INSTEAD"
		}

		detail := fmt.Sprintf(
			"Table '%s' has %s rule "+
				"'%s' on %s events. "+
				"Rules rewrite queries before execution, which means the WAL "+
				"records the rewritten operations, not the original SQL. On the "+
				"subscriber side, the Spock apply worker replays the row-level "+
				"changes from WAL, and the subscriber's rules will also fire on "+
				"the applied changes — potentially causing double-application or "+
				"unexpected side effects.",
			fqn, articleStr, ruleName, event,
		)
		if isInstead {
			detail += " INSTEAD rules are particularly dangerous as they completely " +
				"replace the original operation."
		}

		findings = append(findings, models.Finding{
			Severity:  severity,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("%sRule '%s' on '%s' (%s)", insteadPrefix, ruleName, fqn, event),
			Detail:    detail,
			ObjectName: fmt.Sprintf("%s.%s", fqn, ruleName),
			Remediation: "Consider converting rules to triggers (which can be controlled " +
				"via session_replication_role), or disable rules on subscriber " +
				"nodes. Review whether the rule's effect should apply on both " +
				"provider and subscriber.",
			Metadata: map[string]any{"event": event, "is_instead": isInstead},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rules rows iteration failed: %w", err)
	}
	return findings, nil
}
