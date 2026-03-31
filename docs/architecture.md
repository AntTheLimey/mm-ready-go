# Architecture

This document describes the internal architecture of mm-ready-go, a
PostgreSQL readiness scanner for pgEdge Spock 5 multi-master
replication.

## Module Overview

The project follows this directory structure:

```
mm-ready-go/
  main.go                          # Entry point, imports checks/register.go
  internal/
    models/models.go               # Severity (iota), Finding, CheckResult,
                                   #   ScanReport
    check/
      check.go                     # Check interface, Register(),
                                   #   AllRegistered()
      registry.go                  # GetChecks(mode, categories) with
                                   #   filtering/sorting
    checks/
      register.go                  # Blank imports of all 7 category packages
      schema/                      # 22 schema check files
      replication/                 # 12 replication check files
      config/                      # 8 configuration check files
      extensions/                  # 5 extension check files
      sql_patterns/                # 5 SQL pattern check files
      functions/                   # 3 function/trigger check files
      sequences/                   # 2 sequence check files
    config/
      config.go                    # YAML configuration file loading
      config_test.go               # Configuration tests
    connection/connection.go       # pgx connection factory, GetPGVersion()
    scanner/scanner.go             # RunScan() orchestrator
    parser/
      types.go                     # ParsedSchema, TableDef, ColumnDef, etc.
      parser.go                    # ParseDump() - pg_dump SQL parser
    analyzer/
      analyzer.go                  # RunAnalyze() orchestrator
      checks.go                    # 19 static check functions for offline
                                   #   analysis
    reporter/
      json.go                      # Machine-readable JSON output
      markdown.go                  # Human-readable Markdown output
      html.go                      # Styled standalone HTML report
    monitor/
      observer.go                  # Monitor mode orchestrator (3 phases)
      pgstat_collector.go          # pg_stat_statements snapshot & delta
      log_parser.go                # PostgreSQL log file parser
    cmd/
      root.go                      # Cobra root command with
                                   #   default-to-scan
      scan.go                      # scan subcommand
      audit.go                     # audit subcommand
      analyze.go                   # analyze subcommand (offline schema
                                   #   analysis)
      monitor.go                   # monitor subcommand
      listchecks.go                # list-checks subcommand
      output.go                    # Timestamped output path generation
```

## Data Flow

The following diagram shows how data flows through the tool from
CLI invocation to report output:

```
User invokes CLI
       |
       v
   cmd/root.go --- Cobra subcommands
       |
       +-- list-checks --> check.GetChecks() --> print to stdout
       |
       +-- scan / audit
       |     |
       |     +-- config.Load() --> Config (check filtering,
       |     |     report options)
       |     +-- connection.Connect() --> *pgx.Conn (read-only)
       |     +-- scanner.RunScan()
       |     |     +-- check.GetChecks(mode, categories)
       |     |     +-- for each check:
       |     |     |     check.Run(ctx, conn) --> []Finding
       |     |     +-- aggregate into ScanReport
       |     +-- reporter.Render(report, format) --> string
       |     +-- writeOutput() --> file (timestamped) or stdout
       |
       +-- analyze
       |     |
       |     +-- parser.ParseDump(file) --> ParsedSchema
       |     +-- analyzer.RunAnalyze(schema) --> ScanReport
       |     +-- reporter.Render(report, format) --> string
       |     +-- writeOutput()
       |
       +-- monitor
             +-- connection.Connect()
             +-- monitor.RunMonitor()
             |     +-- Run standard scan-mode checks
             |     +-- monitor.CollectOverDuration()
             |     |     +-- TakeSnapshot() [before]
             |     |     +-- time.Sleep(duration)
             |     |     +-- TakeSnapshot() [after]
             |     |           --> StatsDelta
             |     +-- monitor.ParseLogFile()
             |           --> LogAnalysis
             |     +-- Convert deltas/analysis into Findings
             +-- reporter.Render()
             +-- writeOutput()
```

## Core Packages

This section describes each internal package and its
responsibilities.

### internal/cmd

This package serves as the entry point registered via Cobra. The
root command uses custom logic to default to the `scan` subcommand
when no subcommand is specified.

