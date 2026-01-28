# Checks Reference

Complete reference for all mm-ready checks. Each check implements the
`check.Check` interface and is registered via `init()` in its source file.

Checks are organized by category. Within each category, the mode column
indicates when the check runs:
- **scan** — Pre-Spock readiness assessment (default)
- **audit** — Post-Spock health check (requires Spock installed)

---

## Schema (22 checks)

### primary_keys

| | |
|---|---|
| **File** | `internal/checks/schema/primary_keys.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Tables without primary keys — affects Spock replication behaviour |

Queries `pg_class` for user tables that have no primary key constraint.

Spock places tables without primary keys into the `default_insert_only`
replication set, where only INSERT and TRUNCATE operations are replicated.
UPDATE and DELETE are silently filtered out.

**Remediation:** Add a primary key if UPDATE/DELETE replication is needed. If
the table is genuinely insert-only (e.g. an event log), no action required.

---

### tables_update_delete_no_pk

| | |
|---|---|
| **File** | `internal/checks/schema/identity_replica.go` |
| **Mode** | scan |
| **Severity** | CRITICAL |
| **Description** | Tables with UPDATE/DELETE activity but no primary key — operations silently dropped |

Queries `pg_stat_user_tables` for tables without primary keys that have
non-zero `n_tup_upd` or `n_tup_del` counters.

This is the most dangerous finding. These tables will be placed in the
`default_insert_only` replication set, and their UPDATE/DELETE operations
will be silently dropped on other nodes — causing data loss.

Tables that are insert-only (no updates/deletes) are reported as INFO instead.

**Remediation:** Add a primary key. Note: REPLICA IDENTITY FULL is NOT a
substitute — Spock uses replication sets, not replica identity, to determine
replication behavior.

---

### deferrable_constraints

| | |
|---|---|
| **File** | `internal/checks/schema/deferrable_constraints.go` |
| **Mode** | scan |
| **Severity** | CRITICAL (PK) / WARNING (unique) |
| **Description** | Deferrable unique/PK constraints — silently skipped by Spock conflict resolution |

Queries `pg_constraint` for primary key and unique constraints where
`condeferrable = true`.

Spock's `IsIndexUsableForInsertConflict()` function skips deferrable indexes
during conflict detection. This means conflicts on deferrable constraints go
undetected, potentially causing duplicate key violations.

- **CRITICAL** for deferrable primary key constraints
- **WARNING** for deferrable unique constraints

**Remediation:** Make the constraint non-deferrable:
`ALTER TABLE t ALTER CONSTRAINT c NOT DEFERRABLE;`

---

### exclusion_constraints

| | |
|---|---|
| **File** | `internal/checks/schema/exclusion_constraints.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Exclusion constraints — not enforceable across nodes |

Queries `pg_constraint` for exclusion constraints (`contype = 'x'`).

Exclusion constraints are evaluated locally on each node. Two nodes can
independently accept rows that violate the constraint globally.

**Remediation:** Replace with application-level logic, or ensure only one node
writes data that could conflict under this constraint.

---

### foreign_keys

| | |
|---|---|
| **File** | `internal/checks/schema/foreign_keys.go` |
| **Mode** | scan |
| **Severity** | WARNING (CASCADE) / INFO (summary) |
| **Description** | Foreign key relationships — replication ordering and cross-node considerations |

Queries `pg_constraint` for foreign key constraints, including delete and
update action types.

CASCADE actions execute locally on each node independently, which can cause
conflicts in multi-master. Non-CASCADE foreign keys are reported as INFO for
awareness.

**Remediation:** Consider handling cascades in application logic or routing
cascade operations through a single node.

---

### sequence_pks

| | |
|---|---|
| **File** | `internal/checks/schema/sequence_pks.go` |
| **Mode** | scan |
| **Severity** | CRITICAL |
| **Description** | Primary keys using standard sequences — must migrate to pgEdge snowflake |

Queries `pg_constraint` and `pg_attribute` to find primary key columns backed
by sequences (via `pg_get_serial_sequence()`) or identity columns.

Standard sequences produce overlapping values when multiple nodes generate IDs
independently. Must migrate to pgEdge Snowflake for globally unique IDs.

**Remediation:** Convert the column to use the pgEdge snowflake extension.

---

### unlogged_tables

| | |
|---|---|
| **File** | `internal/checks/schema/unlogged_tables.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | UNLOGGED tables — not written to WAL, cannot be replicated |

Queries `pg_class` for tables with `relpersistence = 'u'`.

**Remediation:** Convert with `ALTER TABLE t SET LOGGED;` if replication is
needed.

---

### large_objects

| | |
|---|---|
| **File** | `internal/checks/schema/large_objects.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Large object (LOB) usage — logical decoding does not support them |

