package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// MaxReplicationSlotsCheck verifies sufficient replication slots for Spock.
type MaxReplicationSlotsCheck struct{}

func init() {
	check.Register(&MaxReplicationSlotsCheck{})
}

func (c *MaxReplicationSlotsCheck) Name() string     { return "max_replication_slots" }
func (c *MaxReplicationSlotsCheck) Category() string  { return "replication" }
func (c *MaxReplicationSlotsCheck) Description() string {
	return "Sufficient replication slots for Spock node connections"
}
func (c *MaxReplicationSlotsCheck) Mode() string { return "scan" }

func (c *MaxReplicationSlotsCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var maxSlotsStr string
	err := conn.QueryRow(ctx, "SHOW max_replication_slots;").Scan(&maxSlotsStr)
	if err != nil {
		return nil, fmt.Errorf("querying max_replication_slots: %w", err)
	}

	var maxSlots int
	fmt.Sscanf(maxSlotsStr, "%d", &maxSlots)

	var usedSlots int
	err = conn.QueryRow(ctx, "SELECT count(*) FROM pg_catalog.pg_replication_slots;").Scan(&usedSlots)
	if err != nil {
		return nil, fmt.Errorf("counting replication slots: %w", err)
	}

	if maxSlots < 10 {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("max_replication_slots = %d (currently %d in use)", maxSlots, usedSlots),
			Detail: fmt.Sprintf(
				"max_replication_slots is set to %d with %d currently in use. "+
					"Spock requires at least one replication slot per peer node, plus "+
					"slots for any other logical replication consumers. A multi-master "+
					"cluster with N nodes needs N-1 slots per node at minimum.",
				maxSlots, usedSlots),
			ObjectName: "max_replication_slots",
			Remediation: "Set max_replication_slots to at least 10 (or more for larger clusters) " +
				"in postgresql.conf. Requires a restart.",
			Metadata: map[string]any{"current_value": maxSlots, "used": usedSlots},
		}}, nil
	}

	return nil, nil
}
