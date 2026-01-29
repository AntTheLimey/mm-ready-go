# MM-Ready Project Instructions (Go)

## Project Overview

mm-ready is a Go CLI tool that scans a PostgreSQL database and generates a
compatibility report for converting it to multi-master replication using
pgEdge Spock 5. It compiles to a single static binary and runs from any
platform against any connectable PostgreSQL instance.

## Key Principles

- **Trust source code over documentation.** The Spock source code at
  ~/PROJECTS/spock/ is the authoritative reference. Spock documentation is
  frequently wrong or out of date. Always verify claims against the C source.

- **Operational modes.** The tool has three main modes:
  - `scan` (default) — pre-Spock readiness assessment of a vanilla PostgreSQL
    database that does NOT have Spock installed. This is the primary use case.
  - `audit` — post-Spock health check of a database that already has Spock
    installed and running.
  - `analyze` — offline analysis of a `pg_dump --schema-only` SQL file without
    a database connection. Useful for Customer Success when customers send
    schema dumps. Runs 19 of the 56 checks (those that can work from schema
    structure alone); the remaining 37 are marked as skipped.

  Checks are tagged with `mode = "scan"`, `mode = "audit"`, or `mode = "both"`.
  Scan-mode checks must never assume Spock is installed. Audit-mode checks may
  query Spock catalog tables (spock.subscription, spock.repset_table, etc.).

- **Severity levels.** CRITICAL = must fix before Spock install.
  WARNING = should fix or review. CONSIDER = should investigate, may need
  action depending on context. INFO = pure awareness, no action required.

## Architecture

```
MM_Ready_Go/
  main.go                          # Entry point, imports internal/checks for init()
  internal/
    models/models.go               # Severity (iota), Finding, CheckResult, ScanReport
    check/
      check.go                     # Check interface, Register(), AllRegistered()
      registry.go                  # GetChecks(mode, categories) with filtering/sorting
    checks/
      register.go                  # Blank imports of all 7 category packages
      schema/                      # 22 check files (one per check)
      replication/                 # 12 check files
      config/                      # 8 check files
      extensions/                  # 4 check files
      functions/                   # 3 check files
      sequences/                   # 2 check files
      sql_patterns/                # 5 check files
    parser/
      types.go                     # ParsedSchema, TableDef, ColumnDef, etc.
      parser.go                    # ParseDump() - pg_dump SQL parser
    analyzer/
      analyzer.go                  # RunAnalyze() orchestrator
      checks.go                    # 19 static check functions for offline analysis
    connection/connection.go       # Connect(Config) -> *pgx.Conn, GetPGVersion()
    scanner/scanner.go             # RunScan() orchestrator
    reporter/
      json.go                      # JSON reporter
      markdown.go                  # Markdown reporter
      html.go                      # HTML reporter (string building, embedded CSS/JS)
    monitor/
      observer.go                  # RunMonitor() orchestrator (3 phases)
      pgstat_collector.go          # pg_stat_statements snapshots
      log_parser.go                # PostgreSQL log file parser
    cmd/
      root.go                      # Cobra root command with default-to-scan logic
      scan.go                      # scan subcommand
      audit.go                     # audit subcommand
      analyze.go                   # analyze subcommand (offline schema analysis)
      monitor.go                   # monitor subcommand
      listchecks.go                # list-checks subcommand
      output.go                    # Timestamped output path generation
```

## Adding a New Check

1. Create a new `.go` file in the appropriate `internal/checks/` subdirectory.
2. Define a struct implementing the `check.Check` interface (Name, Category,
   Description, Mode, Run).
3. Add an `init()` function that calls `check.Register(&MyCheck{})`.
4. If the check is in a new category package, add a blank import to
   `internal/checks/register.go`.
5. Update the expected count in `internal/checks/registry_test.go`.

Example:
```go
package schema

import (
    "context"
    "github.com/AntTheLimey/mm-ready/internal/check"
    "github.com/AntTheLimey/mm-ready/internal/models"
    "github.com/jackc/pgx/v5"
)

type myCheck struct{}

func init() { check.Register(&myCheck{}) }

func (c *myCheck) Name() string        { return "my_check" }
func (c *myCheck) Category() string    { return "schema" }
func (c *myCheck) Description() string { return "Check for something important" }
func (c *myCheck) Mode() string        { return "scan" }

func (c *myCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
    rows, err := conn.Query(ctx, "SELECT ...")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var findings []models.Finding
    for rows.Next() {
        // ... scan and build findings ...
    }
    return findings, rows.Err()
}
```

