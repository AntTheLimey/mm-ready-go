// Package analyzer provides offline schema dump analysis for mm-ready.
package analyzer

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/AntTheLimey/mm-ready/internal/parser"
)

// Extension known issues map
var extensionKnownIssues = map[string]string{
	"postgis": "PostGIS is compatible with Spock but requires identical versions on all nodes. " +
		"Spatial indexes and topology objects need careful replication planning.",
	"pg_partman": "pg_partman manages partitions via background worker. Ensure partition maintenance " +
		"runs on all nodes or is replicated through DDL replication.",
	"pgcrypto":   "pgcrypto is compatible. Ensure encryption keys are identical across nodes.",
	"pg_trgm":    "pg_trgm provides trigram similarity functions. Compatible with Spock.",
	"btree_gist": "btree_gist provides GiST operator classes. Compatible with Spock, but exclusion " +
		"constraints using these operators are evaluated locally per node.",
	"btree_gin": "btree_gin provides GIN operator classes. Compatible with Spock.",
	"hstore":    "hstore is compatible. Ensure the extension is installed on all nodes.",
	"ltree":     "ltree is compatible. Ensure the extension is installed on all nodes.",
	"citext":    "citext is compatible. Ensure the extension is installed on all nodes.",
	"lo": "The 'lo' extension manages large object references but does not solve logical " +
		"replication of large objects. Consider LOLOR instead.",
	"pg_stat_statements": "pg_stat_statements is a monitoring extension. Node-local — not replicated.",
	"dblink": "dblink allows cross-database queries. Connections are node-local; ensure " +
		"connection strings are valid on all nodes.",
	"postgres_fdw": "postgres_fdw provides foreign data wrappers. Foreign tables are not replicated. " +
		"Ensure FDW configurations are set up on each node.",
	"file_fdw":    "file_fdw reads from local files. Node-local — file paths must exist on each node.",
	"timescaledb": "TimescaleDB has its own replication mechanisms that may conflict with Spock. " +
		"Co-existence is not supported.",
	"citus":       "Citus distributed tables are incompatible with Spock logical replication.",
	"pgstattuple": "pgstattuple provides tuple-level statistics functions. " +
		"Monitoring-only extension, compatible with Spock.",
}

var extensionWarningNames = map[string]bool{
	"timescaledb": true,
	"citus":       true,
	"lo":          true,
}

// Volatile default patterns
var volatilePatterns = []string{
	"now()", "current_timestamp", "current_date", "current_time",
	"clock_timestamp()", "statement_timestamp()", "transaction_timestamp()",
	"timeofday()", "random()", "gen_random_uuid()", "uuid_generate_",
	"pg_current_xact_id()",
}

// Numeric suspect patterns
var numericSuspectPatterns = []string{
	"count", "total", "sum", "balance", "quantity", "qty", "amount",
	"tally", "counter", "num_", "cnt", "running_", "cumulative",
	"aggregate", "accrued", "inventory",
}

var numericTypes = map[string]bool{
	"integer": true, "bigint": true, "smallint": true, "numeric": true,
	"real": true, "double precision": true, "int": true, "int4": true,
	"int8": true, "int2": true, "float4": true, "float8": true, "decimal": true,
}

// Spock 5 supported PG majors
var supportedPgMajors = map[int]bool{15: true, 16: true, 17: true, 18: true}

// nextval extraction regex
var reNextval = regexp.MustCompile(`(?i)nextval\('([^']+)'`)

// CheckFunc is the signature for a static analysis check function.
type CheckFunc func(schema *parser.ParsedSchema, checkName, category string) []models.Finding

// checkPrimaryKeys identifies tables without primary keys.
func checkPrimaryKeys(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	// Build set of tables with PKs
	pkTables := make(map[string]bool)
	for _, c := range schema.Constraints {
		if c.ConstraintType == "PRIMARY KEY" {
			key := c.TableSchema + "." + c.TableName
			pkTables[key] = true
		}
	}

	for _, tbl := range schema.Tables {
		if tbl.PartitionBy != "" {
			continue // skip partitioned parent tables
		}
		fqn := tbl.SchemaName + "." + tbl.TableName
		if !pkTables[fqn] {
			findings = append(findings, models.Finding{
				Severity:  models.SeverityWarning,
				CheckName: checkName,
				Category:  category,
				Title:     fmt.Sprintf("Table '%s' has no primary key", fqn),
				Detail: fmt.Sprintf("Table '%s' lacks a primary key. Spock automatically places "+
					"tables without primary keys into the 'default_insert_only' "+
					"replication set. In this set, only INSERT and TRUNCATE operations "+
					"are replicated — UPDATE and DELETE operations are silently filtered "+
					"out by the Spock output plugin and never sent to subscribers.", fqn),
				ObjectName: fqn,
				Remediation: fmt.Sprintf("Add a primary key to '%s' if UPDATE/DELETE replication is "+
					"needed. If the table is genuinely insert-only (e.g. an event log), "+
					"no action is required — it will replicate correctly in the "+
					"default_insert_only replication set.", fqn),
			})
		}
	}
	return findings
}

