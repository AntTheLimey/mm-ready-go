// Package analyzer provides offline schema dump analysis for mm-ready.
package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/AntTheLimey/mm-ready/internal/parser"
)

// CheckDef defines a static analysis check.
type CheckDef struct {
	Name        string
	Category    string
	Description string
	Fn          CheckFunc
}

// SkippedCheckDef defines a check that requires a live database connection.
type SkippedCheckDef struct {
	Name        string
	Category    string
	Description string
}

// StaticChecks is the list of 19 checks that can run on parsed schema.
var StaticChecks = []CheckDef{
	{"primary_keys", "schema", "Tables without primary keys", checkPrimaryKeys},
	{"sequence_pks", "schema", "Primary keys using standard sequences", checkSequencePKs},
	{"foreign_keys", "schema", "Foreign key relationships", checkForeignKeys},
	{"deferrable_constraints", "schema", "Deferrable unique/PK constraints", checkDeferrableConstraints},
	{"exclusion_constraints", "schema", "Exclusion constraints", checkExclusionConstraints},
	{"missing_fk_indexes", "schema", "Foreign key columns without indexes", checkMissingFKIndexes},
	{"unlogged_tables", "schema", "UNLOGGED tables", checkUnloggedTables},
	{"large_objects", "schema", "Large object (OID column) usage", checkLargeObjects},
	{"column_defaults", "schema", "Volatile column defaults", checkColumnDefaults},
	{"numeric_columns", "schema", "Numeric columns (Delta-Apply candidates)", checkNumericColumns},
	{"multiple_unique_indexes", "schema", "Tables with multiple unique indexes", checkMultipleUniqueIndexes},
	{"enum_types", "schema", "ENUM types", checkEnumTypes},
	{"generated_columns", "schema", "Generated/stored columns", checkGeneratedColumns},
	{"rules", "schema", "Rules on tables", checkRules},
	{"inheritance", "schema", "Table inheritance (non-partition)", checkInheritance},
	{"installed_extensions", "extensions", "Installed extensions audit", checkInstalledExtensions},
	{"sequence_audit", "sequences", "Sequence inventory and ownership", checkSequenceAudit},
	{"sequence_data_types", "sequences", "Sequence data types", checkSequenceDataTypes},
	{"pg_version", "config", "PostgreSQL version compatibility with Spock 5", checkPgVersion},
}

// SkippedChecks is the list of 37 checks that require a live database connection.
var SkippedChecks = []SkippedCheckDef{
	// Replication
	{"wal_level", "replication", "WAL level check (wal_level = logical)"},
	{"max_replication_slots", "replication", "Sufficient replication slots for Spock nodes"},
	{"max_worker_processes", "replication", "Sufficient worker processes for Spock apply workers"},
	{"max_wal_senders", "replication", "Sufficient WAL senders for replication connections"},
	{"database_encoding", "replication", "Database encoding consistency across nodes"},
	{"hba_config", "replication", "pg_hba.conf allows replication connections"},
	{"stale_replication_slots", "replication", "Check for stale replication slots"},
	{"multiple_databases", "replication", "Multiple databases in the cluster"},
	{"repset_membership", "replication", "Spock replication set membership audit"},
	{"subscription_health", "replication", "Spock subscription health check"},
	{"conflict_log", "replication", "Spock conflict log review"},
	{"exception_log", "replication", "Spock exception log review"},
	// Config (except pg_version)
	{"track_commit_timestamp", "config", "track_commit_timestamp GUC enabled"},
	{"shared_preload_libraries", "config", "shared_preload_libraries includes spock"},
	{"spock_gucs", "config", "Spock-specific GUC settings"},
	{"idle_transaction_timeout", "config", "Idle transaction timeout settings"},
	{"pg_minor_version", "config", "PostgreSQL minor version patch level"},
	{"parallel_apply", "config", "Parallel apply worker settings"},
	{"timezone_config", "config", "Timezone configuration consistency"},
	// Extensions (except installed_extensions)
	{"snowflake_check", "extensions", "pgEdge snowflake extension installation"},
	{"pg_stat_statements_check", "extensions", "pg_stat_statements availability"},
	{"lolor_check", "extensions", "LOLOR extension for large object replication"},
	// SQL patterns
	{"advisory_locks", "sql_patterns", "Advisory lock usage in queries"},
	{"ddl_statements", "sql_patterns", "DDL statements in pg_stat_statements"},
	{"truncate_cascade", "sql_patterns", "TRUNCATE CASCADE usage"},
	{"concurrent_indexes", "sql_patterns", "Concurrent index operations"},
	{"temp_table_queries", "sql_patterns", "Temporary table usage in queries"},
	// Functions
	{"stored_procedures", "functions", "Stored procedures and functions audit"},
	{"trigger_functions", "functions", "Trigger functions audit"},
	{"views_audit", "functions", "Views audit"},
	// Schema (live-only)
	{"tables_update_delete_no_pk", "schema", "UPDATE/DELETE on tables without PKs (requires pg_stat)"},
	{"row_level_security", "schema", "Row-level security policies"},
	{"partitioned_tables", "schema", "Partitioned table hierarchy"},
	{"tablespace_usage", "schema", "Non-default tablespace usage"},
	{"temp_tables", "schema", "Temporary table existence"},
	{"event_triggers", "schema", "Event triggers"},
	{"notify_listen", "schema", "NOTIFY/LISTEN channel usage"},
}

