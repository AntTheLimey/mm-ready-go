// Package checks imports all check category packages to trigger their init() registrations.
package checks

// Each category package registers its checks via init() functions.
// Blank imports ensure the init() functions run.

import (
	_ "github.com/pgEdge/mm-ready-go/internal/checks/config"
	_ "github.com/pgEdge/mm-ready-go/internal/checks/extensions"
	_ "github.com/pgEdge/mm-ready-go/internal/checks/functions"
	_ "github.com/pgEdge/mm-ready-go/internal/checks/replication"
	_ "github.com/pgEdge/mm-ready-go/internal/checks/schema"
	_ "github.com/pgEdge/mm-ready-go/internal/checks/sequences"
	_ "github.com/pgEdge/mm-ready-go/internal/checks/sql_patterns"
)