// checkSequencePKs identifies PK columns backed by standard sequences.
func checkSequencePKs(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, pk := range schema.Constraints {
		if pk.ConstraintType != "PRIMARY KEY" {
			continue
		}
		tbl := schema.GetTable(pk.TableSchema, pk.TableName)
		if tbl == nil {
			continue
		}
		fqn := pk.TableSchema + "." + pk.TableName

		for _, colName := range pk.Columns {
			var col *parser.ColumnDef
			for i := range tbl.Columns {
				if tbl.Columns[i].Name == colName {
					col = &tbl.Columns[i]
					break
				}
			}
			if col == nil {
				continue
			}

			var seqName string
			isSequenceBacked := false

			// Check for identity column
			if col.Identity != "" {
				isSequenceBacked = true
				seqName = fmt.Sprintf("%s_%s_seq (identity)", tbl.TableName, colName)
			} else if col.DefaultExpr != "" && strings.Contains(strings.ToLower(col.DefaultExpr), "nextval(") {
				isSequenceBacked = true
				if m := reNextval.FindStringSubmatch(col.DefaultExpr); m != nil {
					seqName = m[1]
				} else {
					seqName = col.DefaultExpr
				}
			}

			if isSequenceBacked {
				findings = append(findings, models.Finding{
					Severity:  models.SeverityCritical,
					CheckName: checkName,
					Category:  category,
					Title:     fmt.Sprintf("PK column '%s.%s' uses a standard sequence", fqn, colName),
					Detail: fmt.Sprintf("Primary key column '%s' on table '%s' is backed by "+
						"sequence '%s'. In a multi-master setup, "+
						"standard sequences will produce conflicting values across nodes. "+
						"Must migrate to pgEdge snowflake sequences.", colName, fqn, seqName),
					ObjectName: fqn,
					Remediation: fmt.Sprintf("Convert '%s.%s' to use the pgEdge snowflake extension "+
						"for globally unique ID generation. See: pgEdge snowflake documentation.", fqn, colName),
					Metadata: map[string]any{"column": colName, "sequence": seqName},
				})
			}
		}
	}
	return findings
}

// checkForeignKeys identifies FK constraints and warns about CASCADE actions.
func checkForeignKeys(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding
	var fkConstraints []parser.ConstraintDef
	var cascadeFKs []parser.ConstraintDef

	for _, c := range schema.Constraints {
		if c.ConstraintType == "FOREIGN KEY" {
			fkConstraints = append(fkConstraints, c)
		}
	}

	for _, fk := range fkConstraints {
		fqn := fk.TableSchema + "." + fk.TableName
		refFqn := "unknown"
		if fk.RefTable != "" {
			refFqn = fk.RefSchema + "." + fk.RefTable
		}

		if fk.OnDelete == "CASCADE" || fk.OnUpdate == "CASCADE" {
			cascadeFKs = append(cascadeFKs, fk)
			findings = append(findings, models.Finding{
				Severity:  models.SeverityWarning,
				CheckName: checkName,
				Category:  category,
				Title:     fmt.Sprintf("CASCADE foreign key '%s' on '%s'", fk.Name, fqn),
				Detail: fmt.Sprintf("Foreign key '%s' on '%s' references '%s' with "+
					"ON DELETE %s / ON UPDATE %s. CASCADE actions are "+
					"executed locally on each node, meaning the cascaded changes happen "+
					"independently on provider and subscriber, which can lead to conflicts "+
					"in a multi-master setup.", fk.Name, fqn, refFqn, fk.OnDelete, fk.OnUpdate),
				ObjectName: fqn,
				Remediation: "Review CASCADE behavior. In multi-master, consider handling cascades " +
					"in application logic or ensuring operations flow through a single node.",
				Metadata: map[string]any{"constraint": fk.Name, "references": refFqn},
			})
		}
	}

	if len(fkConstraints) > 0 {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("Database has %d foreign key constraint(s)", len(fkConstraints)),
			Detail: fmt.Sprintf("Found %d foreign key constraints. Ensure all referenced tables "+
				"are included in the replication set, and that replication ordering will "+
				"satisfy referential integrity.", len(fkConstraints)),
			ObjectName:  "(database)",
			Remediation: "Ensure all FK-related tables are in the same replication set.",
			Metadata:    map[string]any{"fk_count": len(fkConstraints), "cascade_count": len(cascadeFKs)},
		})
	}

	return findings
}

