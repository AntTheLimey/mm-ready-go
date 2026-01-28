# Architecture

This document describes the internal architecture of mm-ready, a PostgreSQL
readiness scanner for pgEdge Spock 5 multi-master replication.

## Module Overview

```
MM_Ready_Go/
  main.go                              # Entry point, imports checks/register.go
  internal/
    models/models.go                    # Severity (iota), Finding, CheckResult, ScanReport
    check/
      check.go                          # Check interface, Register(), AllRegistered()
      registry.go                       # GetChecks(mode, categories) with filtering/sorting
    checks/
      register.go                       # Blank imports of all 7 category packages
      schema/                           # 22 schema check files
      replication/                      # 12 replication check files
      config/                           # 8 configuration check files
      extensions/                       # 4 extension check files
      sql_patterns/                     # 5 SQL pattern check files
      functions/                        # 3 function/trigger check files
      sequences/                        # 2 sequence check files
    connection/connection.go            # pgx connection factory, GetPGVersion()
    scanner/scanner.go                  # RunScan() orchestrator
    reporter/
      json.go                           # Machine-readable JSON output
      markdown.go                       # Human-readable Markdown output
      html.go                           # Styled standalone HTML report
    monitor/
      observer.go                       # Monitor mode orchestrator (3 phases)
      pgstat_collector.go               # pg_stat_statements snapshot & delta
      log_parser.go                     # PostgreSQL log file parser
    cmd/
      root.go                           # Cobra root command with default-to-scan
      scan.go                           # scan subcommand
      audit.go                          # audit subcommand
      monitor.go                        # monitor subcommand
      listchecks.go                     # list-checks subcommand
      output.go                         # Timestamped output path generation
```

## Data Flow

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
       |     +-- connection.Connect() --> *pgx.Conn (read-only)
       |     +-- scanner.RunScan()
       |     |     +-- check.GetChecks(mode, categories)
       |     |     +-- for each check: check.Run(ctx, conn) --> []Finding
       |     |     +-- aggregate into ScanReport
       |     +-- reporter.Render(report, format) --> string
       |     +-- writeOutput() --> file (timestamped) or stdout
       |
       +-- monitor
             +-- connection.Connect()
             +-- monitor.RunMonitor()
             |     +-- Run standard scan-mode checks
             |     +-- monitor.CollectOverDuration()
             |     |     +-- TakeSnapshot() [before]
             |     |     +-- time.Sleep(duration)
             |     |     +-- TakeSnapshot() [after] --> StatsDelta
             |     +-- monitor.ParseLogFile() --> LogAnalysis
             |     +-- Convert deltas/analysis into Findings
             +-- reporter.Render()
             +-- writeOutput()
