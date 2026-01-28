// Audit check: verify key Spock GUC settings.
package config

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// SpockGucsCheck verifies key Spock configuration parameters (GUCs).
type SpockGucsCheck struct{}

func init() {
	check.Register(SpockGucsCheck{})
}

func (SpockGucsCheck) Name() string        { return "spock_gucs" }
func (SpockGucsCheck) Category() string     { return "config" }
func (SpockGucsCheck) Description() string  { return "Verify key Spock configuration parameters (GUCs)" }
func (SpockGucsCheck) Mode() string         { return "audit" }

type gucSpec struct {
	name        string
	recommended string
	severity    models.Severity
	detail      string
}

var spockGUCs = []gucSpec{
	{
		name:        "spock.conflict_resolution",
		recommended: "last_update_wins",
		severity:    models.SeverityWarning,
		detail: "Controls how Spock resolves UPDATE/UPDATE conflicts. " +
			"'last_update_wins' uses commit timestamps (requires " +
			"track_commit_timestamp=on) to keep the most recent change.",
	},
	{
		name:        "spock.save_resolutions",
		recommended: "on",
		severity:    models.SeverityInfo,
		detail: "When enabled, conflict resolutions are logged to " +
			"spock.conflict_history for analysis.",
	},
	{
		name:        "spock.enable_ddl_replication",
		recommended: "on",
		severity:    models.SeverityWarning,
		detail: "Controls whether DDL statements are automatically captured " +
			"and replicated (AutoDDL). When enabled, DDL classified as " +
			"LOGSTMT_DDL by PostgreSQL is intercepted and sent to " +
			"subscribers. Note: TRUNCATE, VACUUM, and ANALYZE are NOT " +
			"captured by AutoDDL regardless of this setting.",
	},
	{
		name:        "spock.include_ddl_repset",
		recommended: "on",
		severity:    models.SeverityInfo,
		detail: "When enabled alongside enable_ddl_replication, tables created " +
			"via DDL are automatically added to the appropriate replication " +
			"set (default for tables with PKs, default_insert_only otherwise).",
	},
	{
		name:        "spock.allow_ddl_from_functions",
		recommended: "on",
		severity:    models.SeverityInfo,
		detail: "When enabled, DDL executed inside functions and procedures is " +
			"also captured by AutoDDL. Without this, only top-level DDL " +
			"statements are replicated.",
	},
}

func (c SpockGucsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var findings []models.Finding

	for _, guc := range spockGUCs {
		var value string
		err := conn.QueryRow(ctx, "SELECT current_setting($1);", guc.name).Scan(&value)
		if err != nil {
			// GUC not available — Spock may not be loaded
			findings = append(findings, models.Finding{
				Severity:  models.SeverityInfo,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("GUC '%s' not available", guc.name),
				Detail: fmt.Sprintf(
					"Could not read '%s'. Spock may not be "+
						"loaded in shared_preload_libraries.", guc.name,
				),
				ObjectName: guc.name,
			})
			continue
		}

		if value != guc.recommended {
			findings = append(findings, models.Finding{
				Severity:  guc.severity,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("%s = '%s' (recommended: '%s')", guc.name, value, guc.recommended),
				Detail:    fmt.Sprintf("%s\n\nCurrent value: '%s'.", guc.detail, value),
				ObjectName: guc.name,
				Remediation: fmt.Sprintf(
					"Consider setting:\n  ALTER SYSTEM SET %s = '%s';",
					guc.name, guc.recommended,
				),
				Metadata: map[string]any{"current": value, "recommended": guc.recommended},
			})
		} else {
			findings = append(findings, models.Finding{
				Severity:   models.SeverityInfo,
				CheckName:  c.Name(),
				Category:   c.Category(),
				Title:      fmt.Sprintf("%s = '%s' (OK)", guc.name, value),
				Detail:     guc.detail,
				ObjectName: guc.name,
				Metadata:   map[string]any{"current": value},
			})
		}
	}

	return findings, nil
}