// checkDeferrableConstraints identifies deferrable PK/UNIQUE constraints.
func checkDeferrableConstraints(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, con := range schema.Constraints {
		if con.ConstraintType != "PRIMARY KEY" && con.ConstraintType != "UNIQUE" {
			continue
		}
		if !con.Deferrable {
			continue
		}

		fqn := con.TableSchema + "." + con.TableName
		conLabel := con.ConstraintType
		severity := models.SeverityWarning
		if con.ConstraintType == "PRIMARY KEY" {
			severity = models.SeverityCritical
		}

		initiallyStr := "IMMEDIATE"
		if con.InitiallyDeferred {
			initiallyStr = "DEFERRED"
		}

		findings = append(findings, models.Finding{
			Severity:  severity,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("Deferrable %s '%s' on '%s'", conLabel, con.Name, fqn),
			Detail: fmt.Sprintf("Table '%s' has a DEFERRABLE %s constraint "+
				"'%s' (initially %s). "+
				"Spock's conflict resolution checks indimmediate on indexes via "+
				"IsIndexUsableForInsertConflict() and silently SKIPS deferrable "+
				"indexes. This means conflicts on this constraint will NOT be "+
				"detected during replication apply, potentially causing "+
				"duplicate key violations or data inconsistencies.", fqn, conLabel, con.Name, initiallyStr),
			ObjectName: fmt.Sprintf("%s.%s", fqn, con.Name),
			Remediation: fmt.Sprintf("If possible, make the constraint non-deferrable:\n"+
				"  ALTER TABLE %s ALTER CONSTRAINT %s NOT DEFERRABLE;\n"+
				"If deferral is required by the application, be aware that Spock "+
				"will not use this constraint for conflict detection.", fqn, con.Name),
			Metadata: map[string]any{
				"constraint_type":    conLabel,
				"initially_deferred": con.InitiallyDeferred,
			},
		})
	}

	return findings
}

// checkExclusionConstraints identifies EXCLUDE constraints.
func checkExclusionConstraints(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, con := range schema.Constraints {
		if con.ConstraintType != "EXCLUDE" {
			continue
		}

		fqn := con.TableSchema + "." + con.TableName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("Exclusion constraint '%s' on '%s'", con.Name, fqn),
			Detail: fmt.Sprintf("Table '%s' has exclusion constraint '%s'. "+
				"Exclusion constraints are evaluated locally on each node. In a "+
				"multi-master topology, two nodes could independently accept rows "+
				"that would violate the exclusion constraint if evaluated globally, "+
				"leading to replication conflicts or data inconsistencies.", fqn, con.Name),
			ObjectName: fmt.Sprintf("%s.%s", fqn, con.Name),
			Remediation: "Review whether this exclusion constraint can be replaced with " +
				"application-level logic, or ensure that only one node writes data " +
				"that could conflict under this constraint.",
		})
	}

	return findings
}

