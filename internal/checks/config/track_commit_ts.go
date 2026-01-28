// Check track_commit_timestamp is enabled.
package config

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// TrackCommitTimestampCheck verifies that track_commit_timestamp is on.
type TrackCommitTimestampCheck struct{}

func init() {
	check.Register(TrackCommitTimestampCheck{})
}

func (TrackCommitTimestampCheck) Name() string     { return "track_commit_timestamp" }
func (TrackCommitTimestampCheck) Category() string  { return "config" }
func (TrackCommitTimestampCheck) Description() string {
	return "track_commit_timestamp must be on for Spock conflict resolution"
}
func (TrackCommitTimestampCheck) Mode() string { return "scan" }

func (c TrackCommitTimestampCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var val string
	err := conn.QueryRow(ctx, "SHOW track_commit_timestamp;").Scan(&val)
	if err != nil {
		return nil, fmt.Errorf("track_commit_timestamp query failed: %w", err)
	}

	if val != "on" {
		return []models.Finding{{
			Severity:  models.SeverityCritical,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("track_commit_timestamp is '%s' — must be 'on'", val),
			Detail: fmt.Sprintf(
				"track_commit_timestamp = '%s'. Spock uses commit "+
					"timestamps for last-update-wins conflict resolution. This "+
					"is a PostgreSQL server setting that should be configured "+
					"before installing Spock.", val,
			),
			ObjectName: "track_commit_timestamp",
			Remediation: "Configure before installing Spock:\n" +
				"  ALTER SYSTEM SET track_commit_timestamp = on;\n" +
				"Then restart PostgreSQL. No Spock installation is needed " +
				"for this change — it is a standard PostgreSQL setting.",
			Metadata: map[string]any{"current_value": val},
		}}, nil
	}

	return nil, nil
}
