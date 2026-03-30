# mm-ready-go

A database scanner that assesses PostgreSQL readiness for
[pgEdge Spock 5](https://www.pgedge.com/) multi-master replication.

Point it at any PostgreSQL database and get a detailed report of schema,
configuration, extension, and SQL pattern issues that need to be addressed
before (or after) deploying Spock.

## Features

- **57 automated checks** across 7 categories — schema, replication, config,
  extensions, SQL patterns, functions, and sequences
- **Three operational modes:**
    - `scan` — pre-Spock readiness assessment (vanilla PostgreSQL, no Spock needed)
    - `audit` — post-Spock health check (database with Spock already running)
    - `analyze` — offline analysis of pg_dump schema files (no database connection needed)
- **Three output formats:** HTML, Markdown, JSON
- **Timestamped reports** — output filenames include a timestamp so previous
  scans are never overwritten
- **Monitor mode** — observe SQL activity over a time window via
  `pg_stat_statements` snapshots and PostgreSQL log parsing
- **Configuration file** — YAML-based configuration for check filtering and
  report customization
- **Single static binary** — no runtime dependencies, cross-compiled for
  Linux, macOS, and Windows

## Quick Install

```bash
go install github.com/pgEdge/mm-ready-go@latest
```

Or build from source:

```bash
git clone https://github.com/pgEdge/mm-ready-go.git && cd mm-ready-go
make build
```

## Usage

mm-ready-go supports three operational modes.

### Scan (pre-Spock readiness)

Run a scan against a vanilla PostgreSQL database to assess readiness for
Spock installation:

```bash
mm-ready-go scan \
  --host db.example.com --port 5432 \
  --dbname myapp --user postgres --password secret \
  --format html --output report.html
```

### Audit (post-Spock health check)

Run an audit against a database that already has Spock installed and running:

```bash
mm-ready-go audit \
  --host db.example.com --dbname myapp --user postgres --password secret \
  --format html --output audit.html
```

### Analyze (offline schema analysis)

Analyze a pg_dump schema file without a database connection:

```bash
mm-ready-go analyze --file customer_schema.sql --format html -v
```

### Monitor (observe activity over time)

Observe SQL activity for a specified duration, then report the patterns found:

```bash
mm-ready-go monitor \
  --host db.example.com --dbname myapp --user postgres --password secret \
  --duration 300
```

## Configuration File

mm-ready-go supports an optional YAML configuration file for check filtering
and report customization. Place a `.mm-ready.yml` file in the current
directory or specify a path with `--config`:

```yaml
checks:
  exclude:
    - temp_tables
    - temp_table_queries

report:
  todo_list: true
  todo_include_consider: false
```

See the [Quickstart Guide](quickstart.md) for more configuration examples.

## Filtering Checks

Filter checks at runtime by category:

```bash
mm-ready-go scan ... --categories schema,replication
```

Or exclude specific checks via configuration file:

```yaml
checks:
  exclude:
    - advisory_locks
```

Available categories: `schema`, `replication`, `config`, `extensions`,
`sql_patterns`, `functions`, `sequences`.

## Severity Levels

| Level | Meaning |
|-------|---------|
| **CRITICAL** | Must be resolved before Spock installation can proceed |
| **WARNING** | Should be reviewed; may cause issues in multi-master operation |
| **CONSIDER** | Should be investigated; may need action depending on context |
| **INFO** | Informational — pure awareness items, no action required |

## Readiness Verdict

The report includes an overall verdict:

- **READY** — no critical or warning issues found
- **CONDITIONALLY READY** — no critical issues, but warnings should be reviewed
- **NOT READY** — critical issues must be resolved first

## Next Steps

- [Quickstart Guide](quickstart.md) — Get running in under 5 minutes
- [Tutorial](tutorial.md) — Hands-on walkthrough of scan, audit, and analyze modes
- [Checks Reference](checks-reference.md) — Detailed documentation of all 57 checks
- [Architecture](architecture.md) — Internal design, module overview, data flow