// checkMissingFKIndexes identifies FK columns without supporting indexes.
func checkMissingFKIndexes(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, fk := range schema.Constraints {
		if fk.ConstraintType != "FOREIGN KEY" || len(fk.Columns) == 0 {
			continue
		}

		fqn := fk.TableSchema + "." + fk.TableName
		fkCols := fk.Columns

		// Check if any index covers these FK columns as a prefix
		indexes := schema.GetIndexesForTable(fk.TableSchema, fk.TableName)

		// Also check PK/UNIQUE constraints
		pkUkCols := make([][]string, 0)
		for _, c := range schema.Constraints {
			if c.TableSchema == fk.TableSchema && c.TableName == fk.TableName {
				if c.ConstraintType == "PRIMARY KEY" || c.ConstraintType == "UNIQUE" {
					pkUkCols = append(pkUkCols, c.Columns)
				}
			}
		}

		hasIndex := false
		for _, idx := range indexes {
			if len(idx.Columns) >= len(fkCols) && sliceEqual(idx.Columns[:len(fkCols)], fkCols) {
				hasIndex = true
				break
			}
		}

		if !hasIndex {
			for _, pkCols := range pkUkCols {
				if len(pkCols) >= len(fkCols) && sliceEqual(pkCols[:len(fkCols)], fkCols) {
					hasIndex = true
					break
				}
			}
		}

		if !hasIndex {
			colList := strings.Join(fkCols, ", ")
			findings = append(findings, models.Finding{
				Severity:  models.SeverityConsider,
				CheckName: checkName,
				Category:  category,
				Title:     fmt.Sprintf("No index on FK columns '%s' (%s)", fqn, colList),
				Detail: fmt.Sprintf("Foreign key constraint '%s' on '%s' references "+
					"columns (%s) that have no supporting index. Without "+
					"an index, DELETE and UPDATE on the referenced (parent) table "+
					"require a sequential scan of the child table while holding a "+
					"lock. In multi-master replication, this causes longer lock "+
					"hold times and increases the likelihood of conflicts.", fk.Name, fqn, colList),
				ObjectName: fqn,
				Remediation: fmt.Sprintf("Create an index:\n"+
					"  CREATE INDEX ON %s (%s);", fqn, colList),
				Metadata: map[string]any{"constraint": fk.Name, "columns": fkCols},
			})
		}
	}

	return findings
}

// checkUnloggedTables identifies UNLOGGED tables.
func checkUnloggedTables(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, tbl := range schema.Tables {
		if !tbl.Unlogged {
			continue
		}
		fqn := tbl.SchemaName + "." + tbl.TableName
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("UNLOGGED table '%s'", fqn),
			Detail: fmt.Sprintf("Table '%s' is UNLOGGED. Unlogged tables are not written to the "+
				"write-ahead log and therefore cannot be replicated by Spock. Data in "+
				"this table will exist only on the local node.", fqn),
			ObjectName: fqn,
			Remediation: fmt.Sprintf("If this table needs to be replicated, convert it: "+
				"ALTER TABLE %s SET LOGGED;", fqn),
		})
	}

	return findings
}

// checkLargeObjects identifies OID columns that may reference large objects.
func checkLargeObjects(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, tbl := range schema.Tables {
		fqn := tbl.SchemaName + "." + tbl.TableName
		for _, col := range tbl.Columns {
			if strings.ToLower(col.DataType) == "oid" {
				findings = append(findings, models.Finding{
					Severity:  models.SeverityWarning,
					CheckName: checkName,
					Category:  category,
					Title:     fmt.Sprintf("OID column '%s.%s' may reference large objects", fqn, col.Name),
					Detail: fmt.Sprintf("Column '%s' on table '%s' uses the OID data type, "+
						"which is commonly used to reference large objects. If used for LOB "+
						"references, these will not replicate through logical decoding.", col.Name, fqn),
					ObjectName: fmt.Sprintf("%s.%s", fqn, col.Name),
					Remediation: "If this column references large objects, migrate to LOLOR or " +
						"BYTEA. LOLOR requires lolor.node to be set uniquely per node " +
						"and its tables added to a replication set. " +
						"If the column is used for other purposes, this finding can be ignored.",
				})
			}
		}
	}

	return findings
}

// checkColumnDefaults identifies volatile column defaults.
func checkColumnDefaults(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, tbl := range schema.Tables {
		fqn := tbl.SchemaName + "." + tbl.TableName
		for _, col := range tbl.Columns {
			if col.DefaultExpr == "" || col.GeneratedExpr != "" {
				continue
			}

			defaultLower := strings.ToLower(col.DefaultExpr)

			// Skip nextval defaults (handled by sequence_pks)
			if strings.Contains(defaultLower, "nextval(") {
				continue
			}

			isVolatile := false
			for _, pat := range volatilePatterns {
				if strings.Contains(defaultLower, pat) {
					isVolatile = true
					break
				}
			}
			if !isVolatile {
				continue
			}

			findings = append(findings, models.Finding{
				Severity:  models.SeverityConsider,
				CheckName: checkName,
				Category:  category,
				Title:     fmt.Sprintf("Volatile default on '%s.%s'", fqn, col.Name),
				Detail: fmt.Sprintf("Column '%s' on table '%s' has a volatile default: "+
					"%s. In multi-master replication, if a row is inserted "+
					"without specifying this column, each node could compute a different "+
					"default value. However, Spock replicates the actual inserted value, "+
					"so this is only an issue if the same row is independently inserted "+
					"on multiple nodes.", col.Name, fqn, col.DefaultExpr),
				ObjectName: fmt.Sprintf("%s.%s", fqn, col.Name),
				Remediation: "Ensure the application always provides an explicit value for this column, " +
					"or accept that conflict resolution may be needed for concurrent inserts.",
				Metadata: map[string]any{"default_expr": col.DefaultExpr},
			})
		}
	}

	return findings
}

