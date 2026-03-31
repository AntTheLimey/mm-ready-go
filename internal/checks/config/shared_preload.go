// Check shared_preload_libraries includes spock (audit mode only).
package config

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
)

// SharedPreloadCheck verifies that 'spock' is in shared_preload_libraries.
type SharedPreloadCheck struct{}

func init() {
	check.Register(SharedPreloadCheck{})
}

// Name returns the unique identifier for this check.
func (SharedPreloadCheck) Name() string     { return "shared_preload_libraries" }
// Category returns the check category.
func (SharedPreloadCheck) Category() string { return "config" }
// Description returns a human-readable summary of this check.
func (SharedPreloadCheck) Description() string {
	return "shared_preload_libraries must include 'spock' for Spock operation"
}
// Mode returns when this check runs (scan, audit, or both).
func (SharedPreloadCheck) Mode() string { return "audit" }

// Run executes the check against the database connection.
func (c SharedPreloadCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var libs string
	err := conn.QueryRow(ctx, "SHOW shared_preload_libraries;").Scan(&libs)
	if err != nil {
		return nil, fmt.Errorf("shared_preload_libraries query failed: %w", err)
	}

	// Parse comma-separated list
	var libList []string
	if libs != "" {
		for _, l := range strings.Split(libs, ",") {
			trimmed := strings.TrimSpace(l)
			if trimmed != "" {
				libList = append(libList, trimmed)
			}
		}
	}

	// Check if "spock" is in the list
	found := false
	for _, lib := range libList {
		if lib == "spock" {
			found = true
			break
		}
	}

	if !found {
		return []models.Finding{{
			Severity:  models.SeverityCritical,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "'spock' not in shared_preload_libraries",
			Detail: fmt.Sprintf(
				"shared_preload_libraries = '%s'. The 'spock' library must be "+
					"included for Spock to function. This requires a server restart.", libs,
			),
			ObjectName: "shared_preload_libraries",
			Remediation: "Add 'spock' to shared_preload_libraries in postgresql.conf and restart. " +
				"Example: shared_preload_libraries = 'spock'",
			Metadata: map[string]any{"current_libs": libList},
		}}, nil
	}

	return nil, nil
}
