// Check installed extension versions against available upgrades.
package extensions

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// ExtensionVersionsCheck compares installed extension versions to available versions.
type ExtensionVersionsCheck struct{}

func init() {
	check.Register(ExtensionVersionsCheck{})
}

func (ExtensionVersionsCheck) Name() string        { return "extension_versions" }
func (ExtensionVersionsCheck) Category() string     { return "extensions" }
func (ExtensionVersionsCheck) Mode() string         { return "scan" }
func (ExtensionVersionsCheck) Description() string {
	return "Check installed extension versions against available upgrades"
}

func (c ExtensionVersionsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT
			e.extname,
			e.extversion AS installed_version,
			a.default_version AS available_version
		FROM pg_catalog.pg_extension e
		JOIN pg_catalog.pg_available_extensions a ON a.name = e.extname
		WHERE e.extversion <> a.default_version
		ORDER BY e.extname;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("extension_versions query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var extname, installed, available string
		if err := rows.Scan(&extname, &installed, &available); err != nil {
			return nil, fmt.Errorf("extension_versions scan failed: %w", err)
		}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Extension '%s' can be upgraded: v%s -> v%s", extname, installed, available),
			Detail: fmt.Sprintf(
				"Extension '%s' is installed at v%s but v%s is available. "+
					"In a multi-master cluster all nodes must run identical extension versions. "+
					"Consider upgrading before Spock setup to ensure version consistency.",
				extname, installed, available,
			),
			ObjectName: extname,
			Remediation: fmt.Sprintf(
				"Upgrade extension: ALTER EXTENSION %s UPDATE TO '%s';",
				extname, available,
			),
			Metadata: map[string]any{
				"installed_version": installed,
				"available_version": available,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("extension_versions rows error: %w", err)
	}

	return findings, nil
}