// checkNumericColumns identifies numeric columns that may be Delta-Apply candidates.
func checkNumericColumns(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, tbl := range schema.Tables {
		fqn := tbl.SchemaName + "." + tbl.TableName
		for _, col := range tbl.Columns {
			dataTypeLower := strings.ToLower(strings.TrimSpace(col.DataType))
			if !numericTypes[dataTypeLower] {
				continue
			}

			colLower := strings.ToLower(col.Name)
			isSuspect := false
			for _, pat := range numericSuspectPatterns {
				if strings.Contains(colLower, pat) {
					isSuspect = true
					break
				}
			}
			if !isSuspect {
				continue
			}

			if !col.NotNull {
				findings = append(findings, models.Finding{
					Severity:  models.SeverityWarning,
					CheckName: checkName,
					Category:  category,
					Title:     fmt.Sprintf("Delta-Apply candidate '%s.%s' allows NULL", fqn, col.Name),
					Detail: fmt.Sprintf("Column '%s' on table '%s' is numeric (%s) "+
						"and its name suggests it may be an accumulator or counter. "+
						"If configured for Delta-Apply in Spock, the column MUST have a "+
						"NOT NULL constraint. The Spock apply worker "+
						"(spock_apply_heap.c:613-627) checks this and will reject "+
						"delta-apply on nullable columns.", col.Name, fqn, col.DataType),
					ObjectName: fmt.Sprintf("%s.%s", fqn, col.Name),
					Remediation: fmt.Sprintf("If this column will use Delta-Apply, add a NOT NULL constraint:\n"+
						"  ALTER TABLE %s ALTER COLUMN %s SET NOT NULL;\n"+
						"Ensure existing rows have no NULL values first.", fqn, col.Name),
					Metadata: map[string]any{"column": col.Name, "data_type": col.DataType, "nullable": true},
				})
			} else {
				findings = append(findings, models.Finding{
					Severity:  models.SeverityConsider,
					CheckName: checkName,
					Category:  category,
					Title:     fmt.Sprintf("Potential Delta-Apply column: '%s.%s' (%s)", fqn, col.Name, col.DataType),
					Detail: fmt.Sprintf("Column '%s' on table '%s' is numeric (%s) "+
						"and its name suggests it may be an accumulator or counter. In "+
						"multi-master replication, concurrent updates to such columns can "+
						"cause conflicts. Delta-Apply can resolve this by applying the "+
						"delta (change) rather than the absolute value. This column has a "+
						"NOT NULL constraint, so it meets the Delta-Apply prerequisite.", col.Name, fqn, col.DataType),
					ObjectName: fmt.Sprintf("%s.%s", fqn, col.Name),
					Remediation: "Investigate whether this column receives concurrent " +
						"increment/decrement updates from multiple nodes. If so, " +
						"configure it for Delta-Apply in Spock.",
					Metadata: map[string]any{"column": col.Name, "data_type": col.DataType, "nullable": false},
				})
			}
		}
	}

	return findings
}

// checkMultipleUniqueIndexes identifies tables with multiple unique indexes.
func checkMultipleUniqueIndexes(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	tableUnique := make(map[string][]string) // "schema.table" -> []indexNames

	for _, idx := range schema.Indexes {
		if idx.IsUnique {
			key := idx.TableSchema + "." + idx.TableName
			tableUnique[key] = append(tableUnique[key], idx.Name)
		}
	}

	for _, con := range schema.Constraints {
		if con.ConstraintType == "PRIMARY KEY" || con.ConstraintType == "UNIQUE" {
			key := con.TableSchema + "." + con.TableName
			tableUnique[key] = append(tableUnique[key], con.Name)
		}
	}

	for fqn, indexNames := range tableUnique {
		if len(indexNames) <= 1 {
			continue
		}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("Table '%s' has %d unique indexes", fqn, len(indexNames)),
			Detail: fmt.Sprintf("Table '%s' has %d unique indexes: %s. "+
				"When check_all_uc_indexes is enabled in Spock, the apply worker "+
				"iterates all unique indexes for conflict detection and uses the "+
				"first match it finds (spock_apply_heap.c). With multiple unique "+
				"constraints, conflicts may be detected on different indexes on "+
				"different nodes, which could lead to unexpected resolution behaviour.",
				fqn, len(indexNames), strings.Join(indexNames, ", ")),
			ObjectName: fqn,
			Remediation: "Review whether all unique indexes are necessary for replication " +
				"conflict detection. Consider whether check_all_uc_indexes should " +
				"be enabled, and ensure the application can tolerate conflict " +
				"resolution on any of the unique constraints.",
			Metadata: map[string]any{"unique_index_count": len(indexNames), "indexes": indexNames},
		})
	}

	return findings
}