// RunAnalyze runs all static checks against a parsed schema dump.
func RunAnalyze(schema *parser.ParsedSchema, filePath string, categories []string, verbose bool) (*models.ScanReport, error) {
	// Build category filter set
	var catFilter map[string]bool
	if len(categories) > 0 {
		catFilter = make(map[string]bool)
		for _, c := range categories {
			catFilter[c] = true
		}
	}

	// Determine database name from file path
	dbName := filepath.Base(filePath)
	ext := filepath.Ext(dbName)
	if ext != "" {
		dbName = dbName[:len(dbName)-len(ext)]
	}

	// Create report
	report := &models.ScanReport{
		Database:    dbName,
		Host:        filePath,
		Port:        0,
		Timestamp:   time.Now().UTC(),
		PGVersion:   schema.PgVersion,
		SpockTarget: "5.0",
		ScanMode:    "analyze",
	}
	if report.PGVersion == "" {
		report.PGVersion = "unknown"
	}

	// Filter checks by category
	var checksToRun []CheckDef
	for _, check := range StaticChecks {
		if catFilter == nil || catFilter[check.Category] {
			checksToRun = append(checksToRun, check)
		}
	}

	total := len(checksToRun)
	if verbose {
		fmt.Fprintf(os.Stderr, "Analyze: running %d static checks against %s...\n", total, filePath)
	}

	// Run each check
	for i, check := range checksToRun {
		if verbose {
			fmt.Fprintf(os.Stderr, "  [%d/%d] %s/%s: %s\n", i+1, total, check.Category, check.Name, check.Description)
		}

		result := models.CheckResult{
			CheckName:   check.Name,
			Category:    check.Category,
			Description: check.Description,
		}

		// Run the check function with panic recovery
		func() {
			defer func() {
				if r := recover(); r != nil {
					result.Error = fmt.Sprintf("panic: %v", r)
					if verbose {
						fmt.Fprintf(os.Stderr, "    ERROR: %s\n", result.Error)
					}
				}
			}()
			result.Findings = check.Fn(schema, check.Name, check.Category)
		}()

		report.Results = append(report.Results, result)
	}

	// Add skipped checks
	for _, skip := range SkippedChecks {
		if catFilter != nil && !catFilter[skip.Category] {
			continue
		}
		report.Results = append(report.Results, models.CheckResult{
			CheckName:   skip.Name,
			Category:    skip.Category,
			Description: skip.Description,
			Skipped:     true,
			SkipReason:  "Requires live database connection",
		})
	}

	if verbose {
		fmt.Fprintf(os.Stderr, "Done. %d critical, %d warnings, %d consider, %d info.\n",
			report.CriticalCount(), report.WarningCount(), report.ConsiderCount(), report.InfoCount())
	}

	return report, nil
}
