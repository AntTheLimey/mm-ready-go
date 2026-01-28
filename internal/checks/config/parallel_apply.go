// Check parallel apply worker configuration.
package config

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// ParallelApplyCheck verifies parallel apply worker configuration for Spock performance.
type ParallelApplyCheck struct{}

func init() {
	check.Register(ParallelApplyCheck{})
}

func (ParallelApplyCheck) Name() string        { return "parallel_apply" }
func (ParallelApplyCheck) Category() string     { return "config" }
func (ParallelApplyCheck) Description() string  { return "Parallel apply workers configuration for Spock performance" }
func (ParallelApplyCheck) Mode() string         { return "scan" }

func (c ParallelApplyCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	paramNames := []string{
		"max_worker_processes",
		"max_parallel_workers",
		"max_logical_replication_workers",
		"max_sync_workers_per_subscription",
	}

	params := make(map[string]any, len(paramNames))
	for _, param := range paramNames {
		var val string
		err := conn.QueryRow(ctx, fmt.Sprintf("SHOW %s;", param)).Scan(&val)
		if err != nil {
			params[param] = nil
			continue
		}
		params[param] = val
	}

	var findings []models.Finding

	// max_logical_replication_workers
	if lrRaw, ok := params["max_logical_replication_workers"]; ok && lrRaw != nil {
		lrStr := lrRaw.(string)
		lrVal, err := strconv.Atoi(lrStr)
		if err == nil && lrVal < 4 {
			findings = append(findings, models.Finding{
				Severity:  models.SeverityWarning,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("max_logical_replication_workers = %d", lrVal),
				Detail: fmt.Sprintf(
					"max_logical_replication_workers is %d. Spock uses logical "+
						"replication workers for apply. Increase for better parallel apply throughput.",
					lrVal,
				),
				ObjectName:  "max_logical_replication_workers",
				Remediation: "Set max_logical_replication_workers to at least 4 in postgresql.conf.",
				Metadata:    map[string]any{"current_value": lrVal},
			})
		}
	}

	// max_sync_workers_per_subscription
	if syncRaw, ok := params["max_sync_workers_per_subscription"]; ok && syncRaw != nil {
		syncStr := syncRaw.(string)
		sv, err := strconv.Atoi(syncStr)
		if err == nil && sv < 2 {
			findings = append(findings, models.Finding{
				Severity:  models.SeverityConsider,
				CheckName: c.Name(),
				Category:  c.Category(),
				Title:     fmt.Sprintf("max_sync_workers_per_subscription = %d", sv),
				Detail: fmt.Sprintf(
					"max_sync_workers_per_subscription is %d. Higher values allow "+
						"faster initial table synchronization when setting up Spock subscriptions.",
					sv,
				),
				ObjectName:  "max_sync_workers_per_subscription",
				Remediation: "Consider increasing to 2-4 for faster initial sync.",
				Metadata:    map[string]any{"current_value": sv},
			})
		}
	}

	// Summary finding
	var lines []string
	for _, p := range paramNames {
		v := params[p]
		if v == nil {
			lines = append(lines, fmt.Sprintf("  %s = <not available>", p))
		} else {
			lines = append(lines, fmt.Sprintf("  %s = %s", p, v))
		}
	}
	findings = append(findings, models.Finding{
		Severity:    models.SeverityConsider,
		CheckName:   c.Name(),
		Category:    c.Category(),
		Title:       "Parallel apply configuration summary",
		Detail:      strings.Join(lines, "\n"),
		ObjectName:  "(config)",
		Remediation: "Review values for your expected cluster size and workload.",
		Metadata:    params,
	})

	return findings, nil
}