// checkEnumTypes identifies ENUM types.
func checkEnumTypes(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, enum := range schema.EnumTypes {
		fqn := enum.SchemaName + "." + enum.TypeName
		labels := enum.Labels

		labelsPreview := labels
		ellipsis := ""
		if len(labelsPreview) > 10 {
			labelsPreview = labelsPreview[:10]
			ellipsis = "..."
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("ENUM type '%s' (%d values)", fqn, len(labels)),
			Detail: fmt.Sprintf("ENUM type '%s' has %d values: %s%s. "+
				"In multi-master replication, ALTER TYPE ... ADD VALUE is a DDL "+
				"change that must be applied on all nodes. Spock can replicate DDL "+
				"through the ddl_sql replication set, but ENUM modifications must "+
				"be coordinated carefully to avoid type mismatches during apply.",
				fqn, len(labels), strings.Join(labelsPreview, ", "), ellipsis),
			ObjectName: fqn,
			Remediation: "Plan ENUM modifications to be applied through Spock's DDL " +
				"replication (spock.replicate_ddl) to ensure all nodes stay in sync. " +
				"Alternatively, consider using a lookup table instead of ENUMs for " +
				"values that change frequently.",
			Metadata: map[string]any{"label_count": len(labels), "labels": labels},
		})
	}

	return findings
}

// checkGeneratedColumns identifies generated/stored columns.
func checkGeneratedColumns(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, tbl := range schema.Tables {
		fqn := tbl.SchemaName + "." + tbl.TableName
		for _, col := range tbl.Columns {
			if col.GeneratedExpr == "" {
				continue
			}

			findings = append(findings, models.Finding{
				Severity:  models.SeverityConsider,
				CheckName: checkName,
				Category:  category,
				Title:     fmt.Sprintf("Generated column '%s.%s' (STORED)", fqn, col.Name),
				Detail: fmt.Sprintf("Column '%s' on table '%s' is a STORED generated column "+
					"with expression: %s. Generated columns are recomputed on the "+
					"subscriber side. If the expression depends on functions or data that "+
					"differs across nodes, values may diverge.", col.Name, fqn, col.GeneratedExpr),
				ObjectName: fmt.Sprintf("%s.%s", fqn, col.Name),
				Remediation: "Verify the generation expression produces identical results on all nodes. " +
					"Avoid expressions that depend on volatile functions or node-local state.",
				Metadata: map[string]any{"gen_type": "STORED", "expression": col.GeneratedExpr},
			})
		}
	}

	return findings
}

// checkRules identifies rules on tables.
func checkRules(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, rule := range schema.Rules {
		fqn := rule.SchemaName + "." + rule.TableName
		severity := models.SeverityConsider
		if rule.IsInstead {
			severity = models.SeverityWarning
		}

		insteadStr := ""
		if rule.IsInstead {
			insteadStr = "INSTEAD "
		}

		detail := fmt.Sprintf("Table '%s' has %srule "+
			"'%s' on %s events. "+
			"Rules rewrite queries before execution, which means the WAL "+
			"records the rewritten operations, not the original SQL. On the "+
			"subscriber side, the Spock apply worker replays the row-level "+
			"changes from WAL, and the subscriber's rules will also fire on "+
			"the applied changes — potentially causing double-application or "+
			"unexpected side effects.", fqn, func() string {
			if rule.IsInstead {
				return "an INSTEAD "
			}
			return "a "
		}(), rule.RuleName, rule.Event)

		if rule.IsInstead {
			detail += " INSTEAD rules are particularly dangerous as they completely " +
				"replace the original operation."
		}

		findings = append(findings, models.Finding{
			Severity:    severity,
			CheckName:   checkName,
			Category:    category,
			Title:       fmt.Sprintf("%sRule '%s' on '%s' (%s)", insteadStr, rule.RuleName, fqn, rule.Event),
			Detail:      detail,
			ObjectName:  fmt.Sprintf("%s.%s", fqn, rule.RuleName),
			Remediation: "Consider converting rules to triggers (which can be controlled " +
				"via session_replication_role), or disable rules on subscriber " +
				"nodes. Review whether the rule's effect should apply on both " +
				"provider and subscriber.",
			Metadata: map[string]any{"event": rule.Event, "is_instead": rule.IsInstead},
		})
	}

	return findings
}

