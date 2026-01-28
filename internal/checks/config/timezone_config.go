// Check timezone configuration for commit timestamp consistency across nodes.
package config

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// TimezoneConfigCheck verifies that timezone settings are UTC for multi-master consistency.
type TimezoneConfigCheck struct{}

func init() {
	check.Register(TimezoneConfigCheck{})
}

func (TimezoneConfigCheck) Name() string        { return "timezone_config" }
func (TimezoneConfigCheck) Category() string     { return "config" }
func (TimezoneConfigCheck) Description() string  { return "Timezone settings — UTC recommended for consistent commit timestamps" }
func (TimezoneConfigCheck) Mode() string         { return "scan" }

func (c TimezoneConfigCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var tz string
	err := conn.QueryRow(ctx, "SHOW timezone;").Scan(&tz)
	if err != nil {
		return nil, fmt.Errorf("timezone query failed: %w", err)
	}

	var logTz string
	err = conn.QueryRow(ctx, "SHOW log_timezone;").Scan(&logTz)
	if err != nil {
		return nil, fmt.Errorf("log_timezone query failed: %w", err)
	}

	var findings []models.Finding

	if tz != "UTC" {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("timezone = '%s' (recommended: UTC)", tz),
			Detail: fmt.Sprintf(
				"Server timezone is set to '%s'. In multi-master replication, "+
					"Spock's last-update-wins conflict resolution relies on commit "+
					"timestamps (track_commit_timestamp). While PostgreSQL stores "+
					"timestamps in UTC internally, using UTC as the server timezone "+
					"avoids confusion in logs, monitoring, and debugging across nodes "+
					"in different geographic locations.", tz,
			),
			ObjectName: "timezone",
			Remediation: "Set all cluster nodes to UTC:\n" +
				"  ALTER SYSTEM SET timezone = 'UTC';\n" +
				"  SELECT pg_reload_conf();",
			Metadata: map[string]any{"current": tz},
		})
	}

	if logTz != "UTC" {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("log_timezone = '%s' (recommended: UTC)", logTz),
			Detail: fmt.Sprintf(
				"Log timezone is '%s'. Using UTC for log timestamps "+
					"across all nodes makes it easier to correlate events and "+
					"troubleshoot replication issues.", logTz,
			),
			ObjectName: "log_timezone",
			Remediation: "  ALTER SYSTEM SET log_timezone = 'UTC';\n" +
				"  SELECT pg_reload_conf();",
			Metadata: map[string]any{"current": logTz},
		})
	}

	if tz == "UTC" && logTz == "UTC" {
		findings = append(findings, models.Finding{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Timezone and log_timezone are both UTC (OK)",
			Detail:     "Both timezone settings are UTC, which is the recommended configuration for multi-master clusters.",
			ObjectName: "timezone",
		})
	}

	return findings, nil
}
