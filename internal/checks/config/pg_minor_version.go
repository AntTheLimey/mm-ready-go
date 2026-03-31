// Audit check: report PostgreSQL minor version for cross-node consistency.
package config

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// PgMinorVersionCheck reports the PostgreSQL minor version so all cluster nodes can be compared.
type PgMinorVersionCheck struct{}

func init() {
	check.Register(PgMinorVersionCheck{})
}

// Name returns the unique identifier for this check.
func (PgMinorVersionCheck) Name() string { return "pg_minor_version" }

// Category returns the check category.
func (PgMinorVersionCheck) Category() string { return "config" }

// Description returns a human-readable summary of this check.
func (PgMinorVersionCheck) Description() string {
	return "PostgreSQL minor version — all cluster nodes should match"
}

// Mode returns when this check runs (scan, audit, or both).
func (PgMinorVersionCheck) Mode() string { return "audit" }

// Run executes the check against the database connection.
func (c PgMinorVersionCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var fullVersion, serverVersion string
	err := conn.QueryRow(ctx, "SELECT version(), current_setting('server_version');").Scan(&fullVersion, &serverVersion)
	if err != nil {
		return nil, fmt.Errorf("pg_minor_version query failed: %w", err)
	}

	return []models.Finding{{
		Severity:  models.SeverityConsider,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("PostgreSQL %s", serverVersion),
		Detail: fmt.Sprintf(
			"Server version: %s\nFull version string: %s\n\n"+
				"All nodes in a Spock cluster should run the same PostgreSQL "+
				"minor version. Minor version mismatches can introduce subtle "+
				"behavioral differences and complicate troubleshooting. Verify "+
				"this version matches all other cluster nodes.",
			serverVersion, fullVersion,
		),
		ObjectName: "pg_version",
		Remediation: "Ensure all cluster nodes are upgraded to the same minor version " +
			"during maintenance windows. Apply minor upgrades to all nodes " +
			"before resuming normal operation.",
		Metadata: map[string]any{"server_version": serverVersion},
	}}, nil
}
