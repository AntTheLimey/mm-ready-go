package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// ExceptionLogCheck reviews the Spock exception log for apply errors.
type ExceptionLogCheck struct{}

func init() {
	check.Register(&ExceptionLogCheck{})
}

func (c *ExceptionLogCheck) Name() string     { return "exception_log" }
func (c *ExceptionLogCheck) Category() string  { return "replication" }
func (c *ExceptionLogCheck) Description() string {
	return "Review Spock exception log for replication apply errors"
}
func (c *ExceptionLogCheck) Mode() string { return "audit" }

func (c *ExceptionLogCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	// Check if spock.exception_log table exists
	var hasTable bool
	err := conn.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1
			FROM pg_tables
			WHERE schemaname = 'spock'
			  AND tablename = 'exception_log'
		);
	`).Scan(&hasTable)
	if err != nil {
		hasTable = false
	}

	if !hasTable {
		return []models.Finding{{
			Severity:  models.SeverityInfo,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "No spock.exception_log table found",
			Detail: "The spock.exception_log table does not exist. This is " +
				"normal if Spock is not installed or exception logging is not configured.",
			ObjectName: "spock.exception_log",
		}}, nil
	}

	// Get exception summary
	query := `
		SELECT
			remote_origin AS origin,
			table_name,
			error_message,
			count(*) AS error_count,
			max(exception_time)::text AS last_error
		FROM spock.exception_log
		GROUP BY remote_origin, table_name, error_message
		ORDER BY count(*) DESC
		LIMIT 50;
	`
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return []models.Finding{{
			Severity:   models.SeverityWarning,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Could not query spock.exception_log",
			Detail:     fmt.Sprintf("Error querying exception log: %v", err),
			ObjectName: "spock.exception_log",
		}}, nil
	}
	defer rows.Close()

	type exceptionRow struct {
		origin    string
		tableName string
		errorMsg  string
		count     int
		lastError string
	}

	var exceptions []exceptionRow
	for rows.Next() {
		var er exceptionRow
		if err := rows.Scan(&er.origin, &er.tableName, &er.errorMsg, &er.count, &er.lastError); err != nil {
			return nil, fmt.Errorf("scanning exception_log row: %w", err)
		}
		exceptions = append(exceptions, er)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating exception_log rows: %w", err)
	}

	if len(exceptions) == 0 {
		return []models.Finding{{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "No replication exceptions found",
			Detail:     "The spock.exception_log table contains no records.",
			ObjectName: "spock.exception_log",
		}}, nil
	}

	totalErrors := 0
	for _, er := range exceptions {
		totalErrors += er.count
	}

	var findings []models.Finding

	sev := models.SeverityInfo
	if totalErrors > 0 {
		sev = models.SeverityCritical
	}
	findings = append(findings, models.Finding{
		Severity:  sev,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("%d total replication exception(s) recorded", totalErrors),
		Detail: fmt.Sprintf(
			"The exception log shows %d total apply errors. These represent rows "+
				"that could not be applied on this node. Each exception means data "+
				"divergence between nodes.", totalErrors),
		ObjectName: "spock.exception_log",
		Metadata:   map[string]any{"total_errors": totalErrors},
	})

	for _, er := range exceptions {
		errorSnippet := er.errorMsg
		if len(errorSnippet) > 300 {
			errorSnippet = errorSnippet[:300]
		}
		findings = append(findings, models.Finding{
			Severity:  models.SeverityCritical,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("%d exception(s) on '%s' from origin %s", er.count, er.tableName, er.origin),
			Detail: fmt.Sprintf(
				"Table '%s' has %d apply exception(s) from origin %s. Error: %s. "+
					"Last occurrence: %s.",
				er.tableName, er.count, er.origin, errorSnippet, er.lastError),
			ObjectName: er.tableName,
			Remediation: "Review the exception_log_detail table for full row data. " +
				"Resolve the underlying issue and re-apply or manually fix the affected rows.",
			Metadata: map[string]any{
				"origin":     er.origin,
				"error":      truncate(er.errorMsg, 500),
				"count":      er.count,
				"last_error": er.lastError,
			},
		})
	}

	return findings, nil
}

// truncate returns s truncated to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
