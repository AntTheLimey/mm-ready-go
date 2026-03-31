# mm-ready-go

A database scanner that assesses PostgreSQL readiness for
[pgEdge Spock 5](https://www.pgedge.com/) multi-master
replication.

Point it at any PostgreSQL database and get a detailed report
of schema, configuration, extension, and SQL pattern issues
that need to be addressed before (or after) deploying Spock.

mm-ready-go includes the following features:

- 57 automated checks across 7 categories - schema,
  replication, config, extensions, SQL patterns, functions,
  and sequences
- Four operational modes:
  - `scan` - pre-Spock readiness assessment (vanilla
    PostgreSQL, no Spock needed)
  - `audit` - post-Spock health check (database with Spock
    already running)
  - `analyze` - offline schema dump analysis (no database
    connection required)
  - `monitor` - observe SQL activity over a time window via
    `pg_stat_statements` snapshots and PostgreSQL log parsing
- Three output formats: HTML, Markdown, JSON
- Timestamped reports - output filenames include a timestamp
  so previous scans are never overwritten
- Configuration file support - YAML-based check filtering
  and report options
- Single static binary - no runtime dependencies, cross-compiled
  for Linux, macOS, and Windows
- Source-code-verified - check logic is verified against the
  Spock C source code, not just documentation

## Installation

mm-ready-go can be installed directly from GitHub or built
from source.

### From GitHub

Install the latest version:

```bash
go install github.com/pgEdge/mm-ready-go@latest
```

### Build from source

Clone the repository and build:

```bash
git clone https://github.com/pgEdge/mm-ready-go.git && cd mm-ready-go
make build
# Binary at ./bin/mm-ready-go
```

### Cross-compile

```bash
make build-all
# Binaries at ./bin/mm-ready-go-{linux,darwin,windows}-{amd64,arm64}
```

Requires **Go 1.21+**.

## Usage

The following sections describe each operational mode and the
available options.

### Scan (pre-Spock readiness)

Run a scan to assess a database before installing Spock:

```bash
# Using individual connection parameters
mm-ready-go scan \
  --host db.example.com --port 5432 \
  --dbname myapp --user postgres --password secret \
  --format html --output report.html

# Using a DSN
mm-ready-go scan --dsn "postgresql://postgres:secret@db.example.com/myapp" \
  --format json --output report.json

# With SSL
mm-ready-go scan \
  --host db.example.com --dbname myapp --user postgres \
  --sslmode verify-full --sslrootcert /path/to/ca.crt

# Minimal - defaults to scan, HTML format, writes to ./reports/
mm-ready-go --host localhost --dbname myapp --user postgres
```

### Environment variables

All connection parameters fall back to standard PostgreSQL
environment variables when not provided via CLI flags.

The following table maps each CLI flag to its corresponding
environment variable:

| CLI flag | Environment variable |
|----------|---------------------|
| `--host` | `PGHOST` |
| `--port` | `PGPORT` |
| `--dbname` | `PGDATABASE` |
| `--user` | `PGUSER` |
| `--password` | `PGPASSWORD` |
| `--sslmode` | `PGSSLMODE` |
| `--sslcert` | `PGSSLCERT` |
| `--sslkey` | `PGSSLKEY` |
| `--sslrootcert` | `PGSSLROOTCERT` |

CLI flags always take precedence over environment variables.
When using `--dsn`, any additional CLI flags or environment
variables override the corresponding DSN component.

### Audit (post-Spock health check)

Run an audit to check the health of a database that already
has Spock installed:

```bash
mm-ready-go audit \
  --host db.example.com --dbname myapp --user postgres --password secret \
  --format html --output audit.html
```

### Analyze (offline schema dump analysis)

Analyze a `pg_dump --schema-only` SQL file without a live
database connection:

```bash
# Analyze a pg_dump --schema-only SQL file
mm-ready-go analyze --file customer_schema.sql --format html --output report.html

# With verbose output
mm-ready-go analyze --file schema.sql -v
```

The `analyze` mode runs 19 of the 57 checks - those that can
work from schema structure alone. Checks requiring live
database access (GUCs, pg_stat_statements, Spock catalogs,
etc.) are marked as skipped.

### Monitor (observe activity over time)

Run the monitor to observe SQL activity over a specified time
window:

```bash
mm-ready-go monitor \
  --host db.example.com --dbname myapp --user postgres --password secret \
  --duration 3600 --format html --output monitor.html
```

### List available checks

List the checks that mm-ready-go can run:

```bash
mm-ready-go list-checks              # All checks
mm-ready-go list-checks --mode scan  # Scan-mode only
mm-ready-go list-checks --mode audit # Audit-mode only
```

### Check filtering

Control which checks run using CLI flags or a configuration
file:

```bash
# Exclude specific checks
mm-ready-go scan --host localhost --dbname myapp --exclude sequence_audit,sequence_data_types

# Run only specific checks (whitelist mode)
mm-ready-go scan --host localhost --dbname myapp --include-only primary_keys,foreign_keys,wal_level

# Use a specific config file
mm-ready-go scan --host localhost --dbname myapp --config ./customer-config.yaml

# Skip config file entirely
mm-ready-go scan --host localhost --dbname myapp --no-config
```

### Report options

Control the content included in reports:

```bash
# Omit the To Do list from the report
mm-ready-go scan --host localhost --dbname myapp --no-todo

# Include CONSIDER severity items in the To Do list (excluded by default)
mm-ready-go scan --host localhost --dbname myapp --todo-include-consider
```

## Configuration File

Create a `mm-ready.yaml` file to persistently configure check
filtering and report options. The tool searches for
configuration in the following order:

- `--config /path/to/file.yaml` (explicit path)
- `./mm-ready.yaml` (current directory)
- `~/mm-ready.yaml` (home directory)

CLI flags override config file settings.

### Example configuration

The following YAML file demonstrates the available
configuration options:

```yaml
# mm-ready.yaml

# Global settings (apply to all modes)
checks:
  exclude:
    - sequence_audit      # Already addressed
    - sequence_data_types # Not relevant for this project

# Mode-specific overrides
scan:
  checks:
    exclude:
      - pg_version  # We know we're on PG 16

audit:
  checks:
    exclude:
      - conflict_log  # Too noisy in dev environment

# Alternative: whitelist mode (mutually exclusive with exclude)
# checks:
#   include_only:
#     - primary_keys
#     - foreign_keys
#     - wal_level

# Report settings
report:
  todo_list: true              # Show To Do list (default: true)
  todo_include_consider: false # Include CONSIDER in To Do (default: false)
```

## Output

When `--output` is specified, the filename automatically
includes a timestamp:

```text
report.html  -->  report_20260127_131504.html
```

This means you can re-run scans without losing previous
results. You can also pass a directory path and the tool will
generate a filename automatically:

```bash
mm-ready-go scan ... --output ./reports/
# Creates: ./reports/mm-ready-go_20260127_131504.html
```

### Severity Levels

The following table describes the severity levels:

| Level | Meaning |
|-------|---------|
| CRITICAL | Must be resolved before Spock installation can proceed |
| WARNING | Should be reviewed; may cause issues in multi-master operation |
| CONSIDER | Should be investigated; may need action depending on context |
| INFO | Informational items for pure awareness, no action required |

### Readiness Verdict

The report includes an overall verdict based on the findings.

- READY means no critical or warning issues were found.
- CONDITIONALLY READY means no critical issues exist, but
  warnings should be reviewed.
- NOT READY means critical issues must be resolved first.

## Check Categories

The checks are organized into seven categories, each covering
a different aspect of Spock compatibility.

### Schema (22 checks)

These checks analyze table structure for Spock compatibility.

The following table lists all schema checks:

| Check | Severity | What it detects |
|-------|----------|-----------------|
| `primary_keys` | WARNING | Tables missing primary keys (routed to insert-only replication) |
| `tables_update_delete_no_pk` | CRITICAL | Tables with UPDATE/DELETE activity but no PK (changes silently lost) |
| `deferrable_constraints` | CRITICAL/WARNING | Deferrable PK/unique constraints (silently skipped by Spock conflict resolution) |
| `exclusion_constraints` | WARNING | Exclusion constraints (not enforceable cross-node) |
| `foreign_keys` | WARNING/CONSIDER | Foreign key relationships requiring replication set coordination |
| `sequence_pks` | WARNING | PKs using standard sequences (need Snowflake migration) |
| `unlogged_tables` | WARNING | UNLOGGED tables (not written to WAL) |
| `large_objects` | WARNING | Large object usage (not supported by logical decoding) |
| `generated_columns` | CONSIDER | Generated/stored columns |
| `partitioned_tables` | CONSIDER | Partition strategy review |
| `inheritance` | WARNING | Table inheritance |
| `column_defaults` | WARNING | Volatile defaults (now(), random()) |
| `numeric_columns` | WARNING/CONSIDER | Delta-Apply candidates and NOT NULL requirements |
| `multiple_unique_indexes` | CONSIDER | Multiple unique indexes affecting conflict resolution |
| `enum_types` | CONSIDER | ENUM types requiring DDL coordination |
| `rules` | WARNING/CONSIDER | Rules on tables |
| `row_level_security` | WARNING | RLS policies (apply worker bypasses RLS) |
| `event_triggers` | WARNING/INFO | Event triggers interacting with DDL replication |
| `notify_listen` | WARNING | NOTIFY/LISTEN (not replicated) |
| `tablespace_usage` | CONSIDER | Non-default tablespaces (must exist on all nodes) |
| `temp_tables` | INFO | Functions creating temporary tables |
| `missing_fk_indexes` | WARNING | Foreign key columns without indexes (slow cascades, lock contention) |

### Replication (12 checks)

These checks validate PostgreSQL replication configuration.

The following table lists all replication checks:

| Check | Mode | What it detects |
|-------|------|-----------------|
| `wal_level` | scan | Must be 'logical' |
| `max_replication_slots` | scan | Sufficient slots for Spock |
| `max_worker_processes` | scan | Sufficient workers |
| `max_wal_senders` | scan | Sufficient WAL senders |
| `database_encoding` | scan | Encoding consistency requirement |
| `multiple_databases` | scan | Multiple databases in instance |
| `hba_config` | scan | pg_hba.conf replication entries |
| `repset_membership` | audit | Tables not in any replication set |
| `subscription_health` | audit | Disabled subscriptions, inactive slots |
| `conflict_log` | audit | Conflict history analysis |
| `exception_log` | audit | Apply error analysis |
| `stale_replication_slots` | audit | Inactive replication slots retaining WAL |

### Config (8 checks)

These checks cover PostgreSQL server settings.

The following table lists all configuration checks:

| Check | Mode | What it detects |
|-------|------|-----------------|
| `pg_version` | scan | PostgreSQL version compatibility (15, 16, 17, 18) |
| `track_commit_timestamp` | scan | Must be 'on' for conflict resolution |
| `parallel_apply` | scan | Worker configuration summary |
| `shared_preload_libraries` | audit | Spock in shared_preload_libraries |
| `spock_gucs` | audit | Spock-specific GUC settings (conflict resolution, AutoDDL) |
| `timezone_config` | scan | Timezone settings (UTC recommended for commit timestamps) |
| `idle_transaction_timeout` | scan | Idle-in-transaction timeout (blocks VACUUM, causes bloat) |
| `pg_minor_version` | audit | PostgreSQL minor version (all nodes should match) |

### Extensions (5 checks)

These checks review installed extensions and their
compatibility with Spock.

The following table lists all extension checks:

| Check | What it detects |
|-------|-----------------|
| `installed_extensions` | All extensions with compatibility notes |
| `extension_versions` | Installed extension versions vs available upgrades |
| `snowflake_ext` | pgEdge Snowflake availability |
| `pg_stat_statements_check` | pg_stat_statements availability for SQL analysis |
| `lolor_check` | LOLOR extension for large object replication |

### SQL Patterns (5 checks)

These checks analyze `pg_stat_statements` for problematic
query patterns.

The following table lists all SQL pattern checks:

| Check | What it detects |
|-------|-----------------|
| `truncate_cascade` | TRUNCATE CASCADE and RESTART IDENTITY |
| `ddl_statements` | DDL in tracked queries |
| `advisory_locks` | Advisory lock usage (node-local) |
| `concurrent_indexes` | CREATE INDEX CONCURRENTLY |
| `temp_table_queries` | CREATE TEMP TABLE patterns |

### Functions (3 checks)

These checks review stored procedures, triggers, and views.

The following table lists all function checks:

| Check | What it detects |
|-------|-----------------|
| `stored_procedures` | Write operations in functions |
| `trigger_functions` | ENABLE REPLICA / ENABLE ALWAYS triggers |
| `views_audit` | Materialized views requiring refresh coordination |

### Sequences (2 checks)

These checks review sequence configuration and data types.

The following table lists all sequence checks:

| Check | What it detects |
|-------|-----------------|
| `sequence_audit` | Full sequence inventory |
| `sequence_data_types` | smallint/integer sequences (overflow risk) |

## Architecture

The following directory tree shows the project layout:

```text
MM_Ready_Go/
  main.go                          # Entry point
  internal/
    models/models.go               # Severity, Finding, CheckResult, ScanReport
    check/check.go                 # Check interface + Register() + global registry
    check/registry.go              # GetChecks() with mode/category filtering
    config/config.go               # YAML config loader
    checks/register.go             # Blank imports triggering init() registrations
    checks/schema/                 # 22 schema checks
    checks/replication/            # 12 replication checks (scan + audit)
    checks/config/                 # 8 configuration checks
    checks/extensions/             # 5 extension checks
    checks/sql_patterns/           # 5 SQL pattern checks
    checks/functions/              # 3 function/trigger checks
    checks/sequences/              # 2 sequence checks
    parser/
      types.go                     # ParsedSchema, TableDef, ColumnDef, etc.
      parser.go                    # ParseDump() - pg_dump SQL parser
    analyzer/
      analyzer.go                  # RunAnalyze() orchestrator
      checks.go                    # 19 static check functions for offline analysis
    connection/connection.go       # pgx connection, GetPGVersion()
    scanner/scanner.go             # RunScan() orchestrator
    reporter/
      json.go                      # JSON output
      markdown.go                  # Markdown output
      html.go                      # Standalone HTML report
    monitor/
      observer.go                  # Monitor mode orchestrator
      pgstat_collector.go          # pg_stat_statements snapshots
      log_parser.go                # PostgreSQL log file parser
    cmd/
      root.go                      # Cobra root command
      scan.go                      # scan subcommand (default)
      audit.go                     # audit subcommand
      analyze.go                   # analyze subcommand (offline schema analysis)
      monitor.go                   # monitor subcommand
      listchecks.go                # list-checks subcommand
      output.go                    # Timestamped output path generation
```

## Building from Source

```bash
make build          # Build for current platform
make test           # Run unit tests
make test-integration  # Run integration tests
make lint           # go vet
make build-all      # Cross-compile for all platforms
make clean          # Remove binaries
```

## Contributing

Bug reports and pull requests are welcome on
[GitHub](https://github.com/pgEdge/mm-ready-go/issues).

To set up a development environment:

```bash
git clone https://github.com/pgEdge/mm-ready-go.git && cd mm-ready-go
make build
go test ./internal/...
```

## Requirements

- Go 1.21+ (build only - the compiled binary has no runtime dependencies)
- Target database: PostgreSQL 15, 16, 17, or 18
- Read-only access to `pg_catalog`, `pg_stat_statements` (optional), and
  `pg_hba_file_rules` (optional)

## License

Copyright pgEdge, Inc. Licensed under the
[PostgreSQL License](LICENSE.md).
