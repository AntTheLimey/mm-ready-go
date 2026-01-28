package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// MaxWorkerProcessesCheck verifies sufficient worker processes for Spock.
type MaxWorkerProcessesCheck struct{}

func init() {
	check.Register(&MaxWorkerProcessesCheck{})
}

func (c *MaxWorkerProcessesCheck) Name() string     { return "max_worker_processes" }
func (c *MaxWorkerProcessesCheck) Category() string  { return "replication" }
func (c *MaxWorkerProcessesCheck) Description() string {
	return "Sufficient worker processes for Spock background workers"
}
func (c *MaxWorkerProcessesCheck) Mode() string { return "scan" }

func (c *MaxWorkerProcessesCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var maxWorkersStr string
	err := conn.QueryRow(ctx, "SHOW max_worker_processes;").Scan(&maxWorkersStr)
	if err != nil {
		return nil, fmt.Errorf("querying max_worker_processes: %w", err)
	}

	var maxWorkers int
	fmt.Sscanf(maxWorkersStr, "%d", &maxWorkers)

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
