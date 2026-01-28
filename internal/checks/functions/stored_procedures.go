// Audit stored procedures for potential replication issues.
package functions

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// StoredProceduresCheck audits stored procedures/functions for write operations and DDL.
type StoredProceduresCheck struct{}

func init() {
	check.Register(StoredProceduresCheck{})
}

func (StoredProceduresCheck) Name() string        { return "stored_procedures" }
func (StoredProceduresCheck) Category() string     { return "functions" }
func (StoredProceduresCheck) Mode() string         { return "scan" }
func (StoredProceduresCheck) Description() string {
	return "Audit stored procedures/functions for write operations and DDL"
}

var kindLabels = map[string]string{
	"f": "function",
	"p": "procedure",
	"a": "aggregate",
	"w": "window",
}

var volLabels = map[string]string{
	"i": "IMMUTABLE",
	"s": "STABLE",
	"v": "VOLATILE",
}

var writePatterns = []string{
	"INSERT", "UPDATE", "DELETE", "TRUNCATE",
	"CREATE ", "ALTER ", "DROP ",
	"EXECUTE ", "PERFORM ",
}

func (c StoredProceduresCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const query = `
		SELECT
			n.nspname AS schema_name,
			p.proname AS func_name,
			p.prokind::text AS kind,
			l.lanname AS language,
			p.provolatile::text AS volatility,
			pg_get_functiondef(p.oid) AS func_def
		FROM pg_catalog.pg_proc p
		JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
		JOIN pg_catalog.pg_language l ON l.oid = p.prolang
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND l.lanname IN ('plpgsql', 'sql', 'plpython3u', 'plperl', 'plv8')
		ORDER BY n.nspname, p.proname;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("stored_procedures query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	rowCount := 0

	for rows.Next() {
		var schemaName, funcName, kind, language, volatility string
		var funcDef *string
		if err := rows.Scan(&schemaName, &funcName, &kind, &language, &volatility, &funcDef); err != nil {
			return nil, fmt.Errorf("stored_procedures scan failed: %w", err)
		}
		rowCount++

		if funcDef == nil {
			continue
		}

		funcUpper := strings.ToUpper(*funcDef)
		var foundWrites []string
		for _, p := range writePatterns {
			if strings.Contains(funcUpper, p) {
				foundWrites = append(foundWrites, p)
			}
		}

		if len(foundWrites) == 0 {
			continue
		}

		fqn := fmt.Sprintf("%s.%s", schemaName, funcName)
		kindLabel := kindLabels[kind]
		if kindLabel == "" {
			kindLabel = kind
		}
		volLabel := volLabels[volatility]
		if volLabel == "" {
			volLabel = volatility
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title: fmt.Sprintf("%s '%s' (%s, %s) contains write operations",
				strings.Title(kindLabel), fqn, language, volLabel), //nolint:staticcheck
			Detail: fmt.Sprintf(
				"%s '%s' written in %s (%s) contains potential write operations: %s. "+
					"Write operations inside functions/procedures are replicated through "+
					"the WAL (row-level changes), not by replaying the function call. "+
					"However, side effects like DDL, NOTIFY, or external calls are not replicated.",
				strings.Title(kindLabel), fqn, language, volLabel, //nolint:staticcheck
				strings.Join(foundWrites, ", "),
			),
			ObjectName: fqn,
			Remediation: "Review this function for side effects that won't replicate: DDL, " +
				"NOTIFY/LISTEN, advisory locks, temp tables, external system calls.",
			Metadata: map[string]any{
				"kind":           kindLabel,
				"language":       language,
				"volatility":     volLabel,
				"write_patterns": foundWrites,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("stored_procedures rows error: %w", err)
	}

	// Summary finding.
	if rowCount > 0 {
		findings = append(findings, models.Finding{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      fmt.Sprintf("Found %d user-defined function(s)/procedure(s)", rowCount),
			Detail:     fmt.Sprintf("Audited %d functions/procedures across all user schemas.", rowCount),
			ObjectName: "(functions)",
			Metadata:   map[string]any{"total_count": rowCount},
		})
	}

	return findings, nil
}