// checkInheritance identifies table inheritance (non-partition).
func checkInheritance(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, tbl := range schema.Tables {
		if len(tbl.Inherits) == 0 {
			continue
		}
		childFqn := tbl.SchemaName + "." + tbl.TableName
		for _, parent := range tbl.Inherits {
			findings = append(findings, models.Finding{
				Severity:  models.SeverityWarning,
				CheckName: checkName,
				Category:  category,
				Title:     fmt.Sprintf("Table inheritance: '%s' inherits from '%s'", childFqn, parent),
				Detail: fmt.Sprintf("Table '%s' uses traditional table inheritance from "+
					"'%s'. Logical replication does not replicate through "+
					"inheritance hierarchies — each table is replicated independently. "+
					"Queries against the parent that include child data via inheritance "+
					"may behave differently across nodes.", childFqn, parent),
				ObjectName: childFqn,
				Remediation: "Consider migrating from table inheritance to declarative partitioning " +
					"(if appropriate) or separate standalone tables.",
				Metadata: map[string]any{"parent": parent},
			})
		}
	}

	return findings
}

// checkInstalledExtensions audits installed extensions for Spock compatibility.
func checkInstalledExtensions(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, ext := range schema.Extensions {
		extname := strings.ToLower(ext.Name)
		if issue, ok := extensionKnownIssues[extname]; ok {
			severity := models.SeverityInfo
			if extensionWarningNames[extname] {
				severity = models.SeverityWarning
			}
			remediation := ""
			if severity != models.SeverityInfo {
				remediation = issue
			}
			findings = append(findings, models.Finding{
				Severity:    severity,
				CheckName:   checkName,
				Category:    category,
				Title:       fmt.Sprintf("Extension '%s'", ext.Name),
				Detail:      issue,
				ObjectName:  ext.Name,
				Remediation: remediation,
				Metadata:    map[string]any{"schema": ext.SchemaName},
			})
		}
	}

	if len(schema.Extensions) > 0 {
		extList := make([]string, 0, len(schema.Extensions))
		for _, e := range schema.Extensions {
			extList = append(extList, e.Name)
		}
		findings = append(findings, models.Finding{
			Severity:    models.SeverityConsider,
			CheckName:   checkName,
			Category:    category,
			Title:       fmt.Sprintf("Installed extensions: %d", len(schema.Extensions)),
			Detail:      "Extensions: " + strings.Join(extList, ", "),
			ObjectName:  "(extensions)",
			Remediation: "Ensure all extensions are installed at identical versions on every node.",
			Metadata:    map[string]any{"extensions": extList},
		})
	}

	return findings
}

// checkSequenceAudit lists all sequences with ownership info.
func checkSequenceAudit(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	for _, seq := range schema.Sequences {
		fqn := seq.SchemaName + "." + seq.SequenceName
		ownership := "not owned by any column"
		if seq.OwnedByTable != "" {
			ownership = fmt.Sprintf("owned by %s.%s", seq.OwnedByTable, seq.OwnedByColumn)
		}

		startVal := "default"
		if seq.StartValue != nil {
			startVal = strconv.FormatInt(*seq.StartValue, 10)
		}
		incVal := "default"
		if seq.Increment != nil {
			incVal = strconv.FormatInt(*seq.Increment, 10)
		}
		cycleStr := "no"
		if seq.Cycle {
			cycleStr = "yes"
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("Sequence '%s' (%s, %s)", fqn, seq.DataType, ownership),
			Detail: fmt.Sprintf("Sequence '%s': type=%s, "+
				"start=%s, increment=%s, "+
				"cycle=%s, "+
				"%s. Standard sequences produce overlapping values in "+
				"multi-master setups. Must migrate to pgEdge snowflake sequences "+
				"or implement another globally-unique ID strategy.", fqn, seq.DataType,
				startVal, incVal, cycleStr, ownership),
			ObjectName: fqn,
			Remediation: fmt.Sprintf("Migrate sequence '%s' to use pgEdge snowflake for globally "+
				"unique ID generation across all cluster nodes.", fqn),
			Metadata: map[string]any{
				"data_type":    seq.DataType,
				"start":        seq.StartValue,
				"increment":    seq.Increment,
				"cycle":        seq.Cycle,
				"owner_table":  seq.OwnedByTable,
				"owner_column": seq.OwnedByColumn,
			},
		})
	}

	return findings
}