Two queries:
1. Counts rows in `pg_largeobject_metadata`
2. Finds columns with `atttypid = 'oid'::regtype` that may reference LOBs

**Remediation:** Migrate to the LOLOR extension for replication-safe large
object management, or store binary data in BYTEA columns.

---

### generated_columns

| | |
|---|---|
| **File** | `internal/checks/schema/generated_columns.go` |
| **Mode** | scan |
| **Severity** | CONSIDER |
| **Description** | Generated/stored columns — replication behavior differences |

Queries `pg_attribute` for columns where `attgenerated != ''`.

Generated columns are recomputed on the subscriber side. If expressions depend
on volatile functions or node-local state, values may diverge across nodes.

**Remediation:** Ensure generation expressions produce identical results on all
nodes.

---

### partitioned_tables

| | |
|---|---|
| **File** | `internal/checks/schema/partitioned_tables.go` |
| **Mode** | scan |
| **Severity** | CONSIDER |
| **Description** | Partitioned tables — review partition strategy for Spock compatibility |

Spock 5 supports partition replication, but partition structure must be
identical on all nodes.

**Remediation:** Ensure partition definitions are identical across nodes.

---

### inheritance

| | |
|---|---|
| **File** | `internal/checks/schema/inheritance.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Table inheritance (non-partition) — not well supported in logical replication |

Logical replication does not replicate through inheritance hierarchies.

**Remediation:** Migrate to declarative partitioning or separate standalone
tables.

---

### column_defaults

| | |
|---|---|
| **File** | `internal/checks/schema/column_defaults.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Volatile column defaults — may differ across nodes |

Detects volatile patterns: `now()`, `current_timestamp`, `random()`,
`gen_random_uuid()`, etc.

**Remediation:** Ensure the application provides explicit values, or accept
that conflict resolution may be needed.

---

### numeric_columns

| | |
|---|---|
| **File** | `internal/checks/schema/numeric_columns.go` |
| **Mode** | scan |
| **Severity** | WARNING (nullable) / CONSIDER (NOT NULL) |
| **Description** | Numeric columns that may be Delta-Apply candidates |

Spock's Delta-Apply conflict resolution requires columns to have a NOT NULL
constraint (verified in `spock_apply_heap.c:613-627`).

**Remediation:** Add NOT NULL constraint if needed, then configure for
Delta-Apply in Spock.

---

### multiple_unique_indexes

| | |
|---|---|
| **File** | `internal/checks/schema/multiple_unique_indexes.go` |
| **Mode** | scan |
| **Severity** | CONSIDER |
| **Description** | Tables with multiple unique indexes — affects conflict resolution |

**Remediation:** Review whether all unique indexes are necessary for conflict
detection.

---

### enum_types

| | |
|---|---|
| **File** | `internal/checks/schema/enum_types.go` |
| **Mode** | scan |
| **Severity** | CONSIDER |
| **Description** | ENUM types — DDL changes require multi-node coordination |

**Remediation:** Use Spock DDL replication for enum modifications, or consider
a lookup table for frequently changing values.

---

### rules

| | |
|---|---|
| **File** | `internal/checks/schema/rules.go` |
| **Mode** | scan |
| **Severity** | WARNING (INSTEAD rules) / CONSIDER (other rules) |
| **Description** | Rules on tables — can cause unexpected behaviour with logical replication |

**Remediation:** Convert rules to triggers (controllable via
`session_replication_role`) or disable rules on subscriber nodes.

---

### row_level_security

