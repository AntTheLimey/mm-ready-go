package monitor

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// LogStatement represents a notable statement found in a log file.
type LogStatement struct {
	LineNumber int
	Timestamp  string
	Statement  string
	DurationMs *float64
}

// LogAnalysis aggregates log analysis results.
type LogAnalysis struct {
	TotalStatements  int
	DDLStatements    []LogStatement
	TruncateCascade  []LogStatement
	CreateTempTable  []LogStatement
	AdvisoryLocks    []LogStatement
	ConcurrentIndexes []LogStatement
	OtherNotable     []LogStatement
}

// HasFindings returns true if any notable patterns were found.
func (a *LogAnalysis) HasFindings() bool {
	return len(a.DDLStatements) > 0 ||
		len(a.TruncateCascade) > 0 ||
		len(a.CreateTempTable) > 0 ||
		len(a.AdvisoryLocks) > 0 ||
		len(a.ConcurrentIndexes) > 0 ||
		len(a.OtherNotable) > 0
}

// Patterns for PostgreSQL log formats.
var (
	logLinePattern   = regexp.MustCompile(`(?i)^(\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}:\d{2}[.\d]*\s*\w*)\s+.*?(?:LOG|STATEMENT|ERROR):\s+(.*)`)
	statementPattern = regexp.MustCompile(`(?i)(?:statement|execute\s+\w+):\s+(.*)`)
	ddlPattern       = regexp.MustCompile(`(?i)\b(CREATE|ALTER|DROP)\s+(TABLE|INDEX|VIEW|FUNCTION|PROCEDURE|TRIGGER|TYPE|SCHEMA|SEQUENCE)\b`)
	truncateCascade  = regexp.MustCompile(`(?i)\bTRUNCATE\b.*\bCASCADE\b`)
	tempTable        = regexp.MustCompile(`(?i)\bCREATE\s+(TEMP|TEMPORARY)\s+TABLE\b`)
	advisoryLock     = regexp.MustCompile(`(?i)\bpg_(try_)?advisory_lock`)
	concurrentIndex  = regexp.MustCompile(`(?i)\bCREATE\s+INDEX\s+CONCURRENTLY\b`)
)

// ParseLogFile parses a PostgreSQL log file and extracts notable SQL patterns.
func ParseLogFile(logPath string) (*LogAnalysis, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return nil, fmt.Errorf("log file not found: %w", err)
	}
	defer f.Close()

	analysis := &LogAnalysis{}
	var currentStmt string
	var currentTS string
	var currentLine int

	scanner := bufio.NewScanner(f)
	// Increase buffer for long lines
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		match := logLinePattern.FindStringSubmatch(line)
		if match != nil {
			// Process previous statement if any
			if currentStmt != "" {
				classifyStatement(analysis, currentStmt, currentTS, currentLine)
			}

			currentTS = match[1]
			content := match[2]
			currentLine = lineNum

			// Extract statement
			stmtMatch := statementPattern.FindStringSubmatch(content)
			if stmtMatch != nil {
				currentStmt = stmtMatch[1]
				analysis.TotalStatements++
			} else {
				currentStmt = content
			}
		} else if currentStmt != "" && strings.HasPrefix(line, "\t") {
			// Continuation line
			currentStmt += " " + strings.TrimSpace(line)
		}
	}

	// Process last statement
	if currentStmt != "" {
		classifyStatement(analysis, currentStmt, currentTS, currentLine)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("log file read error: %w", err)
	}

	return analysis, nil
}

func classifyStatement(analysis *LogAnalysis, stmt, ts string, line int) {
	truncated := stmt
	if len(truncated) > 500 {
		truncated = truncated[:500]
	}
	entry := LogStatement{LineNumber: line, Timestamp: ts, Statement: truncated}

	if ddlPattern.MatchString(stmt) {
		if concurrentIndex.MatchString(stmt) {
			analysis.ConcurrentIndexes = append(analysis.ConcurrentIndexes, entry)
		} else {
			analysis.DDLStatements = append(analysis.DDLStatements, entry)
		}
	}

	if truncateCascade.MatchString(stmt) {
		analysis.TruncateCascade = append(analysis.TruncateCascade, entry)
	}

	if tempTable.MatchString(stmt) {
		analysis.CreateTempTable = append(analysis.CreateTempTable, entry)
	}

	if advisoryLock.MatchString(stmt) {
		analysis.AdvisoryLocks = append(analysis.AdvisoryLocks, entry)
	}
}