The package handles the following responsibilities:

- Connection flags: `--dsn` or individual parameters such as
  `--host/--port/--dbname/--user/--password`
- SSL flags: `--sslmode/--sslcert/--sslkey/--sslrootcert`
- Output flags: `--format` (json/markdown/html) and `--output`
  (file path)
- Configuration: `--config` (path to YAML config file)
- Routing subcommands to handler functions
- Generating timestamped output filenames (for example,
  `report.html` becomes `report_20260127_131504.html`)

### internal/config

This package loads YAML configuration files for check filtering
and report customization.

The `Load(path)` function performs these steps:

- Reads `mm-ready.yaml` from the specified path, current
  directory, or home directory
- Returns a `Config` struct with check include/exclude lists and
  report options
- Supports global check filtering and mode-specific overrides

### internal/scanner

This package orchestrates check execution for scan and audit modes.

The `RunScan(ctx, conn, host, port, dbname, categories, mode,
verbose)` function follows these steps:

1. Creates a `ScanReport` with metadata (database, host,
   timestamp, PG version)
2. Calls `check.GetChecks()` with mode and category filters
3. Iterates through checks, calling `check.Run(ctx, conn)` on
   each
4. Wraps results in `CheckResult` objects, capturing errors if a
   check fails
5. Returns the completed `ScanReport`

Individual check failures are caught and recorded as errors. They
do not stop the scan.

### internal/check

This package defines the check interface and registry.

The `Check` interface requires the following methods:

```go
type Check interface {
    Name() string
    Category() string
    Description() string
    Mode() string  // "scan", "audit", "both"
    Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error)
}
```

Registration uses `init()` functions. Each check file calls
`check.Register(&myCheck{})` in its `init()`. A central
`checks/register.go` file blank-imports all 7 category packages
to trigger registration. `GetChecks()` returns checks sorted by
`(category, name)`.

### internal/connection

This package provides a database connection factory using
`pgx/v5`.

The `Connect(cfg Config)` function handles these tasks:

- Accepts either a DSN string or individual connection parameters
- Falls back to standard PG environment variables (PGHOST,
  PGUSER, etc.)
- Supports SSL/TLS configuration (sslmode, client certs, CA)
- Configures the connection with
  `default_transaction_read_only = on`
- Returns a `*pgx.Conn`

The `GetPGVersion(ctx, conn)` function returns the PostgreSQL
version string.

### internal/parser

This package provides a schema parser for pg_dump SQL files.

The `ParseDump(file)` function performs these operations:

- Reads a `pg_dump --schema-only` SQL file
- Extracts tables, columns, constraints, indexes, sequences,
  extensions, ENUMs, and rules
- Returns a `ParsedSchema` struct used by the analyzer

### internal/analyzer

This package provides the offline analysis engine for parsed
schema files.

The `RunAnalyze(schema)` function works as follows:

- Runs 19 static checks against the parsed schema structure
- Marks checks requiring live database access as skipped
- Returns a `ScanReport` compatible with the standard reporters

### internal/models

This package defines the data structures used throughout the
application.

`Severity` uses iota ordering: `SeverityCritical=0`,
`SeverityWarning=1`, `SeverityConsider=2`, `SeverityInfo=3`. The
iota ordering gives free `<` comparison so CRITICAL sorts first.

`Finding` is a struct with these fields:

- `Severity`, `CheckName`, `Category`, `Title`, `Detail`
- Optional: `ObjectName`, `Remediation`, `Metadata`
  (map[string]any)

`CheckResult` is a struct with these fields:

- `CheckName`, `Category`, `Description`
- `Findings` slice, `Error` string, `Skipped` bool, `SkipReason`

`ScanReport` is a struct with these fields:

- `Database`, `Host`, `Port`, `Timestamp`, `PGVersion`
- `Results` slice, `ScanMode`, `SpockTarget`
- Methods: `Findings()`, `CriticalCount()`, `WarningCount()`,
  `ConsiderCount()`, `InfoCount()`, `ChecksPassed()`,
  `ChecksTotal()`

### internal/checks

Each check is a single `.go` file containing these components:

1. A struct implementing `check.Check`
2. An `init()` function calling `check.Register()`
3. A `Run()` method executing SQL and building findings

All SQL is embedded as string constants - no external SQL files.

## Reporters

All reporters implement the same function signature:
`func Render(report *models.ScanReport) string`

### json.go

The JSON reporter produces structured JSON with three top-level
keys:

- `meta`: tool version, timestamp, database info, PG version
- `summary`: total checks, passed, critical/warning/info counts
- `results`: array of check results with nested findings

### markdown.go

The Markdown reporter produces a document with these sections:

- Header with database and version info
- Summary table with check counts
- Readiness verdict: READY, CONDITIONALLY READY, or NOT READY
- Findings grouped by severity (CRITICAL first), then by category
- Error section if any checks failed

### html.go

The HTML reporter produces a standalone HTML document with
embedded CSS and JavaScript. It includes the following features:

- Fixed left sidebar with collapsible tree navigation (severity
  to category)
- Scroll tracking via IntersectionObserver that highlights the
  active section in the sidebar
- Summary cards with severity-colored badges
- Semantic color scheme: red (critical), amber (warning), teal
  (consider), blue (info)
- Findings grouped by severity then category with anchor-based
  navigation
- To Do checklist collecting CRITICAL, WARNING, and CONSIDER
  remediations
- Interactive checkboxes with live completion counter
- Print-friendly layout (sidebar hidden)
- Output safety via `html.EscapeString()`

## Monitor Subsystem

The monitor subsystem observes database activity over a time
window and combines the results with a standard scan.

### observer.go

This package provides a three-phase monitoring orchestrator that
runs these phases:

1. Phase 1: Run all standard scan-mode checks (same as
   `mm-ready-go scan`)
2. Phase 2: If `pg_stat_statements` is available, collect
   snapshots before and after the observation window, then compute
   deltas to identify new or changed queries
3. Phase 3: If a log file path is provided, parse it for
   replication-relevant SQL patterns

The orchestrator converts `StatsDelta` and `LogAnalysis` objects
into standard `Finding` objects for inclusion in the report.

### pgstat_collector.go

This package provides snapshot-based observation of
`pg_stat_statements` with these functions:

- `IsAvailable(ctx, conn)`: Checks if the extension is installed
  and queryable
- `TakeSnapshot(ctx, conn)`: Reads all rows, keyed by `queryid`
- `CollectOverDuration(ctx, conn, duration, verbose)`: Takes
  before/after snapshots, returns a `StatsDelta` with new queries
  and changed query metrics

### log_parser.go

This package provides a PostgreSQL log file parser with pattern
classification. It includes the following capabilities:

- Handles multi-line log entries (tab-indented continuation lines)
- Classifies statements into: DDL, TRUNCATE CASCADE, CREATE INDEX
  CONCURRENTLY, temp tables, advisory locks
- Returns a `LogAnalysis` object with categorized statement lists
- Handles encoding issues gracefully

## Design Principles

The project follows these design principles:

1. Read-only safety - Database connections are configured
   read-only. The tool never modifies the target database.

2. init() registration - New checks register themselves via
   `init()`. Adding a check requires creating one file and (for
   new categories) adding a blank import to `register.go`.

3. Error resilience - Individual check failures are captured and
   reported but do not stop the scan. The report includes an error
   section.

4. Scan vs Audit separation - Each check declares its mode
   (`scan`, `audit`, or `both`). Scan mode assumes no Spock
   installed. Audit mode assumes Spock is running and checks its
   health.

5. Multiple output formats - The strategy pattern drives the
   reporters. JSON serves automation, Markdown serves
   terminals/tickets, and HTML serves stakeholders.

6. Timestamped output - Output filenames include a timestamp so
   repeated scans never overwrite previous results.

7. Single binary - All dependencies compile in. No runtime
   requirements exist beyond the binary itself. The build system
   cross-compiles for Linux, macOS, and Windows.

8. context.Context - All database operations accept `ctx` for
   cancellation support (standard Go practice).

9. Configuration file - YAML-based configuration handles check
   filtering and report customization, keeping CLI invocations
   clean.