| | |
|---|---|
| **File** | `internal/checks/schema/row_level_security.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Row-level security policies — apply worker runs as superuser, bypasses RLS |

**Remediation:** Use replication sets for data filtering instead of relying on
RLS for subscriber-side restrictions.

---

### event_triggers

| | |
|---|---|
| **File** | `internal/checks/schema/event_triggers.go` |
| **Mode** | scan |
| **Severity** | WARNING (REPLICA) / CONSIDER (ALWAYS) / INFO (ORIGIN, DISABLED) |
| **Description** | Event triggers — fire on DDL events, interact with Spock DDL replication |

**Remediation:** Use ORIGIN mode for most triggers. Use ALWAYS only for
DDL-automation triggers that must fire during Spock DDL replay.

---

### notify_listen

| | |
|---|---|
| **File** | `internal/checks/schema/notify_listen.go` |
| **Mode** | scan |
| **Severity** | WARNING (functions) / CONSIDER (pg_stat_statements) |
| **Description** | LISTEN/NOTIFY usage — notifications are not replicated |

**Remediation:** Ensure listeners connect to all nodes, or implement an
application-level notification mechanism.

---

### tablespace_usage

| | |
|---|---|
| **File** | `internal/checks/schema/tablespace_usage.go` |
| **Mode** | scan |
| **Severity** | CONSIDER |
| **Description** | Non-default tablespace usage — tablespaces must exist on all nodes |

**Remediation:** Create matching tablespace names on all Spock nodes before
initializing replication.

---

### temp_tables

| | |
|---|---|
| **File** | `internal/checks/schema/temp_tables.go` |
| **Mode** | scan |
| **Severity** | INFO |
| **Description** | Functions creating temporary tables — session-local, never replicated |

**Remediation:** No action needed if temp table usage is intentional and
node-local.

---

### missing_fk_indexes

| | |
|---|---|
| **File** | `internal/checks/schema/missing_fk_indexes.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Foreign key columns without indexes — slow cascades and lock contention |

**Remediation:** Create an index on the referencing column(s):
```sql
CREATE INDEX ON referencing_table (fk_column);
```

---

## Replication (12 checks)

### wal_level

| | |
|---|---|
| **File** | `internal/checks/replication/wal_level.go` |
| **Mode** | scan |
| **Severity** | CRITICAL |
| **Description** | wal_level must be 'logical' for Spock replication |

**Remediation:**
```sql
ALTER SYSTEM SET wal_level = 'logical';
-- Restart PostgreSQL
```

---

### max_replication_slots

| | |
|---|---|
| **File** | `internal/checks/replication/max_replication_slots.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Sufficient replication slots for Spock node connections |

**Remediation:** Set `max_replication_slots` to at least 10 and restart.

---

### max_worker_processes

| | |
|---|---|
| **File** | `internal/checks/replication/max_worker_processes.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Sufficient worker processes for Spock background workers |

**Remediation:** Set `max_worker_processes` to at least 16 and restart.

---

### max_wal_senders

| | |
|---|---|
| **File** | `internal/checks/replication/max_wal_senders.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Sufficient WAL senders for Spock logical replication |

**Remediation:** Set `max_wal_senders` to at least 10 and restart.

---

### database_encoding

| | |
|---|---|
| **File** | `internal/checks/replication/database_encoding.go` |
| **Mode** | scan |
| **Severity** | CONSIDER (non-UTF8) / INFO (UTF-8) |
| **Description** | Database encoding — all Spock nodes must use the same encoding |

**Remediation:** Ensure all nodes use the same encoding. Prefer UTF-8.

---

### multiple_databases

| | |
|---|---|
| **File** | `internal/checks/replication/multiple_databases.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | More than one user database — Spock supports one DB per instance |

**Remediation:** Separate databases into individual PostgreSQL instances.

---

### hba_config

| | |
|---|---|
| **File** | `internal/checks/replication/hba_config.go` |
| **Mode** | scan |
| **Severity** | WARNING (no entries) / CONSIDER (cannot read) / INFO (entries found) |
| **Description** | pg_hba.conf must allow replication connections between nodes |

**Remediation:** Add replication entries:
```
host replication spock_user 0.0.0.0/0 scram-sha-256
```

---

### repset_membership

| | |
|---|---|
| **File** | `internal/checks/replication/repset_membership.go` |
| **Mode** | audit |
| **Severity** | WARNING |
| **Description** | Tables not in any Spock replication set |

**Remediation:**
```sql
SELECT spock.repset_add_table('default', 'schema.table');
```

---

### subscription_health

| | |
|---|---|
| **File** | `internal/checks/replication/sub_health.go` |
| **Mode** | audit |
| **Severity** | CRITICAL (disabled) / WARNING (inactive slot) / INFO (no subscriptions) |
| **Description** | Health of Spock subscriptions |

**Remediation:** Re-enable with
`SELECT spock.alter_subscription_enable('name');`

---

### conflict_log

| | |
|---|---|
| **File** | `internal/checks/replication/conflict_log.go` |
| **Mode** | audit |
| **Severity** | WARNING |
| **Description** | Spock conflict history analysis |

**Remediation:** Review conflict patterns and adjust conflict resolution
strategy or data access patterns.

---

### exception_log

| | |
|---|---|
| **File** | `internal/checks/replication/exception_log.go` |
| **Mode** | audit |
| **Severity** | CRITICAL |
| **Description** | Spock exception (apply error) log analysis |

