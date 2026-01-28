// Audit views for replication considerations.
package functions

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// ViewsAuditCheck audits views for write-capable views and materialized views.
type ViewsAuditCheck struct{}

func init() {
	check.Register(ViewsAuditCheck{})
}

func (ViewsAuditCheck) Name() string        { return "views_audit" }
func (ViewsAuditCheck) Category() string     { return "functions" }
func (ViewsAuditCheck) Mode() string         { return "scan" }
func (ViewsAuditCheck) Description() string {
	return "Audit views — updatable views and materialized views have replication considerations"
}

func (c ViewsAuditCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	// Check for materialized views.
	const matviewQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS view_name,
			pg_catalog.pg_get_viewdef(c.oid, true) AS definition
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'm'
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname;
	`
	rows, err := conn.Query(ctx, matviewQuery)
	if err != nil {
		return nil, fmt.Errorf("views_audit matview query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, viewName, definition string
		if err := rows.Scan(&schemaName, &viewName, &definition); err != nil {
			return nil, fmt.Errorf("views_audit matview scan failed: %w", err)
		}
		fqn := fmt.Sprintf("%s.%s", schemaName, viewName)
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Materialized view '%s'", fqn),
			Detail: fmt.Sprintf(
				"Materialized view '%s' stores data physically. REFRESH MATERIALIZED VIEW "+
					"is not replicated by Spock — it must be run independently on each node. "+
					"The underlying query results may also differ across nodes if they "+
					"reference node-local data.",
				fqn,
			),
			ObjectName: fqn,
			Remediation: "Schedule REFRESH MATERIALIZED VIEW independently on each node, " +
				"or consider replacing with a regular view if real-time data is acceptable.",
			Metadata: map[string]any{"type": "materialized"},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("views_audit matview rows error: %w", err)
	}

	// Check for views with INSTEAD OF triggers (updatable views).
	const trigViewQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS view_name,
			t.tgname AS trigger_name
		FROM pg_catalog.pg_trigger t
		JOIN pg_catalog.pg_class c ON c.oid = t.tgrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'v'
		  AND t.tgtype & 64 > 0
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname;
	`
	trigRows, err := conn.Query(ctx, trigViewQuery)
	if err != nil {
		return nil, fmt.Errorf("views_audit trigger view query failed: %w", err)
	}
	defer trigRows.Close()

	for trigRows.Next() {
		var schemaName, viewName, trigName string
		if err := trigRows.Scan(&schemaName, &viewName, &trigName); err != nil {
			return nil, fmt.Errorf("views_audit trigger view scan failed: %w", err)
		}
		fqn := fmt.Sprintf("%s.%s", schemaName, viewName)
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Updatable view '%s' (INSTEAD OF trigger: %s)", fqn, trigName),
			Detail: fmt.Sprintf(
				"View '%s' has INSTEAD OF trigger '%s'. Writes through this view are "+
					"handled by the trigger, which modifies underlying tables. The underlying "+
					"table changes are replicated, but the trigger itself may have side effects "+
					"that do not replicate.",
				fqn, trigName,
			),
			ObjectName: fqn,
			Remediation: "Review the INSTEAD OF trigger function for side effects. " +
				"Ensure underlying table writes are the only meaningful operations.",
			Metadata: map[string]any{
				"type":         "updatable_view",
				"trigger_name": trigName,
			},
		})
	}
	if err := trigRows.Err(); err != nil {
		return nil, fmt.Errorf("views_audit trigger rows error: %w", err)
	}

	// Summary: count all user views.
	var viewCount int
	err = conn.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind IN ('v', 'm')
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast');
	`).Scan(&viewCount)
	if err != nil {
		return nil, fmt.Errorf("views_audit count query failed: %w", err)
	}

	if viewCount > 0 {
		kinds := []string{}
		matCount := 0
		for _, f := range findings {
			if m, ok := f.Metadata["type"]; ok && m == "materialized" {
				matCount++
			}
		}
		if matCount > 0 {
			kinds = append(kinds, fmt.Sprintf("%d materialized", matCount))
		}
		regularCount := viewCount - matCount
		if regularCount > 0 {
			kinds = append(kinds, fmt.Sprintf("%d regular", regularCount))
		}
		findings = append(findings, models.Finding{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      fmt.Sprintf("Found %d user view(s) (%s)", viewCount, strings.Join(kinds, ", ")),
			Detail:     fmt.Sprintf("Audited %d views across all user schemas.", viewCount),
			ObjectName: "(views)",
			Metadata:   map[string]any{"total_count": viewCount},
		})
	}

	return findings, nil
}
