package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// StaleReplicationSlotsCheck detects inactive replication slots retaining WAL.
type StaleReplicationSlotsCheck struct{}

func init() {
	check.Register(&StaleReplicationSlotsCheck{})
}

func (c *StaleReplicationSlotsCheck) Name() string     { return "stale_replication_slots" }
func (c *StaleReplicationSlotsCheck) Category() string  { return "replication" }
func (c *StaleReplicationSlotsCheck) Description() string {
	return "Inactive replication slots — retaining WAL and risk filling disk"
}
func (c *StaleReplicationSlotsCheck) Mode() string { return "audit" }

func (c *StaleReplicationSlotsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	query := `
		SELECT
			slot_name,
			slot_type,
			active,
			restart_lsn::text,
			confirmed_flush_lsn::text,
			pg_wal_lsn_diff(pg_current_wal_lsn(), restart_lsn) AS wal_retained_bytes
		FROM pg_catalog.pg_replication_slots
		ORDER BY wal_retained_bytes DESC NULLS LAST;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying pg_replication_slots: %w", err)
	}
	defer rows.Close()

	var findings []models.Finding
	for rows.Next() {
		var slotName, slotType string
		var active bool
		var restartLSN, flushLSN *string
		var walBytes *float64
		if err := rows.Scan(&slotName, &slotType, &active, &restartLSN, &flushLSN, &walBytes); err != nil {
			return nil, fmt.Errorf("scanning replication slot row: %w", err)
		}

		if active {
			continue
		}

		walMB := float64(0)
		if walBytes != nil {
			walMB = *walBytes / (1024 * 1024)
		}

		var sev models.Severity
		if walMB > 1024 {
			sev = models.SeverityCritical
		} else if walMB > 100 {
			sev = models.SeverityWarning
		} else {
			sev = models.SeverityConsider
		}

		restartStr := "<nil>"
		if restartLSN != nil {
			restartStr = *restartLSN
		}
		flushStr := "<nil>"
		if flushLSN != nil {
			flushStr = *flushLSN
		}

		findings = append(findings, models.Finding{
			Severity:  sev,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Inactive replication slot '%s' retaining %.0f MB of WAL", slotName, walMB),
			Detail: fmt.Sprintf(
				"Replication slot '%s' (%s) is inactive and preventing WAL cleanup. "+
					"restart_lsn=%s, confirmed_flush_lsn=%s. Retained WAL: %.1f MB.\n\n"+
					"Inactive slots retain WAL segments indefinitely, which can fill the disk. "+
					"This typically indicates a subscriber that is down, unreachable, or has "+
					"been removed without cleaning up its slot.",
				slotName, slotType, restartStr, flushStr, walMB),
			ObjectName: slotName,
			Remediation: fmt.Sprintf(
				"If the subscriber is permanently gone, drop the slot:\n"+
					"  SELECT pg_drop_replication_slot('%s');\n"+
					"If the subscriber is temporarily down, restart it to resume "+
					"consuming WAL. Monitor disk space in the meantime.", slotName),
			Metadata: map[string]any{
				"slot_type":           slotType,
				"wal_retained_mb":     fmt.Sprintf("%.1f", walMB),
				"restart_lsn":        restartStr,
				"confirmed_flush_lsn": flushStr,
			},
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating replication slots: %w", err)
	}

	return findings, nil
}
