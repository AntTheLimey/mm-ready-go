// Package checks imports all check category packages to trigger their init() registrations.
package checks

// Each category package registers its checks via init() functions.
// Blank imports ensure the init() functions run.

import (
	_ "github.com/AntTheLimey/mm-ready/internal/checks/config"
	_ "github.com/AntTheLimey/mm-ready/internal/checks/extensions"
	_ "github.com/AntTheLimey/mm-ready/internal/checks/functions"
	_ "github.com/AntTheLimey/mm-ready/internal/checks/replication"
	_ "github.com/AntTheLimey/mm-ready/internal/checks/schema"
	_ "github.com/AntTheLimey/mm-ready/internal/checks/sequences"
	_ "github.com/AntTheLimey/mm-ready/internal/checks/sql_patterns"
)