// checkSequenceDataTypes identifies sequences using smallint/integer.
func checkSequenceDataTypes(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding

	typeMaxes := map[string]int64{"smallint": 32767, "integer": 2147483647}

	for _, seq := range schema.Sequences {
		dt := strings.ToLower(seq.DataType)
		typeMax, ok := typeMaxes[dt]
		if !ok {
			continue
		}

		fqn := seq.SchemaName + "." + seq.SequenceName
		maxValue := typeMax
		if seq.MaxValue != nil {
			maxValue = *seq.MaxValue
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("Sequence '%s' uses %s (max %d)", fqn, dt, typeMax),
			Detail: fmt.Sprintf("Sequence '%s' is defined as %s with max value "+
				"%d. In a multi-master setup with pgEdge Snowflake "+
				"sequences, the ID space is partitioned across nodes and includes "+
				"a node identifier component. Smaller integer types can exhaust "+
				"their range much faster. Consider upgrading to bigint.", fqn, dt, maxValue),
			ObjectName: fqn,
			Remediation: "Alter the column and sequence to use bigint:\n" +
				"  ALTER TABLE ... ALTER COLUMN ... TYPE bigint;\n" +
				"This allows room for Snowflake-style globally unique IDs.",
			Metadata: map[string]any{
				"data_type": dt,
				"max_value": maxValue,
				"increment": seq.Increment,
			},
		})
	}

	return findings
}

// checkPgVersion checks PostgreSQL version compatibility with Spock 5.
func checkPgVersion(schema *parser.ParsedSchema, checkName, category string) []models.Finding {
	var findings []models.Finding
	versionStr := schema.PgVersion

	if versionStr == "" {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: checkName,
			Category:  category,
			Title:     "PostgreSQL version could not be determined from dump header",
			Detail: "The pg_dump file does not contain a recognizable " +
				"'Dumped from database version' header comment. " +
				"Cannot assess PostgreSQL version compatibility.",
			ObjectName:  "pg_version",
			Remediation: "Verify the PostgreSQL version manually.",
		})
		return findings
	}

	// Extract major version
	reMajor := regexp.MustCompile(`^(\d+)`)
	m := reMajor.FindStringSubmatch(versionStr)
	if m == nil {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("Unrecognized PostgreSQL version: %s", versionStr),
			Detail:    fmt.Sprintf("Could not parse major version from '%s'.", versionStr),
			ObjectName:  "pg_version",
			Remediation: "Verify the PostgreSQL version manually.",
		})
		return findings
	}

	major, _ := strconv.Atoi(m[1])

	if !supportedPgMajors[major] {
		supportedList := []string{"15", "16", "17", "18"}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityCritical,
			CheckName: checkName,
			Category:  category,
			Title:     fmt.Sprintf("PostgreSQL %d is not supported by Spock 5", major),
			Detail: fmt.Sprintf("Dump was taken from PostgreSQL %d (%s). "+
				"Spock 5 supports PostgreSQL versions: %s. "+
				"A PostgreSQL upgrade is required before Spock can be installed.",
				major, versionStr, strings.Join(supportedList, ", ")),
			ObjectName: "pg_version",
			Remediation: fmt.Sprintf("Upgrade PostgreSQL to version "+
				"18 (recommended) or any of: %s.", strings.Join(supportedList, ", ")),
			Metadata: map[string]any{"major": major, "version": versionStr},
		})
	} else {
		findings = append(findings, models.Finding{
			Severity:   models.SeverityInfo,
			CheckName:  checkName,
			Category:   category,
			Title:      fmt.Sprintf("PostgreSQL %d is supported by Spock 5", major),
			Detail:     fmt.Sprintf("Dump was taken from PostgreSQL %s, which is compatible with Spock 5.", versionStr),
			ObjectName: "pg_version",
			Metadata:   map[string]any{"major": major, "version": versionStr},
		})
	}

	return findings
}

// Helper function to compare slices
func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
