// Check for pgEdge Snowflake extension — required for globally unique IDs in multi-master.
package extensions

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// SnowflakeExtCheck verifies whether the pgEdge Snowflake extension is available.
type SnowflakeExtCheck struct{}

func init() {
	check.Register(SnowflakeExtCheck{})
}

// Name returns the unique identifier for this check.
func (SnowflakeExtCheck) Name() string { return "snowflake_ext" }

// Category returns the check category.
func (SnowflakeExtCheck) Category() string { return "extensions" }

// Mode returns when this check runs (scan, audit, or both).
func (SnowflakeExtCheck) Mode() string { return "scan" }

// Description returns a human-readable summary of this check.
func (SnowflakeExtCheck) Description() string {
	return "pgEdge Snowflake extension availability for globally unique ID generation"
}

// Run executes the check against the database connection.
func (c SnowflakeExtCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	// Check if snowflake is installed.
	var extversion *string
	err := conn.QueryRow(ctx,
		`SELECT extversion FROM pg_catalog.pg_extension WHERE extname = 'snowflake';`,
	).Scan(&extversion)

	if err != nil || extversion == nil {
		// Check if snowflake is available but not installed.
		var available *string
		_ = conn.QueryRow(ctx,
			`SELECT default_version FROM pg_catalog.pg_available_extensions WHERE name = 'snowflake';`,
		).Scan(&available)

		if available != nil {
			return []models.Finding{{
				Severity:   models.SeverityConsider,
				CheckName:  c.Name(),
				Category:   c.Category(),
				Title:      fmt.Sprintf("Snowflake extension available (v%s) but not installed", *available),
				Detail:     fmt.Sprintf("The pgEdge Snowflake extension v%s is available on this server but not yet installed. Snowflake provides globally unique ID generation required for multi-master replication.", *available),
				ObjectName: "snowflake",
				Remediation: "Install the Snowflake extension:\n" +
					"  CREATE EXTENSION snowflake;\n" +
					"Then migrate sequences to use Snowflake for globally unique IDs.",
				Metadata: map[string]any{"available_version": *available},
			}}, nil
		}

		return []models.Finding{{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Snowflake extension not available",
			Detail:     "The pgEdge Snowflake extension is not available on this server. It will be available after installing pgEdge and is required for globally unique ID generation in multi-master setups.",
			ObjectName: "snowflake",
			Remediation: "Install pgEdge to obtain the Snowflake extension. " +
				"Snowflake provides globally unique sequence values across all cluster nodes.",
		}}, nil
	}

	return []models.Finding{{
		Severity:   models.SeverityInfo,
		CheckName:  c.Name(),
		Category:   c.Category(),
		Title:      fmt.Sprintf("Snowflake extension installed (v%s)", *extversion),
		Detail:     fmt.Sprintf("The pgEdge Snowflake extension v%s is installed. This provides globally unique ID generation for multi-master replication.", *extversion),
		ObjectName: "snowflake",
		Metadata:   map[string]any{"version": *extversion},
	}}, nil
}
