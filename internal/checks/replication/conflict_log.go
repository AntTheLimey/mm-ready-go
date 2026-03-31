package replication

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// ConflictLogCheck reviews the Spock conflict log for recent conflicts.
type ConflictLogCheck struct{}

func init() {
	check.Register(&ConflictLogCheck{})
}

// Name returns the unique identifier for this check.
func (c *ConflictLogCheck) Name() string     { return "conflict_log" }
// Category returns the check category.
func (c *ConflictLogCheck) Category() string { return "replication" }
// Description returns a human-readable summary of this check.
func (c *ConflictLogCheck) Description() string {
	return "Review Spock conflict log for recent replication conflicts"
}
// Mode returns when this check runs (scan, audit, or both).
func (c *ConflictLogCheck) Mode() string { return "audit" }

// Run executes the check against the database connection.
func (c *ConflictLogCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	// Check if spock.conflict_history table exists
	var hasTable bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_tables
			WHERE schemaname = 'spock'
			  AND tablename = 'conflict_history'
		);
	`).Scan(&hasTable)
	if err != nil {
		hasTable = false
	}

	if !hasTable {
		return []models.Finding{{
			Severity:  models.SeverityInfo,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "No spock.conflict_history table found",
			Detail: "The spock.conflict_history table does not exist. This is " +
				"normal if Spock is not installed or conflict logging is not configured.",
			ObjectName: "spock.conflict_history",
		}}, nil
	}

	// Get conflict summary
	query := `
		SELECT
			ch_reloid::regclass::text AS table_name,
			ch_conflict_type AS conflict_type,
			ch_conflict_resolution AS resolution,
			count(*) AS conflict_count,
			max(ch_timestamp)::text AS last_conflict
		FROM spock.conflict_history
		GROUP BY ch_reloid, ch_conflict_type, ch_conflict_resolution
		ORDER BY count(*) DESC
		LIMIT 50;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return []models.Finding{{
			Severity:   models.SeverityWarning,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Could not query spock.conflict_history",
			Detail:     fmt.Sprintf("Error querying conflict log: %v", err),
			ObjectName: "spock.conflict_history",
		}}, nil
	}
	defer rows.Close()

	type conflictRow struct {
		tableName    string
		conflictType string
		resolution   string
		count        int
		lastConflict string
	}

	var conflicts []conflictRow
	for rows.Next() {
		var cr conflictRow
		if err := rows.Scan(&cr.tableName, &cr.conflictType, &cr.resolution, &cr.count, &cr.lastConflict); err != nil {
			return nil, fmt.Errorf("scanning conflict_history row: %w", err)
		}
		conflicts = append(conflicts, cr)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating conflict_history rows: %w", err)
	}

	if len(conflicts) == 0 {
		return []models.Finding{{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "No replication conflicts found",
			Detail:     "The spock.conflict_history table contains no records.",
			ObjectName: "spock.conflict_history",
		}}, nil
	}

	totalConflicts := 0
	for _, cr := range conflicts {
		totalConflicts += cr.count
	}

	var findings []models.Finding

	sev := models.SeverityInfo
	if totalConflicts > 0 {
		sev = models.SeverityWarning
	}
	findings = append(findings, models.Finding{
		Severity:  sev,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("%d total replication conflict(s) recorded", totalConflicts),
		Detail: fmt.Sprintf(
			"The conflict history shows %d total conflicts across all tables. "+
				"Review the per-table breakdown below.", totalConflicts),
		ObjectName: "spock.conflict_history",
		Metadata:   map[string]any{"total_conflicts": totalConflicts},
	})

	for _, cr := range conflicts {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("%d '%s' conflicts on '%s'", cr.count, cr.conflictType, cr.tableName),
			Detail: fmt.Sprintf(
				"Table '%s' has %d '%s' conflicts resolved by '%s'. Last conflict: %s.",
				cr.tableName, cr.count, cr.conflictType, cr.resolution, cr.lastConflict),
			ObjectName: cr.tableName,
			Metadata: map[string]any{
				"conflict_type": cr.conflictType,
				"resolution":    cr.resolution,
				"count":         cr.count,
				"last_conflict": cr.lastConflict,
			},
		})
	}

	return findings, nil
}
