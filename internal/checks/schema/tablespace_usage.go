// Check for non-default tablespace usage — tablespaces are local to each node.
package schema

import (
	"context"
	"fmt"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// TablespaceUsageCheck finds objects using non-default tablespaces.
type TablespaceUsageCheck struct{}

func init() {
	check.Register(TablespaceUsageCheck{})
}

func (TablespaceUsageCheck) Name() string     { return "tablespace_usage" }
func (TablespaceUsageCheck) Category() string  { return "schema" }
func (TablespaceUsageCheck) Mode() string      { return "scan" }
func (TablespaceUsageCheck) Description() string {
	return "Non-default tablespace usage — tablespaces must exist on all nodes"
}

func (c TablespaceUsageCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	const sqlQuery = `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			ts.spcname AS tablespace_name,
			c.relkind::text
		FROM pg_catalog.pg_class c
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_catalog.pg_tablespace ts ON ts.oid = c.reltablespace
		WHERE c.relkind IN ('r', 'i', 'm')
		  AND c.reltablespace != 0
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		ORDER BY ts.spcname, n.nspname, c.relname;
	`

	kindLabels := map[string]string{
		"r": "table",
		"i": "index",
		"m": "materialized view",
	}

	rows, err := conn.Query(ctx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("tablespace_usage query failed: %w", err)
	}
	defer rows.Close()

	// Group by tablespace
	type tsEntry struct {
		objects []string
	}
	tablespaces := make(map[string]*tsEntry)
	var tsOrder []string

	for rows.Next() {
		var schemaName, tableName, tsName, relkind string
		if err := rows.Scan(&schemaName, &tableName, &tsName, &relkind); err != nil {
			return nil, fmt.Errorf("tablespace_usage scan failed: %w", err)
		}
		fqn := schemaName + "." + tableName
		kind := kindLabels[relkind]
		if kind == "" {
			kind = relkind
		}
		if _, exists := tablespaces[tsName]; !exists {
			tablespaces[tsName] = &tsEntry{}
			tsOrder = append(tsOrder, tsName)
		}
		tablespaces[tsName].objects = append(tablespaces[tsName].objects, fmt.Sprintf("%s (%s)", fqn, kind))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("tablespace_usage rows iteration failed: %w", err)
	}

	if len(tablespaces) == 0 {
		return nil, nil
	}

	var findings []models.Finding
	for _, tsName := range tsOrder {
		entry := tablespaces[tsName]
		objects := entry.objects

		displayObjs := objects
		suffix := ""
		if len(displayObjs) > 10 {
			displayObjs = displayObjs[:10]
			suffix = "..."
		}

		metaObjs := objects
		if len(metaObjs) > 20 {
			metaObjs = metaObjs[:20]
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Tablespace '%s' used by %d object(s)", tsName, len(objects)),
			Detail: fmt.Sprintf(
				"Tablespace '%s' is used by %d object(s): %s%s.\n\n"+
					"Tablespaces are local to each PostgreSQL instance. When setting "+
					"up Spock replication, the same tablespace names must exist on "+
					"all nodes, though they can point to different physical paths.",
				tsName, len(objects), strings.Join(displayObjs, ", "), suffix,
			),
			ObjectName: tsName,
			Remediation: fmt.Sprintf(
				"Ensure tablespace '%s' is created on all Spock nodes "+
					"before initializing replication.",
				tsName,
			),
			Metadata: map[string]any{"object_count": len(objects), "objects": metaObjs},
		})
	}
	return findings, nil
}
