// Check for LOLOR extension — required for replicating large objects.
package extensions

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// LolorCheck verifies whether the LOLOR extension is needed and configured.
type LolorCheck struct{}

func init() {
	check.Register(LolorCheck{})
}

// Name returns the unique identifier for this check.
func (LolorCheck) Name() string { return "lolor_check" }

// Category returns the check category.
func (LolorCheck) Category() string { return "extensions" }

// Mode returns when this check runs (scan, audit, or both).
func (LolorCheck) Mode() string { return "scan" }

// Description returns a human-readable summary of this check.
func (LolorCheck) Description() string {
	return "LOLOR extension — required for replicating large objects"
}

// Run executes the check against the database connection.
func (c LolorCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var lobCount int
	err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_catalog.pg_largeobject_metadata;").Scan(&lobCount)
	if err != nil {
		return nil, fmt.Errorf("lolor_check: query large objects: %w", err)
	}

	var oidColCount int
	err = conn.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND a.atttypid = 'oid'::regtype;
	`).Scan(&oidColCount)
	if err != nil {
		return nil, fmt.Errorf("lolor_check: query OID columns: %w", err)
	}

	if lobCount == 0 && oidColCount == 0 {
		return nil, nil
	}

	var extVersion *string
	err = conn.QueryRow(ctx,
		`SELECT extversion FROM pg_catalog.pg_extension WHERE extname = 'lolor';`,
	).Scan(&extVersion)
	if err != nil && err != pgx.ErrNoRows {
		return nil, fmt.Errorf("lolor_check: query extension version: %w", err)
	}

	if extVersion == nil {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "Large objects detected but LOLOR extension is not installed",
			Detail: fmt.Sprintf("Found %d large object(s) and %d OID-type column(s). "+
				"PostgreSQL's logical decoding does not support large objects. "+
				"The LOLOR (Large Object Logical Replication) extension is required "+
				"to replicate large objects with Spock.", lobCount, oidColCount),
			ObjectName: "lolor",
			Remediation: "Install and configure the LOLOR extension:\n" +
				"  CREATE EXTENSION lolor;\n" +
				"  ALTER SYSTEM SET lolor.node = <unique_node_id>;\n" +
				"  -- Restart PostgreSQL\n" +
				"Each node must have a unique lolor.node value (1 to 2^28). " +
				"Then add lolor tables to the replication set:\n" +
				"  SELECT spock.repset_add_table('default', 'lolor.pg_largeobject');\n" +
				"  SELECT spock.repset_add_table('default', 'lolor.pg_largeobject_metadata');",
			Metadata: map[string]any{"lob_count": lobCount, "oid_col_count": oidColCount},
		}}, nil
	}

	var nodeVal *string
	err = conn.QueryRow(ctx, "SELECT current_setting('lolor.node', true);").Scan(&nodeVal)
	if err != nil {
		return nil, fmt.Errorf("lolor_check: query lolor.node: %w", err)
	}

	if nodeVal == nil || *nodeVal == "" || *nodeVal == "0" {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "LOLOR installed but lolor.node is not configured",
			Detail: fmt.Sprintf("LOLOR extension v%s is installed, but lolor.node is not set "+
				"(or set to 0). Each node must have a unique lolor.node value for large "+
				"object replication to work correctly.", *extVersion),
			ObjectName: "lolor.node",
			Remediation: "Set a unique node identifier:\n" +
				"  ALTER SYSTEM SET lolor.node = <unique_id>;\n" +
				"  -- Restart PostgreSQL\n" +
				"The value must be unique across all nodes (1 to 2^28).",
		}}, nil
	}

	return []models.Finding{{
		Severity:  models.SeverityInfo,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("LOLOR extension installed (v%s, node=%s)", *extVersion, *nodeVal),
		Detail: fmt.Sprintf("LOLOR is installed and lolor.node is set to %s. "+
			"Ensure this value is unique across all cluster nodes and that "+
			"lolor.pg_largeobject and lolor.pg_largeobject_metadata are members of a replication set.",
			*nodeVal),
		ObjectName: "lolor",
		Metadata:   map[string]any{"version": *extVersion, "node": *nodeVal},
	}}, nil
}
