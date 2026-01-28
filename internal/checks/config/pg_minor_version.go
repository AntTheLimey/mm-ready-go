// Audit check: report PostgreSQL minor version for cross-node consistency.
package config

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// PgMinorVersionCheck reports the PostgreSQL minor version so all cluster nodes can be compared.
type PgMinorVersionCheck struct{}

func init() {
	check.Register(PgMinorVersionCheck{})
}

func (PgMinorVersionCheck) Name() string        { return "pg_minor_version" }
func (PgMinorVersionCheck) Category() string     { return "config" }
func (PgMinorVersionCheck) Description() string  { return "PostgreSQL minor version — all cluster nodes should match" }
func (PgMinorVersionCheck) Mode() string         { return "audit" }

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
