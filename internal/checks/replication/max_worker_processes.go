package replication

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// MaxWorkerProcessesCheck verifies sufficient worker processes for Spock.
type MaxWorkerProcessesCheck struct{}

func init() {
	check.Register(&MaxWorkerProcessesCheck{})
}

// Name returns the unique identifier for this check.
func (c *MaxWorkerProcessesCheck) Name() string { return "max_worker_processes" }

// Category returns the check category.
func (c *MaxWorkerProcessesCheck) Category() string { return "replication" }

// Description returns a human-readable summary of this check.
func (c *MaxWorkerProcessesCheck) Description() string {
	return "Sufficient worker processes for Spock background workers"
}

// Mode returns when this check runs (scan, audit, or both).
func (c *MaxWorkerProcessesCheck) Mode() string { return "scan" }

// Run executes the check against the database connection.
func (c *MaxWorkerProcessesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var maxWorkersStr string
	err := conn.QueryRow(ctx, "SHOW max_worker_processes;").Scan(&maxWorkersStr)
	if err != nil {
		return nil, fmt.Errorf("querying max_worker_processes: %w", err)
	}

	maxWorkers, err := strconv.Atoi(strings.TrimSpace(maxWorkersStr))
	if err != nil {
		return nil, fmt.Errorf("parsing max_worker_processes value %q: %w", maxWorkersStr, err)
	}

	if maxWorkers < 16 {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("max_worker_processes = %d", maxWorkers),
			Detail: fmt.Sprintf(
				"max_worker_processes is set to %d. Spock uses multiple background "+
					"worker processes (supervisor, apply workers per subscription, etc.). "+
					"For a multi-master cluster, this should be set higher to accommodate "+
					"Spock workers alongside standard PostgreSQL workers.", maxWorkers),
			ObjectName: "max_worker_processes",
			Remediation: "Set max_worker_processes to at least 16 (or higher for larger clusters) " +
				"in postgresql.conf. Requires a restart.",
			Metadata: map[string]any{"current_value": maxWorkers},
		}}, nil
	}

	return nil, nil
}