**Remediation:** Review `exception_log_detail` for full row data. Resolve the
underlying issue and manually fix affected rows.

---

### stale_replication_slots

| | |
|---|---|
| **File** | `internal/checks/replication/stale_replication_slots.go` |
| **Mode** | audit |
| **Severity** | CRITICAL (>1 GB retained) / WARNING (>100 MB) / INFO (healthy) |
| **Description** | Inactive replication slots retaining WAL — can cause disk exhaustion |

**Remediation:** Investigate why the slot is inactive. If the subscriber is
permanently gone, drop the slot:
```sql
SELECT pg_drop_replication_slot('slot_name');
```

---

## Config (8 checks)

### pg_version

| | |
|---|---|
| **File** | `internal/checks/config/pg_version.go` |
| **Mode** | scan |
| **Severity** | CRITICAL (unsupported) / INFO (supported) |
| **Description** | PostgreSQL version compatibility with Spock 5 |

Spock 5 supports PostgreSQL **15, 16, 17, 18**.

**Remediation:** Upgrade to a supported PostgreSQL version.

---

### track_commit_timestamp

| | |
|---|---|
| **File** | `internal/checks/config/track_commit_ts.go` |
| **Mode** | scan |
| **Severity** | CRITICAL |
| **Description** | track_commit_timestamp must be on for Spock conflict resolution |

**Remediation:**
```sql
ALTER SYSTEM SET track_commit_timestamp = on;
-- Restart PostgreSQL
```

---

### parallel_apply

| | |
|---|---|
| **File** | `internal/checks/config/parallel_apply.go` |
| **Mode** | scan |
| **Severity** | WARNING / CONSIDER / INFO |
| **Description** | Parallel apply worker configuration for Spock performance |

**Remediation:** Set `max_logical_replication_workers >= 4`.

---

### shared_preload_libraries

| | |
|---|---|
| **File** | `internal/checks/config/shared_preload.go` |
| **Mode** | audit |
| **Severity** | CRITICAL |
| **Description** | shared_preload_libraries must include 'spock' |

**Remediation:** Add `spock` to `shared_preload_libraries` and restart.

---

### spock_gucs

| | |
|---|---|
| **File** | `internal/checks/config/spock_gucs.go` |
| **Mode** | audit |
| **Severity** | WARNING / INFO |
| **Description** | Spock-specific GUC settings |

**Remediation:** `ALTER SYSTEM SET spock.conflict_resolution = 'last_update_wins';`

---

### timezone_config

| | |
|---|---|
| **File** | `internal/checks/config/timezone_config.go` |
| **Mode** | scan |
| **Severity** | WARNING (non-UTC) / CONSIDER (log_timezone) / INFO (UTC) |
| **Description** | Timezone settings — UTC recommended for consistent commit timestamps |

**Remediation:**
```sql
ALTER SYSTEM SET timezone = 'UTC';
SELECT pg_reload_conf();
```

---

### idle_transaction_timeout

| | |
|---|---|
| **File** | `internal/checks/config/idle_tx_timeout.go` |
| **Mode** | scan |
| **Severity** | CONSIDER |
| **Description** | Idle-in-transaction timeout — long-idle transactions block VACUUM and cause bloat |

**Remediation:**
```sql
ALTER SYSTEM SET idle_in_transaction_session_timeout = '300s';
SELECT pg_reload_conf();
```

---

### pg_minor_version

| | |
|---|---|
| **File** | `internal/checks/config/pg_minor_version.go` |
| **Mode** | audit |
| **Severity** | INFO |
| **Description** | PostgreSQL minor version — all nodes should run the same minor version |

**Remediation:** Plan a coordinated minor version upgrade across all nodes.

---

## Extensions (4 checks)

### installed_extensions

| | |
|---|---|
| **File** | `internal/checks/extensions/installed_extensions.go` |
| **Mode** | scan |
| **Severity** | WARNING (problematic) / CONSIDER (summary) / INFO (compatible) |
| **Description** | Installed extensions with Spock compatibility notes |

**Remediation:** Ensure all extensions are installed at identical versions on
every node.

---

### snowflake_check

| | |
|---|---|
| **File** | `internal/checks/extensions/snowflake_ext.go` |
| **Mode** | scan |
| **Severity** | WARNING / CONSIDER / INFO |
| **Description** | pgEdge snowflake extension availability and node configuration |

**Remediation:** Install the snowflake extension and set `snowflake.node`.

---

### pg_stat_statements_check

