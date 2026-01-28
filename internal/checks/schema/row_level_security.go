// Check for row-level security policies — apply worker context implications.
package schema

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// RowLevelSecurityCheck finds tables with RLS enabled.
type RowLevelSecurityCheck struct{}

func init() {
	check.Register(RowLevelSecurityCheck{})
}

func (RowLevelSecurityCheck) Name() string     { return "row_level_security" }
func (RowLevelSecurityCheck) Category() string  { return "schema" }
func (RowLevelSecurityCheck) Mode() string      { return "scan" }
func (RowLevelSecurityCheck) Description() string {
	return "Row-level security policies — apply worker runs as superuser, bypasses RLS"
}

func (c RowLevelSecurityCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			c.relrowsecurity AS rls_enabled,
			c.relforcerowsecurity AS rls_forced,
			(SELECT count(*)
			 FROM pg_catalog.pg_policy p
			 WHERE p.polrelid = c.oid) AS policy_count
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND c.relrowsecurity = true
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY n.nspname, c.relname;
	`

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("row_level_security query failed: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var schemaName, tableName string
		var rlsEnabled, rlsForced bool
		var policyCount int
		if err := rows.Scan(&schemaName, &tableName, &rlsEnabled, &rlsForced, &policyCount); err != nil {
			return nil, fmt.Errorf("row_level_security scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		forceStr := ""
		if rlsForced {
			forceStr = " (FORCE)"
		}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Row-level security on '%s' (%d policies)", fqn, policyCount),
			Detail: fmt.Sprintf(
				"Table '%s' has RLS enabled%s with %d "+
					"policy(ies). The Spock apply worker runs as superuser, which "+
					"bypasses RLS policies by default. This means all replicated "+
					"rows will be applied regardless of RLS policies on the "+
					"subscriber. If RLS is used to partition data visibility per "+
					"node, this will not work as expected.",
				fqn, forceStr, policyCount,
			),
			ObjectName: fqn,
			Remediation: "If RLS is used for tenant isolation or data filtering, ensure " +
				"that the replication design accounts for the apply worker " +
				"bypassing RLS. Consider using replication sets to control which " +
				"data is replicated to which nodes instead.",
			Metadata: map[string]any{
				"rls_forced":   rlsForced,
				"policy_count": policyCount,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row_level_security rows iteration failed: %w", err)
	}
	return findings, nil
}