## Important Spock Facts (verified from source code)

- Tables without PKs go to `default_insert_only` replication set automatically.
  UPDATE/DELETE on these tables are silently dropped by the output plugin.
- REPLICA IDENTITY FULL is NOT supported as a standalone replication identity
  by Spock. `get_replication_identity()` returns InvalidOid for FULL without PK.
- Delta-Apply columns MUST have NOT NULL constraints (spock_apply_heap.c:613-627).
- Trigger firing: apply workers run with `session_replication_role='replica'`.
  Both ENABLE REPLICA and ENABLE ALWAYS triggers fire during apply.
- Encoding: source code requires same encoding on both sides, NOT UTF-8 specifically.
- TRUNCATE RESTART IDENTITY: source code passes `restart_seqs` through (docs say
  "not supported" but code handles it).
- Deferrable indexes: silently skipped for conflict resolution via
  `IsIndexUsableForInsertConflict()` checking `indimmediate`.
- Spock 5 supports PostgreSQL 15, 16, 17, 18 (PG 18 added in 5.0.3).

## Testing

### Unit tests (no database required)

```bash
go test ./internal/...
```

### Integration tests (Docker-based)

```bash
# Start pgEdge Postgres
docker run -d --name mmready-test \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mmready \
  -p 5499:5432 \
  ghcr.io/pgedge/pgedge-postgres:18.1-spock5.0.4-standard-1

# Configure for external access and pg_stat_statements
docker exec mmready-test psql -U postgres \
  -c "ALTER SYSTEM SET listen_addresses = '*';" \
  -c "ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';"
docker restart mmready-test

# Load test schema, enable pg_stat_statements, then run workload
docker exec mmready-test psql -U postgres -d mmready \
  -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
docker cp tests/test_schema_setup.sql mmready-test:/tmp/schema.sql
docker exec mmready-test psql -U postgres -d mmready -f /tmp/schema.sql
docker cp tests/test_workload.sql mmready-test:/tmp/workload.sql
docker exec mmready-test psql -U postgres -d mmready -f /tmp/workload.sql

# Run integration tests
go test -tags integration ./tests/ -v

# Run scan
./bin/mm-ready scan --host localhost --port 5499 --dbname mmready \
  --user postgres --password postgres --format html --output report.html
```

- `tests/test_schema_setup.sql` — idempotent setup of the `mmr_` prefixed
  schema, including tables that trigger every scan-mode check.
- `tests/test_workload.sql` — idempotent workload that populates
  pg_stat_statements and exercises the mmr_ tables. The database is unchanged
  after each run (inserts are deleted, updates are reverted).

## Offline Analysis (analyze mode)

The `analyze` subcommand parses a pg_dump SQL file and runs schema-structural
checks without a database connection:

```bash
./bin/mm-ready analyze --file customer_schema.sql --format html -v
```

The schema parser (`internal/parser/parser.go`) extracts:
- Tables (columns, constraints, UNLOGGED, INHERITS, PARTITION BY)
- Constraints (PK, UNIQUE, FK with CASCADE options, EXCLUDE, DEFERRABLE)
- Indexes (unique, method, columns)
- Sequences (data type, ownership)
- Extensions, ENUM types, Rules

The analyzer (`internal/analyzer/`) runs 19 static checks that can operate on
parsed schema structure. Checks requiring live database access (GUCs, pg_stat,
Spock catalogs, etc.) are marked as skipped with reason "Requires live database
connection".

## Code Style

- Go 1.21+ with standard library conventions.
- All functions accept `context.Context` as first parameter.
- Errors returned, not panicked. Check failures captured into `CheckResult.Error`.
- SQL queries use `pg_catalog` system tables, not `information_schema`.
- Check findings should include actionable remediation text.
- Reference specific Spock source file + line when citing behaviour.
- Each check is a single file with struct, init(), and Run() method.
- Use `pgx.Conn` directly (not pool) — scanner runs checks sequentially.

## Build

```bash
make build          # Build for current platform
make test           # Run unit tests
make test-integration  # Run integration tests
make lint           # go vet
make build-all      # Cross-compile for all platforms
make clean          # Remove binaries
```
