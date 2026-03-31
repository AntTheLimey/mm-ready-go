// Package extensions contains checks that audit installed PostgreSQL extensions
// for compatibility with Spock 5 replication.
package extensions

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// InstalledExtensionsCheck audits installed extensions for known Spock compatibility issues.
type InstalledExtensionsCheck struct{}

func init() {
	check.Register(InstalledExtensionsCheck{})
}

// Extensions known to have issues or considerations with logical replication.
var knownIssues = map[string]string{
	"postgis":            "PostGIS is supported but ensure identical versions on all nodes.",
	"pg_partman":         "Partition management must be coordinated across nodes.",
	"pgcrypto":           "Supported. Ensure identical versions across nodes.",
	"pg_trgm":            "Supported. Index-only, no replication concerns.",
	"btree_gist":         "Supported. Index-only, no replication concerns.",
	"btree_gin":          "Supported. Index-only, no replication concerns.",
	"hstore":             "Supported. Ensure identical versions across nodes.",
	"ltree":              "Supported. Ensure identical versions across nodes.",
	"citext":             "Supported. Ensure identical versions across nodes.",
	"lo":                 "Large object helper — consider LOLOR instead for replication.",
	"pg_stat_statements": "Monitoring extension. Node-local data only.",
	"dblink":             "Cross-database queries are node-local. Review usage.",
	"postgres_fdw":       "Foreign data wrappers are node-local. Review usage.",
	"file_fdw":           "Foreign data wrappers are node-local. Review usage.",
	"timescaledb":        "TimescaleDB has its own replication. May conflict with Spock.",
	"citus":              "Citus has its own distributed architecture. Incompatible with Spock.",
}

// Extensions that warrant WARNING severity rather than INFO.
var warnExtensions = map[string]bool{
	"timescaledb": true,
	"citus":       true,
	"lo":          true,
}

// Name returns the unique identifier for this check.
func (InstalledExtensionsCheck) Name() string     { return "installed_extensions" }
// Category returns the check category.
func (InstalledExtensionsCheck) Category() string { return "extensions" }
// Mode returns when this check runs (scan, audit, or both).
func (InstalledExtensionsCheck) Mode() string     { return "scan" }
// Description returns a human-readable summary of this check.
func (InstalledExtensionsCheck) Description() string {
	return "Audit installed extensions for known Spock compatibility issues"
}

// Run executes the check against the database connection.
func (c InstalledExtensionsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT extname, extversion, n.nspname AS schema_name
		FROM pg_catalog.pg_extension e
		JOIN pg_catalog.pg_namespace n ON n.oid = e.extnamespace
		ORDER BY extname;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("installed_extensions query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	var extList []string
	rowCount := 0

	for rows.Next() {
		var extname, extversion, schemaName string
		if err := rows.Scan(&extname, &extversion, &schemaName); err != nil {
			return nil, fmt.Errorf("installed_extensions scan failed: %w", err)
		}
		rowCount++
		extList = append(extList, fmt.Sprintf("%s (%s)", extname, extversion))

		note, known := knownIssues[extname]
		if known {
			sev := models.SeverityInfo
			remediation := ""
			if warnExtensions[extname] {
				sev = models.SeverityWarning
				remediation = note
			}
			findings = append(findings, models.Finding{
				Severity:    sev,
				CheckName:   c.Name(),
				Category:    c.Category(),
				Title:       fmt.Sprintf("Extension '%s' v%s", extname, extversion),
				Detail:      note,
				ObjectName:  extname,
				Remediation: remediation,
				Metadata:    map[string]any{"version": extversion, "schema": schemaName},
			})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("installed_extensions rows error: %w", err)
	}

	// Summary finding.
	findings = append(findings, models.Finding{
		Severity:    models.SeverityConsider,
		CheckName:   c.Name(),
		Category:    c.Category(),
		Title:       fmt.Sprintf("Installed extensions: %d", rowCount),
		Detail:      "Extensions: " + strings.Join(extList, ", "),
		ObjectName:  "(extensions)",
		Remediation: "Ensure all extensions are installed at identical versions on every node.",
		Metadata:    map[string]any{"extensions": extList},
	})
	return findings, nil
}