| | |
|---|---|
| **File** | `internal/checks/extensions/pgstat_statements.go` |
| **Mode** | scan |
| **Severity** | WARNING / CONSIDER / INFO |
| **Description** | pg_stat_statements availability for SQL pattern analysis |

**Remediation:**
```sql
ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';
-- Restart PostgreSQL
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
```

---

### lolor_check

| | |
|---|---|
| **File** | `internal/checks/extensions/lolor_check.go` |
| **Mode** | scan |
| **Severity** | WARNING / INFO |
| **Description** | LOLOR extension for large object replication |

**Remediation:**
```sql
CREATE EXTENSION lolor;
ALTER SYSTEM SET lolor.node = <unique_id>;
```

---

## SQL Patterns (5 checks)

All SQL pattern checks query `pg_stat_statements` for problematic query
patterns. They gracefully skip if `pg_stat_statements` is not available.

### truncate_cascade

| | |
|---|---|
| **File** | `internal/checks/sql_patterns/truncate_cascade.go` |
| **Mode** | scan |
| **Severity** | WARNING (CASCADE) / CONSIDER (RESTART IDENTITY) |
| **Description** | TRUNCATE CASCADE and RESTART IDENTITY replication caveats |

**Remediation:**
- CASCADE: List all dependent tables explicitly in the TRUNCATE statement
- RESTART IDENTITY: Avoid with standard sequences, or switch to Snowflake IDs

---

### ddl_statements

| | |
|---|---|
| **File** | `internal/checks/sql_patterns/ddl_statements.go` |
| **Mode** | scan |
| **Severity** | INFO |
| **Description** | DDL statements found in query history |

**Remediation:** Enable AutoDDL:
```sql
ALTER SYSTEM SET spock.enable_ddl_replication = on;
```

---

### advisory_locks

| | |
|---|---|
| **File** | `internal/checks/sql_patterns/advisory_locks.go` |
| **Mode** | scan |
| **Severity** | CONSIDER |
| **Description** | Advisory lock usage — locks are node-local |

**Remediation:** Implement a distributed locking mechanism if locks are used
for application-level coordination.

---

### concurrent_indexes

| | |
|---|---|
| **File** | `internal/checks/sql_patterns/concurrent_indexes.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | CREATE INDEX CONCURRENTLY — must be created manually on each node |

**Remediation:** Execute `CREATE INDEX CONCURRENTLY` manually on each node.

---

### temp_table_queries

| | |
|---|---|
| **File** | `internal/checks/sql_patterns/temp_table_queries.go` |
| **Mode** | scan |
| **Severity** | INFO |
| **Description** | CREATE TEMP TABLE in SQL — session-local, not replicated |

**Remediation:** No action needed unless temp tables are expected to persist
across nodes.

---

## Functions (3 checks)

### stored_procedures

| | |
|---|---|
| **File** | `internal/checks/functions/stored_procedures.go` |
| **Mode** | scan |
| **Severity** | CONSIDER / INFO |
| **Description** | Stored procedures/functions with write operations or DDL |

**Remediation:** Review functions for non-replicated side effects.

---

### trigger_functions

| | |
|---|---|
| **File** | `internal/checks/functions/trigger_functions.go` |
| **Mode** | scan |
| **Severity** | WARNING (ALWAYS/REPLICA) / INFO (ORIGIN/DISABLED) |
| **Description** | Trigger enabled modes — ENABLE REPLICA and ENABLE ALWAYS fire during Spock apply |

**Remediation:** Use ORIGIN mode for most triggers. Only use REPLICA or ALWAYS
when the trigger must fire during replication apply.

---

### views_audit

| | |
|---|---|
| **File** | `internal/checks/functions/views_audit.go` |
| **Mode** | scan |
| **Severity** | WARNING (materialized views) / CONSIDER (regular views) |
| **Description** | Views and materialized views — refresh coordination |

**Remediation:** Coordinate `REFRESH MATERIALIZED VIEW` across nodes.

---

## Sequences (2 checks)

### sequence_audit

| | |
|---|---|
| **File** | `internal/checks/sequences/sequence_audit.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Full sequence inventory with types, ownership, and migration needs |

**Remediation:** Migrate to pgEdge Snowflake for globally unique IDs.

---

### sequence_data_types

| | |
|---|---|
| **File** | `internal/checks/sequences/sequence_data_types.go` |
| **Mode** | scan |
| **Severity** | WARNING |
| **Description** | Sequence data types — smallint/integer may overflow faster in multi-master |

**Remediation:** Alter columns and sequences to use `bigint` for
Snowflake-compatible globally unique IDs.
