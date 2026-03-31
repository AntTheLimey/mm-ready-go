# Quickstart Guide

Get mm-ready-go running against your database in under 5 minutes.

## 1. Install

Choose one of the following installation methods.

### Option A: go install

```bash
go install github.com/pgEdge/mm-ready-go@latest
```

### Option B: Build from source

```bash
git clone https://github.com/pgEdge/mm-ready-go.git
cd mm-ready-go
make build
```

The binary is at `./bin/mm-ready-go`.

### Option C: Download a pre-built binary

Download from the [releases page](https://github.com/pgEdge/mm-ready-go/releases)
for your platform:

- `mm-ready-go-linux-amd64`
- `mm-ready-go-linux-arm64`
- `mm-ready-go-darwin-amd64`
- `mm-ready-go-darwin-arm64`
- `mm-ready-go-windows-amd64.exe`

Verify the installation:

```bash
mm-ready-go --help
```

## 2. Run Your First Scan

The most common usage is a pre-Spock readiness scan against a PostgreSQL
database that does not yet have Spock installed.

The tool applies sensible defaults when you omit optional flags. If you do not
specify `--format`, `--output`, or a subcommand:

- The subcommand defaults to **scan**
- The format defaults to **HTML**
- The report is saved to the **`reports/`** subdirectory in your current
  working directory, named **`<dbname>_<timestamp>.html`**
  (e.g. `reports/your_database_20260127_131504.html`)
- Timestamps in the filename mean you can re-run without overwriting
  previous results

So the simplest invocation is:

```bash
mm-ready-go \
  --host your-db-host \
  --port 5432 \
  --dbname your_database \
  --user your_user \
  --password your_password
```

Or use a connection URI:

```bash
mm-ready-go --dsn "postgresql://user:password@host:5432/dbname"
```

Or rely on standard PostgreSQL environment variables -- any `PG*` variable
(`PGHOST`, `PGPORT`, `PGDATABASE`, `PGUSER`, `PGPASSWORD`) is used as a
fallback when the corresponding CLI flag is not provided:

```bash
export PGHOST=your-db-host PGDATABASE=your_database PGUSER=your_user
export PGPASSWORD=your_password
mm-ready-go
```

For SSL connections, use `--sslmode`, `--sslcert`, `--sslkey`, and
`--sslrootcert` (or their `PGSSLMODE`, `PGSSLCERT`, `PGSSLKEY`,
`PGSSLROOTCERT` environment variable equivalents):

```bash
mm-ready-go scan --host db.example.com --dbname myapp --user postgres \
  --sslmode verify-full --sslrootcert /path/to/ca.crt
```

You can override any of the defaults explicitly:

```bash
mm-ready-go scan \
  --host your-db-host \
  --dbname your_database \
  --user your_user \
  --password your_password \
  --format json \
  --output /path/to/report.json
```

## 3. Read the Report

Open the HTML report in a browser. You will see three main sections:

- **Summary table** — total checks run, passed, and counts by severity
- **Readiness verdict** — READY, CONDITIONALLY READY, or NOT READY
- **Findings** — grouped by severity (CRITICAL first), then by category

Each finding includes:

- What was found
- Why it matters for Spock multi-master replication
- How to fix it

## 4. Understanding Severity

| Level | Action Required |
|-------|----------------|
| **CRITICAL** | Must fix before installing Spock. These will cause data loss or replication failure. |
| **WARNING** | Should fix or review. May cause issues in production multi-master operation. |
| **CONSIDER** | Should be investigated. May need action depending on your specific use case. |
| **INFO** | Awareness items. No action required, but good to know. |

## 5. Common Critical Findings

The findings below appear most frequently and must be resolved before
installing Spock.

### wal_level is not 'logical'

Spock requires logical decoding. Fix with:

```sql
ALTER SYSTEM SET wal_level = 'logical';
-- Restart PostgreSQL
```

### track_commit_timestamp is off

Required for Spock's conflict resolution:

```sql
ALTER SYSTEM SET track_commit_timestamp = on;
-- Restart PostgreSQL
```

### Tables with UPDATE/DELETE activity but no primary key

This is the most dangerous finding. Spock places tables without primary keys
into the `default_insert_only` replication set, where **UPDATE and DELETE
operations are silently dropped**. If your table receives updates/deletes,
those changes will be lost on other nodes.

Fix: add a primary key to the table.

## 6. Output Formats

Three output formats are available, each suited to a different workflow.

```bash
# HTML (default — best for viewing in a browser)
mm-ready-go scan ...

# JSON (best for programmatic consumption)
mm-ready-go scan ... --format json

# Markdown (best for pasting into tickets/docs)
mm-ready-go scan ... --format markdown
```

## 7. Filter by Category

Run only specific check categories:

```bash
mm-ready-go scan ... --categories schema,replication
```

Available categories: `schema`, `replication`, `config`, `extensions`,
`sql_patterns`, `functions`, `sequences`.

See [Checks Reference](checks-reference.md) for full details on every check.

## 8. Configuration File

mm-ready-go supports an optional YAML configuration file. By default it looks
for `mm-ready.yaml` in the current directory. Override the path with `--config`:

```bash
mm-ready-go scan --config /path/to/mm-ready.yaml ...
```

Example configuration:

```yaml
checks:
  exclude:
    - temp_tables
    - temp_table_queries

report:
  todo_list: true
  todo_include_consider: false
```

You can also apply mode-specific check filtering:

```yaml
mode_checks:
  scan:
    exclude:
      - advisory_locks
  audit:
    include_only:
      - subscription_health
      - conflict_log
```

## 9. Audit Mode (Post-Spock)

If Spock is already installed, use audit mode to check for operational issues:

```bash
mm-ready-go audit \
  --host your-db-host --dbname your_database \
  --user your_user --password your_password
```

Audit mode runs checks specific to Spock installations:

- Replication set membership (are all tables being replicated?)
- Subscription health (any disabled or stalled subscriptions?)
- Conflict log analysis (how many conflicts, on which tables?)
- Exception log analysis (any apply errors causing data divergence?)
- Spock GUC settings (conflict resolution strategy, logging)
- shared_preload_libraries (is Spock loaded?)

## 10. Monitor Mode

Observe database activity over a time window. This snapshots
`pg_stat_statements` at the start and end of the window to identify SQL
patterns (DDL, TRUNCATE, advisory locks, etc.) that emerged during the
observation period.

```bash
# Observe for 5 minutes then generate a report
mm-ready-go monitor \
  --host your-db-host --dbname your_database \
  --user your_user --password your_password \
  --duration 300
```

The `--duration` flag sets the observation window in seconds (default: 3600 /
1 hour). Requires `pg_stat_statements` to be installed — if it is not
available, the observation phase is skipped gracefully.

## 11. List All Checks

View every available check and the mode it belongs to.

```bash
mm-ready-go list-checks              # All checks (scan + audit)
mm-ready-go list-checks --mode scan  # Pre-Spock checks only
mm-ready-go list-checks --mode audit # Post-Spock checks only
```

## 12. Recommended Pre-Spock Workflow

1. **Run a scan** against your production database (read-only, safe to run)
2. **Fix all CRITICAL findings** — these will prevent Spock from working
3. **Review all WARNING findings** — fix or document accepted risks
4. **Install pg_stat_statements** if not already present — enables SQL pattern
   analysis for richer findings
5. **Re-run the scan** to confirm all critical issues resolved
6. **Proceed with Spock installation**
7. **Run audit mode** after Spock is installed to verify health

## Next Steps

- [Tutorial](tutorial.md) — a guided walkthrough covering scan, audit, and
  analyze modes in detail
- [Checks Reference](checks-reference.md) — full documentation for every
  check, including severity, category, and remediation guidance
- [Architecture](architecture.md) — an overview of the mm-ready-go codebase
  and how the scanner, registry, and reporters fit together

## Troubleshooting

Below are solutions to the most common issues encountered when running
mm-ready-go.

### Connection refused

Ensure the database host is reachable, the port is correct, and `pg_hba.conf`
allows your client IP. If connecting to a Docker container, check that
`listen_addresses` is set to `'*'` (not just `localhost`).

### pg_stat_statements unavailable

Some checks require `pg_stat_statements`. To enable it:

```sql
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
-- Restart PostgreSQL
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

Checks that need it will gracefully degrade and report what they could not
analyze.

### Permission denied on pg_hba_file_rules

The `hba_config` check reads `pg_hba_file_rules`, which requires superuser
or `pg_read_all_settings` privileges. Grant the role or accept that this
particular check will report an error.

### mm-ready-go command not found

Ensure `$GOPATH/bin` (or `$HOME/go/bin`) is in your `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Or build from source and run the binary directly:

```bash
make build
./bin/mm-ready-go --help
```
