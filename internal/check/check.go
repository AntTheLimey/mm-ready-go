// Package check defines the Check interface and global registry.
package check

import (
	"context"

	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// Check is the interface that all compatibility checks must implement.
type Check interface {
	// Name returns the unique identifier for this check.
	Name() string
	// Category returns the grouping category (schema, replication, config, etc.).
	Category() string
	// Description returns a human-readable summary of what this check does.
	Description() string
	// Mode returns when this check applies: "scan", "audit", or "both".
	Mode() string
	// Run executes the check against the database connection.
	// An empty slice means the check passed.
	Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error)
}

var registry []Check

// Register adds a check to the global registry. Called from init() in each check file.
func Register(c Check) {
	registry = append(registry, c)
}

// ResetRegistry clears the global registry. Used only in tests.
func ResetRegistry() {
	registry = nil
}

// AllRegistered returns a copy of all registered checks (unfiltered, unsorted).
func AllRegistered() []Check {
	out := make([]Check, len(registry))
	copy(out, registry)
	return out
}
