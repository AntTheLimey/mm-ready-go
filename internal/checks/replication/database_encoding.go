package replication

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// DatabaseEncodingCheck verifies the database encoding for Spock compatibility.
type DatabaseEncodingCheck struct{}

func init() {
	check.Register(&DatabaseEncodingCheck{})
}

func (c *DatabaseEncodingCheck) Name() string        { return "database_encoding" }
func (c *DatabaseEncodingCheck) Category() string     { return "replication" }
func (c *DatabaseEncodingCheck) Description() string {
	return "Database encoding — all Spock nodes must use the same encoding"
}
func (c *DatabaseEncodingCheck) Mode() string { return "scan" }

func (c *DatabaseEncodingCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	query := `
		SELECT
			d.datname,
			pg_encoding_to_char(d.encoding) AS encoding,
			d.datcollate AS collation,
			d.datctype AS ctype
		FROM pg_database d
		WHERE d.datname = current_database();
	`
	var dbName, encoding, collation, ctype string
	err := conn.QueryRow(ctx, query).Scan(&dbName, &encoding, &collation, &ctype)
	if err != nil {
		return nil, fmt.Errorf("querying database encoding: %w", err)
	}

	meta := map[string]any{
		"encoding":  encoding,
		"collation": collation,
		"ctype":     ctype,
	}

	if encoding != "UTF8" {
		return []models.Finding{{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("Database encoding is '%s' (not UTF-8)", encoding),
			Detail: fmt.Sprintf(
				"Database '%s' uses encoding '%s'. Spock requires all nodes to use the "+
					"same encoding (verified in source code — it is NOT restricted to UTF-8 "+
					"as some documentation states). However, UTF-8 is the most common and "+
					"portable choice for multi-master setups. All Spock nodes must be "+
					"provisioned with the same encoding.", dbName, encoding),
			ObjectName: dbName,
			Remediation: "Ensure all Spock nodes use the same encoding. If provisioning new " +
				"nodes, consider using UTF-8 for maximum compatibility.",
			Metadata: meta,
		}}, nil
	}

	return []models.Finding{{
		Severity:  models.SeverityInfo,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("Database encoding: %s", encoding),
		Detail: fmt.Sprintf(
			"Database '%s' uses encoding '%s' with collation '%s' and ctype '%s'. "+
				"All Spock nodes must be provisioned with the same encoding.",
			dbName, encoding, collation, ctype),
		ObjectName: dbName,
		Metadata:   meta,
	}}, nil
}
