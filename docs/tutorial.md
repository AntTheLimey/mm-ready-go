# Tutorial - Hands-On Walkthrough

This tutorial walks through all three mm-ready-go modes using a
local PostgreSQL database with a deliberately problematic
schema. You will see exactly what mm-ready-go finds and how to
read the reports.

## Prerequisites

You will need the following:

- Docker Desktop running.
- mm-ready-go installed
  (`go install github.com/pgEdge/mm-ready-go@latest`) or
  built from source (`make build`).

## Set Up a Test Database

Start a local PostgreSQL instance with Docker.

Create and start the container:

```bash
# Start a PostgreSQL 18 container
docker run -d --name mmready-test \
  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mmready \
  -p 5499:5432 \
  postgres:18

# Wait for it to be ready
docker exec mmready-test pg_isready -U postgres
```

Enable `pg_stat_statements` for SQL pattern analysis:

```bash
docker exec mmready-test psql -U postgres \
  -c "ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';"
docker restart mmready-test
sleep 3

docker exec mmready-test psql -U postgres -d mmready \
  -c "CREATE EXTENSION IF NOT EXISTS pg_stat_statements;"
```

Load the test schema and workload:

```bash
docker cp tests/test_schema_setup.sql mmready-test:/tmp/schema.sql
docker exec mmready-test psql -U postgres -d mmready \
  -f /tmp/schema.sql

docker cp tests/test_workload.sql mmready-test:/tmp/workload.sql
docker exec mmready-test psql -U postgres -d mmready \
  -f /tmp/workload.sql
```

## Scan Mode - Pre-Spock Readiness Assessment

Scan mode checks a vanilla PostgreSQL database for Spock
compatibility issues. This is the primary use case - run it
before installing Spock.

Run a scan against the test database:

```bash
mm-ready-go scan \
  --host localhost --port 5499 --dbname mmready \
  --user postgres --password postgres \
  --format html --output scan-report.html -v
```

The `-v` flag prints progress as each check runs. Open the
report in a browser:

```bash
open scan-report.html   # macOS
# or: xdg-open scan-report.html  # Linux
```

### Reading the Report

The HTML report includes four main sections:

- Summary bar - severity counts (Critical / Warning /
  Consider / Info).
- Readiness verdict - READY, CONDITIONALLY READY, or NOT
  READY.
- Findings by severity - Critical first, then Warning,
  Consider, Info.
- To-Do list - actionable remediation steps at the bottom.

Every finding includes what was found, why it matters for
replication, and how to fix it. The tool queries `pg_catalog`
directly, so it is fast and sees everything including
system-level metadata.

## Analyze Mode - Offline Schema Dump Analysis

Not every customer will give you access to a running database.
Sometimes you receive a `pg_dump --schema-only` SQL file.
Analyze mode handles that scenario.

Generate a schema dump from the test database:

```bash
docker exec mmready-test pg_dump -U postgres -d mmready \
  --schema-only > customer_schema.sql
```

Run the analysis against the dump file:

```bash
mm-ready-go analyze \
  --file customer_schema.sql \
  --format html --output analyze-report.html -v
```

Analyze mode runs 19 of the 57 checks - those that can work
from schema structure alone. Checks requiring a live database
connection (GUCs, pg_stat_statements, Spock catalogs) are
marked as skipped with the reason "Requires live database
connection."

The structural findings (primary keys, constraints, foreign
keys) are identical to what a live scan produces.

## Monitor Mode - Workload Observation

Monitor mode combines a full scan with live observation of SQL
activity. It snapshots `pg_stat_statements` at the start and
end of a time window, then flags problematic query patterns in
the delta.

Run monitor mode with a short duration for testing:

```bash
mm-ready-go monitor \
  --host localhost --port 5499 --dbname mmready \
  --user postgres --password postgres \
  --duration 10 \
  --format html --output monitor-report.html -v
```

In production you would run this for a full business cycle - an
hour, a day, a week. The idea is to catch things that only show
up under real workload: TRUNCATE, DDL in application code,
advisory locks, temporary table creation.

## Clean Up

Remove the test container when you are done:

```bash
docker rm -f mmready-test
```

## Next Steps

Explore these resources to learn more:

- The [Quickstart Guide](quickstart.md) document covers
  additional scan options and configuration.
- The [Checks Reference](checks-reference.md) document
  describes all 57 checks in detail.
- The [Architecture](architecture.md) document explains
  internal design, module overview, and data flow.