```

## Core Packages

### internal/cmd

Entry point registered via Cobra. The root command uses custom logic to
default to the `scan` subcommand when no subcommand is specified.

Responsibilities:
- Connection flags: `--dsn` or `--host/--port/--dbname/--user/--password`
- Output flags: `--format` (json/markdown/html), `--output` (file path)
- Route subcommands to handler functions
- Generate timestamped output filenames (`report.html` becomes
  `report_20260127_131504.html`)

### internal/scanner

Orchestrates check execution for scan and audit modes.

`RunScan(ctx, conn, host, port, dbname, categories, mode, verbose)`:
1. Creates a `ScanReport` with metadata (database, host, timestamp, PG version)
2. Calls `check.GetChecks()` with mode and category filters
3. Iterates through checks, calling `check.Run(ctx, conn)` on each
4. Wraps results in `CheckResult` objects, capturing errors if a check fails
5. Returns the completed `ScanReport`

Individual check failures are caught and recorded as errors — they do not stop
the scan.

### internal/check

Check interface and registry.

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
`checks/register.go` file blank-imports all 7 category packages to trigger
registration. `GetChecks()` returns checks sorted by `(category, name)`.

### internal/connection

Database connection factory using `pgx/v5`.

`Connect(cfg Config)`:
- Accepts either a DSN string or individual connection parameters
- Falls back to standard PG environment variables (PGHOST, PGUSER, etc.)
- Configures the connection with `default_transaction_read_only = on`
- Returns a `*pgx.Conn`

`GetPGVersion(ctx, conn)`: Returns the PostgreSQL version string.

### internal/models

Data structures used throughout the application.

**`Severity`** (iota): `SeverityCritical=0`, `SeverityWarning=1`,
`SeverityConsider=2`, `SeverityInfo=3` — iota ordering gives free `<`
comparison so CRITICAL sorts first.

**`Finding`** (struct):
- `Severity`, `CheckName`, `Category`, `Title`, `Detail`
- Optional: `ObjectName`, `Remediation`, `Metadata` (map[string]any)

**`CheckResult`** (struct):
- `CheckName`, `Category`, `Description`
- `Findings` slice, `Error` string, `Skipped` bool, `SkipReason`

**`ScanReport`** (struct):
- `Database`, `Host`, `Port`, `Timestamp`, `PGVersion`
- `Results` slice, `ScanMode`, `SpockTarget`
- Methods: `Findings()`, `CriticalCount()`, `WarningCount()`,
  `ConsiderCount()`, `InfoCount()`, `ChecksPassed()`, `ChecksTotal()`

### internal/checks

Each check is a single `.go` file containing:
1. A struct implementing `check.Check`
2. An `init()` function calling `check.Register()`
3. A `Run()` method executing SQL and building findings

All SQL is embedded as string constants — no external SQL files.

## Reporters

All reporters implement: `func Render(report *models.ScanReport) string`

### json.go

Produces structured JSON with three top-level keys:
- `meta`: tool version, timestamp, database info, PG version
- `summary`: total checks, passed, critical/warning/info counts
- `results`: array of check results with nested findings

### markdown.go

Produces a Markdown document with:
- Header with database and version info
- Summary table with check counts
- Readiness verdict: **READY**, **CONDITIONALLY READY**, or **NOT READY**
- Findings grouped by severity (CRITICAL first), then by category
- Error section if any checks failed

### html.go

Produces a standalone HTML document with embedded CSS and JavaScript:
- Fixed left sidebar with collapsible tree navigation (severity -> category)
- Scroll tracking via IntersectionObserver highlights active section in sidebar
- Summary cards with severity-colored badges
- Semantic color scheme: red (critical), amber (warning), teal (consider), blue (info)
- Findings grouped by severity -> category with anchor-based navigation
- To Do checklist collecting CRITICAL, WARNING, and CONSIDER remediations
- Interactive checkboxes with live completion counter
- Print-friendly layout (sidebar hidden)
- Uses `html.EscapeString()` for output safety

## Monitor Subsystem

### observer.go

Three-phase monitoring orchestrator:

1. **Phase 1**: Run all standard scan-mode checks (same as `mm-ready scan`)
2. **Phase 2**: If `pg_stat_statements` is available, collect snapshots before
   and after the observation window, then compute deltas to identify new or
   changed queries
3. **Phase 3**: If a log file path is provided, parse it for
   replication-relevant SQL patterns

Converts `StatsDelta` and `LogAnalysis` objects into standard `Finding` objects
for inclusion in the report.

### pgstat_collector.go

Snapshot-based observation of `pg_stat_statements`:

- `IsAvailable(ctx, conn)`: Checks if the extension is installed and queryable
- `TakeSnapshot(ctx, conn)`: Reads all rows, keyed by `queryid`
- `CollectOverDuration(ctx, conn, duration, verbose)`: Takes before/after
  snapshots, returns a `StatsDelta` with new queries and changed query metrics

### log_parser.go

PostgreSQL log file parser with pattern classification:

- Handles multi-line log entries (tab-indented continuation lines)
- Classifies statements into: DDL, TRUNCATE CASCADE, CREATE INDEX CONCURRENTLY,
  temp tables, advisory locks
- Returns a `LogAnalysis` object with categorized statement lists
- Handles encoding issues gracefully

## Design Principles

1. **Read-only safety** — Database connections are configured read-only. The
   tool never modifies the target database.

2. **init() registration** — New checks register themselves via `init()`.
   Adding a check requires creating one file and (for new categories) adding
   a blank import to `register.go`.

3. **Error resilience** — Individual check failures are captured and reported
   but do not stop the scan. The report includes an error section.

4. **Scan vs Audit separation** — Each check declares its mode (`scan`,
   `audit`, or `both`). Scan mode assumes no Spock installed. Audit mode
   assumes Spock is running and checks its health.

5. **Multiple output formats** — Strategy pattern for reporters. JSON for
   automation, Markdown for terminals/tickets, HTML for stakeholders.

6. **Timestamped output** — Output filenames include a timestamp so repeated
   scans never overwrite previous results.

7. **Single binary** — All dependencies compiled in. No runtime requirements
   beyond the binary itself. Cross-compiled for Linux, macOS, Windows.

8. **context.Context** — All database operations accept `ctx` for cancellation
   support (standard Go practice).
