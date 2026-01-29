// Package parser provides SQL dump parsing for offline schema analysis.
package parser

import (
	"os"
	"regexp"
	"strconv"
	"strings"
)

// Excluded schemas that should be filtered out
var excludedSchemas = map[string]bool{
	"pg_catalog":         true,
	"information_schema": true,
	"spock":              true,
	"pg_toast":           true,
}

// Compiled regex patterns
var (
	rePgVersion = regexp.MustCompile(`--\s*Dumped from database version (\S+)`)

	reSetSearchPath = regexp.MustCompile(`(?i)SELECT\s+pg_catalog\.set_config\(\s*'search_path'\s*,\s*'([^']*)'\s*|SET\s+search_path\s*=\s*(.+?)\s*;`)

	reCreateExtension = regexp.MustCompile(`(?i)CREATE\s+EXTENSION\s+(?:IF\s+NOT\s+EXISTS\s+)?(\S+)(?:\s+(?:WITH\s+)?SCHEMA\s+(\S+))?`)

	reCreateTable = regexp.MustCompile(`(?i)CREATE\s+(UNLOGGED\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w"]+(?:\.[\w"]+)?)\s*\(`)

	reCreateSequence = regexp.MustCompile(`(?i)CREATE\s+SEQUENCE\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w"]+(?:\.[\w"]+)?)`)

	reAlterAddConstraint = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?([\w"]+(?:\.[\w"]+)?)\s+ADD\s+CONSTRAINT\s+([\w"]+)\s+(PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY|EXCLUDE|CHECK)`)

	reCreateIndex = regexp.MustCompile(`(?i)CREATE\s+(UNIQUE\s+)?INDEX\s+(?:IF\s+NOT\s+EXISTS\s+)?([\w"]+)\s+ON\s+(?:ONLY\s+)?([\w"]+(?:\.[\w"]+)?)`)

	reAlterSetDefault = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?([\w"]+(?:\.[\w"]+)?)\s+ALTER\s+COLUMN\s+([\w"]+)\s+SET\s+DEFAULT\s+(.+?)\s*;`)

	reAlterAddIdentity = regexp.MustCompile(`(?i)ALTER\s+TABLE\s+(?:ONLY\s+)?([\w"]+(?:\.[\w"]+)?)\s+ALTER\s+COLUMN\s+([\w"]+)\s+ADD\s+GENERATED\s+(ALWAYS|BY\s+DEFAULT)\s+AS\s+IDENTITY`)

	reAlterSeqOwned = regexp.MustCompile(`(?i)ALTER\s+SEQUENCE\s+([\w"]+(?:\.[\w"]+)?)\s+OWNED\s+BY\s+([\w"]+(?:\.[\w"]+)?)\.([\w"]+)`)

	reCreateTypeEnum = regexp.MustCompile(`(?i)CREATE\s+TYPE\s+([\w"]+(?:\.[\w"]+)?)\s+AS\s+ENUM\s*\(`)

	reCreateRule = regexp.MustCompile(`(?i)CREATE\s+(?:OR\s+REPLACE\s+)?RULE\s+([\w"]+)\s+AS\s+ON\s+(\w+)\s+TO\s+([\w"]+(?:\.[\w"]+)?)\s+DO\s+(INSTEAD\s+)?`)

	reFkReferences = regexp.MustCompile(`(?i)REFERENCES\s+([\w"]+(?:\.[\w"]+)?)\s*\(([^)]+)\)`)
	reFkOnDelete   = regexp.MustCompile(`(?i)ON\s+DELETE\s+(CASCADE|SET\s+NULL|SET\s+DEFAULT|RESTRICT|NO\s+ACTION)`)
	reFkOnUpdate   = regexp.MustCompile(`(?i)ON\s+UPDATE\s+(CASCADE|SET\s+NULL|SET\s+DEFAULT|RESTRICT|NO\s+ACTION)`)

	// Column parsing patterns
	reColumnLine     = regexp.MustCompile(`^\s*([\w"]+)\s+(.+)$`)
	reNotNull        = regexp.MustCompile(`(?i)\bNOT\s+NULL\b`)
	reDefault        = regexp.MustCompile(`(?i)\bDEFAULT\s+(.+?)(?:\s+NOT\s+NULL|\s+NULL|\s*,?\s*$)`)
	reGenerated      = regexp.MustCompile(`(?i)\bGENERATED\s+ALWAYS\s+AS\s*\((.+?)\)\s+STORED`)
	reIdentityInline = regexp.MustCompile(`(?i)\bGENERATED\s+(ALWAYS|BY\s+DEFAULT)\s+AS\s+IDENTITY`)

	// Table constraint keywords
	reTableConstraintKw = regexp.MustCompile(`(?i)^\s*(PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY|EXCLUDE|CHECK|CONSTRAINT)\b`)

	// Dollar quote detection
	reDollarQuote = regexp.MustCompile(`(\$[\w]*\$)`)

	// Sequence options
	reSeqAs        = regexp.MustCompile(`(?i)\bAS\s+(smallint|integer|bigint)\b`)
	reSeqStart     = regexp.MustCompile(`(?i)\bSTART\s+WITH\s+(\d+)`)
	reSeqIncrement = regexp.MustCompile(`(?i)\bINCREMENT\s+BY\s+(\d+)`)
	reSeqMinValue  = regexp.MustCompile(`(?i)\bMINVALUE\s+(\d+)`)
	reSeqMaxValue  = regexp.MustCompile(`(?i)\bMAXVALUE\s+(\d+)`)
	reSeqCycle     = regexp.MustCompile(`(?i)\bCYCLE\b`)
	reSeqNoCycle   = regexp.MustCompile(`(?i)\bNO\s+CYCLE\b`)

	// INHERITS and PARTITION BY
	reInherits    = regexp.MustCompile(`(?i)\)\s*INHERITS\s*\(([^)]+)\)`)
	rePartitionBy = regexp.MustCompile(`(?i)\)\s*(?:INHERITS\s*\([^)]*\)\s*)?PARTITION\s+BY\s+(.+?)(?:\s*;|$)`)

	// Index method
	reIndexMethod = regexp.MustCompile(`(?i)\bUSING\s+(\w+)`)

	// Deferrable
	reDeferrable        = regexp.MustCompile(`(?i)\bDEFERRABLE\b`)
	reNotDeferrable     = regexp.MustCompile(`(?i)\bNOT\s+DEFERRABLE\b`)
	reInitiallyDeferred = regexp.MustCompile(`(?i)\bINITIALLY\s+DEFERRED\b`)

	// Constraint name
	reConstraintName = regexp.MustCompile(`(?i)CONSTRAINT\s+([\w"]+)\s+`)
)

// ParseDump parses a pg_dump --schema-only SQL file into a ParsedSchema.
func ParseDump(filePath string) (*ParsedSchema, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	schema := &ParsedSchema{}
	searchPath := "public"

	text := string(data)
	statements := splitStatements(text, schema, searchPath)

	for _, stmtInfo := range statements {
		processStatement(stmtInfo.stmt, stmtInfo.searchPath, schema)
	}

	return schema, nil
}

type statementInfo struct {
	stmt       string
	searchPath string
}

// splitStatements splits SQL text into statements, tracking search_path and extracting PG version.
func splitStatements(text string, schema *ParsedSchema, searchPath string) []statementInfo {
	var results []statementInfo
	var buf []string
	inDollarQuote := false
	dollarTag := ""

	lines := strings.Split(text, "\n")
	for _, line := range lines {
		stripped := strings.TrimSpace(line)

		// Extract PG version from comment header
		if strings.HasPrefix(stripped, "--") {
			if schema.PgVersion == "" {
				if m := rePgVersion.FindStringSubmatch(stripped); m != nil {
					schema.PgVersion = m[1]
				}
			}
			continue
		}

		// Skip blank lines
		if stripped == "" {
			continue
		}

		// Track search_path changes
		if m := reSetSearchPath.FindStringSubmatch(stripped); m != nil {
			spVal := m[1]
			if spVal == "" {
				spVal = m[2]
			}
			if spVal != "" {
				parts := strings.Split(spVal, ",")
				for _, p := range parts {
					p = strings.Trim(strings.TrimSpace(p), "'\"")
					if p != "" && p != "pg_catalog" {
						searchPath = p
						break
					}
				}
			}
		}

		// Handle dollar-quoted strings
		if !inDollarQuote {
			if m := reDollarQuote.FindStringSubmatch(stripped); m != nil {
				tag := m[1]
				// Check if it opens and closes on the same line
				idx := strings.Index(stripped, tag)
				restAfter := stripped[idx+len(tag):]
				if !strings.Contains(restAfter, tag) {
					inDollarQuote = true
					dollarTag = tag
				}
			}
		} else if strings.Contains(stripped, dollarTag) {
			inDollarQuote = false
			dollarTag = ""
		}

		buf = append(buf, line)

		// Statement ends at semicolon (outside dollar quotes)
		if !inDollarQuote && strings.HasSuffix(stripped, ";") {
			stmt := strings.Join(buf, "\n")
			results = append(results, statementInfo{stmt: stmt, searchPath: searchPath})
			buf = nil
		}
	}

	// Flush any remaining buffer
	if len(buf) > 0 {
		stmt := strings.Join(buf, "\n")
		results = append(results, statementInfo{stmt: stmt, searchPath: searchPath})
	}

	return results
}

// processStatement processes a single complete SQL statement.
func processStatement(stmt string, searchPath string, schema *ParsedSchema) {
	upper := strings.ToUpper(strings.TrimSpace(stmt))

	// CREATE EXTENSION
	if m := reCreateExtension.FindStringSubmatch(stmt); m != nil && strings.HasPrefix(strings.TrimSpace(upper), "CREATE") {
		name := strings.TrimSuffix(unquote(m[1]), ";")
		extSchema := "public"
		if m[2] != "" {
			extSchema = strings.TrimSuffix(unquote(m[2]), ";")
		}
		if !excludedSchemas[strings.ToLower(name)] {
			schema.Extensions = append(schema.Extensions, ExtensionDef{
				Name:       name,
				SchemaName: extSchema,
			})
		}
		return
	}

	// CREATE TYPE AS ENUM
	if m := reCreateTypeEnum.FindStringSubmatchIndex(stmt); m != nil {
		matches := reCreateTypeEnum.FindStringSubmatch(stmt)
		s, n := splitQualified(matches[1], searchPath)
		if !excludedSchemas[s] {
			content := extractParenContent(stmt[m[0]:])
			var labels []string
			for _, lbl := range strings.Split(content, ",") {
				lbl = strings.TrimSpace(lbl)
				lbl = strings.Trim(lbl, "'")
				if lbl != "" {
					labels = append(labels, lbl)
				}
			}
			schema.EnumTypes = append(schema.EnumTypes, EnumTypeDef{
				SchemaName: s,
				TypeName:   n,
				Labels:     labels,
			})
		}
		return
	}

	// CREATE SEQUENCE
	if m := reCreateSequence.FindStringSubmatch(stmt); m != nil && strings.Contains(upper, "CREATE SEQUENCE") {
		s, n := splitQualified(m[1], searchPath)
		if !excludedSchemas[s] {
			seq := SequenceDef{
				SchemaName:   s,
				SequenceName: n,
				DataType:     "bigint",
			}

			// Parse options
			if as := reSeqAs.FindStringSubmatch(stmt); as != nil {
				seq.DataType = strings.ToLower(as[1])
			}
			if start := reSeqStart.FindStringSubmatch(stmt); start != nil {
				if v, err := strconv.ParseInt(start[1], 10, 64); err == nil {
					seq.StartValue = &v
				}
			}
			if inc := reSeqIncrement.FindStringSubmatch(stmt); inc != nil {
				if v, err := strconv.ParseInt(inc[1], 10, 64); err == nil {
					seq.Increment = &v
				}
			}
			if min := reSeqMinValue.FindStringSubmatch(stmt); min != nil {
				if v, err := strconv.ParseInt(min[1], 10, 64); err == nil {
					seq.MinValue = &v
				}
			}
			if max := reSeqMaxValue.FindStringSubmatch(stmt); max != nil {
				if v, err := strconv.ParseInt(max[1], 10, 64); err == nil {
					seq.MaxValue = &v
				}
			}
			if reSeqCycle.MatchString(stmt) && !reSeqNoCycle.MatchString(stmt) {
				seq.Cycle = true
			}

			schema.Sequences = append(schema.Sequences, seq)
		}
		return
	}

	// CREATE TABLE
	if m := reCreateTable.FindStringSubmatch(stmt); m != nil {
		if !regexp.MustCompile(`(?i)^\s*CREATE\s`).MatchString(stmt) {
			return
		}
		unlogged := m[1] != ""
		s, n := splitQualified(m[2], searchPath)
		if excludedSchemas[s] {
			return
		}

		tbl := TableDef{
			SchemaName: s,
			TableName:  n,
			Unlogged:   unlogged,
		}

		// Extract column body
		body := extractParenContent(stmt)
		if body != "" {
			parseTableBody(body, &tbl, s, schema)
		}

		// Check for INHERITS
		if inh := reInherits.FindStringSubmatch(stmt); inh != nil {
			for _, p := range strings.Split(inh[1], ",") {
				tbl.Inherits = append(tbl.Inherits, strings.TrimSpace(p))
			}
		}

		// Check for PARTITION BY
		if part := rePartitionBy.FindStringSubmatch(stmt); part != nil {
			tbl.PartitionBy = strings.TrimSuffix(strings.TrimSpace(part[1]), ";")
		}

		schema.Tables = append(schema.Tables, tbl)
		return
	}

	// ALTER TABLE ADD CONSTRAINT
	if m := reAlterAddConstraint.FindStringSubmatch(stmt); m != nil {
		s, n := splitQualified(m[1], searchPath)
		if excludedSchemas[s] {
			return
		}
		conName := unquote(m[2])
		conType := strings.ToUpper(strings.ReplaceAll(m[3], "  ", " "))

		con := ConstraintDef{
			Name:           conName,
			ConstraintType: conType,
			TableSchema:    s,
			TableName:      n,
		}

		// Extract columns from parentheses after constraint type keyword
		afterType := stmt[reAlterAddConstraint.FindStringSubmatchIndex(stmt)[7]:]
		colContent := extractParenContent(afterType)
		if colContent != "" && conType != "EXCLUDE" {
			con.Columns = parseColumnList(colContent)
		}

		// FK specifics
		if conType == "FOREIGN KEY" {
			if ref := reFkReferences.FindStringSubmatch(afterType); ref != nil {
				rs, rn := splitQualified(ref[1], searchPath)
				con.RefSchema = rs
				con.RefTable = rn
				con.RefColumns = parseColumnList(ref[2])
			}
			if del := reFkOnDelete.FindStringSubmatch(afterType); del != nil {
				con.OnDelete = strings.ToUpper(strings.ReplaceAll(del[1], "  ", " "))
			}
			if upd := reFkOnUpdate.FindStringSubmatch(afterType); upd != nil {
				con.OnUpdate = strings.ToUpper(strings.ReplaceAll(upd[1], "  ", " "))
			}
		}

		// Deferrable
		if reDeferrable.MatchString(stmt) && !reNotDeferrable.MatchString(stmt) {
			con.Deferrable = true
		}
		if reInitiallyDeferred.MatchString(stmt) {
			con.InitiallyDeferred = true
		}

		schema.Constraints = append(schema.Constraints, con)
		return
	}

	// CREATE INDEX
	if m := reCreateIndex.FindStringSubmatch(stmt); m != nil {
		isUnique := m[1] != ""
		idxName := unquote(m[2])
		s, n := splitQualified(m[3], searchPath)
		if excludedSchemas[s] {
			return
		}

		idx := IndexDef{
			Name:        idxName,
			TableSchema: s,
			TableName:   n,
			IsUnique:    isUnique,
			Method:      "btree",
		}

		// Extract method
		if method := reIndexMethod.FindStringSubmatch(stmt); method != nil {
			idx.Method = strings.ToLower(method[1])
		}

		// Extract columns
		loc := reCreateIndex.FindStringSubmatchIndex(stmt)
		afterTable := stmt[loc[7]:]
		colContent := extractParenContent(afterTable)
		if colContent != "" {
			idx.Columns = parseColumnList(colContent)
		}

		schema.Indexes = append(schema.Indexes, idx)
		return
	}

	// ALTER TABLE SET DEFAULT
	if m := reAlterSetDefault.FindStringSubmatch(stmt); m != nil {
		s, n := splitQualified(m[1], searchPath)
		col := unquote(m[2])
		defaultExpr := strings.TrimSpace(m[3])
		if !excludedSchemas[s] {
			tbl := schema.GetTable(s, n)
			if tbl != nil {
				for i := range tbl.Columns {
					if tbl.Columns[i].Name == col {
						tbl.Columns[i].DefaultExpr = defaultExpr
						break
					}
				}
			}
		}
		return
	}

	// ALTER TABLE ADD GENERATED AS IDENTITY
	if m := reAlterAddIdentity.FindStringSubmatch(stmt); m != nil {
		s, n := splitQualified(m[1], searchPath)
		col := unquote(m[2])
		identityType := strings.ToUpper(strings.ReplaceAll(m[3], "  ", " "))
		if !excludedSchemas[s] {
			tbl := schema.GetTable(s, n)
			if tbl != nil {
				for i := range tbl.Columns {
					if tbl.Columns[i].Name == col {
						tbl.Columns[i].Identity = identityType
						break
					}
				}
			}
		}
		return
	}

	// ALTER SEQUENCE OWNED BY
	if m := reAlterSeqOwned.FindStringSubmatch(stmt); m != nil {
		seqS, seqN := splitQualified(m[1], searchPath)
		tblS, tblN := splitQualified(m[2], searchPath)
		col := unquote(m[3])
		for i := range schema.Sequences {
			if schema.Sequences[i].SchemaName == seqS && schema.Sequences[i].SequenceName == seqN {
				schema.Sequences[i].OwnedByTable = tblS + "." + tblN
				schema.Sequences[i].OwnedByColumn = col
				break
			}
		}
		return
	}

	// CREATE RULE
	if m := reCreateRule.FindStringSubmatch(stmt); m != nil {
		ruleName := unquote(m[1])
		event := strings.ToUpper(m[2])
		s, n := splitQualified(m[3], searchPath)
		isInstead := m[4] != ""
		if !excludedSchemas[s] {
			schema.Rules = append(schema.Rules, RuleDef{
				SchemaName: s,
				TableName:  n,
				RuleName:   ruleName,
				Event:      event,
				IsInstead:  isInstead,
			})
		}
		return
	}
}

// parseTableBody parses the body (between parens) of a CREATE TABLE statement.
func parseTableBody(body string, tbl *TableDef, searchPath string, schema *ParsedSchema) {
	parts := splitBodyParts(body)

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for inline table constraints
		if reTableConstraintKw.MatchString(part) {
			parseInlineConstraint(part, tbl, searchPath, schema)
			continue
		}

		if col := parseColumn(part); col != nil {
			tbl.Columns = append(tbl.Columns, *col)
		}
	}
}

// splitBodyParts splits CREATE TABLE body on top-level commas.
func splitBodyParts(body string) []string {
	var parts []string
	depth := 0
	var current strings.Builder

	for _, ch := range body {
		switch ch {
		case '(':
			depth++
			current.WriteRune(ch)
		case ')':
			depth--
			current.WriteRune(ch)
		case ',':
			if depth == 0 {
				parts = append(parts, current.String())
				current.Reset()
			} else {
				current.WriteRune(ch)
			}
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseColumn parses a single column definition line inside CREATE TABLE.
func parseColumn(line string) *ColumnDef {
	line = strings.TrimSuffix(strings.TrimSpace(line), ",")
	if line == "" || strings.HasPrefix(line, "--") {
		return nil
	}
	if reTableConstraintKw.MatchString(line) {
		return nil
	}

	m := reColumnLine.FindStringSubmatch(line)
	if m == nil {
		return nil
	}

	name := unquote(m[1])
	rest := m[2]

	// Skip if name looks like a keyword that starts a table constraint
	upperName := strings.ToUpper(name)
	if upperName == "PRIMARY" || upperName == "UNIQUE" || upperName == "FOREIGN" ||
		upperName == "EXCLUDE" || upperName == "CHECK" || upperName == "CONSTRAINT" {
		return nil
	}

	// Extract data type — everything up to NOT NULL / DEFAULT / GENERATED / comma
	typeEnd := len(rest)
	for _, pat := range []*regexp.Regexp{reNotNull, reDefault, reGenerated, reIdentityInline} {
		if loc := pat.FindStringIndex(rest); loc != nil && loc[0] < typeEnd {
			typeEnd = loc[0]
		}
	}

	dataType := strings.TrimSuffix(strings.TrimSpace(rest[:typeEnd]), ",")

	notNull := reNotNull.MatchString(rest)

	var defaultExpr string
	if dm := reDefault.FindStringSubmatch(rest); dm != nil {
		defaultExpr = strings.TrimSuffix(strings.TrimSpace(dm[1]), ",")
	}

	var generatedExpr string
	if gm := reGenerated.FindStringSubmatch(rest); gm != nil {
		generatedExpr = strings.TrimSpace(gm[1])
	}

	var identity string
	if im := reIdentityInline.FindStringSubmatch(rest); im != nil {
		identity = strings.ToUpper(strings.ReplaceAll(im[1], "  ", " "))
	}

	return &ColumnDef{
		Name:          name,
		DataType:      dataType,
		NotNull:       notNull,
		DefaultExpr:   defaultExpr,
		Identity:      identity,
		GeneratedExpr: generatedExpr,
	}
}

// parseInlineConstraint parses inline table-level constraints within CREATE TABLE body.
func parseInlineConstraint(text string, tbl *TableDef, searchPath string, schema *ParsedSchema) {
	// CONSTRAINT name TYPE (cols)
	var conName string
	rest := text
	if m := reConstraintName.FindStringSubmatch(text); m != nil {
		conName = unquote(m[1])
		rest = text[len(m[0]):]
	}

	restUpper := strings.ToUpper(strings.TrimSpace(rest))

	if strings.HasPrefix(restUpper, "PRIMARY KEY") {
		colContent := extractParenContent(rest)
		schema.Constraints = append(schema.Constraints, ConstraintDef{
			Name:           conName,
			ConstraintType: "PRIMARY KEY",
			TableSchema:    tbl.SchemaName,
			TableName:      tbl.TableName,
			Columns:        parseColumnList(colContent),
		})
	} else if strings.HasPrefix(restUpper, "UNIQUE") {
		colContent := extractParenContent(rest)
		con := ConstraintDef{
			Name:           conName,
			ConstraintType: "UNIQUE",
			TableSchema:    tbl.SchemaName,
			TableName:      tbl.TableName,
			Columns:        parseColumnList(colContent),
		}
		if reDeferrable.MatchString(rest) && !reNotDeferrable.MatchString(rest) {
			con.Deferrable = true
		}
		schema.Constraints = append(schema.Constraints, con)
	} else if strings.HasPrefix(restUpper, "FOREIGN KEY") {
		colContent := extractParenContent(rest)
		con := ConstraintDef{
			Name:           conName,
			ConstraintType: "FOREIGN KEY",
			TableSchema:    tbl.SchemaName,
			TableName:      tbl.TableName,
			Columns:        parseColumnList(colContent),
		}
		if ref := reFkReferences.FindStringSubmatch(rest); ref != nil {
			rs, rn := splitQualified(ref[1], searchPath)
			con.RefSchema = rs
			con.RefTable = rn
			con.RefColumns = parseColumnList(ref[2])
		}
		if del := reFkOnDelete.FindStringSubmatch(rest); del != nil {
			con.OnDelete = strings.ToUpper(del[1])
		}
		if upd := reFkOnUpdate.FindStringSubmatch(rest); upd != nil {
			con.OnUpdate = strings.ToUpper(upd[1])
		}
		schema.Constraints = append(schema.Constraints, con)
	}
}

// Helper functions

// unquote strips double-quote wrappers from identifiers.
func unquote(name string) string {
	if strings.HasPrefix(name, "\"") && strings.HasSuffix(name, "\"") {
		return name[1 : len(name)-1]
	}
	return name
}

// splitQualified splits 'schema.name' or just 'name' into (schema, name).
func splitQualified(name string, defaultSchema string) (string, string) {
	if idx := strings.Index(name, "."); idx >= 0 {
		return unquote(name[:idx]), unquote(name[idx+1:])
	}
	return defaultSchema, unquote(name)
}

// parseColumnList parses '(col1, col2)' or 'col1, col2' into a list of column names.
func parseColumnList(text string) []string {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "(")
	text = strings.TrimSuffix(text, ")")

	var cols []string
	for _, c := range strings.Split(text, ",") {
		c = strings.TrimSpace(c)
		if c != "" {
			cols = append(cols, unquote(c))
		}
	}
	return cols
}

// extractParenContent extracts content inside first balanced parentheses.
func extractParenContent(text string) string {
	depth := 0
	start := -1
	for i, ch := range text {
		switch ch {
		case '(':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ')':
			depth--
			if depth == 0 && start >= 0 {
				return text[start:i]
			}
		}
	}
	return ""
}
