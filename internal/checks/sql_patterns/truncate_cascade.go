// Check for TRUNCATE ... CASCADE and RESTART IDENTITY usage.
package sql_patterns

import (
	"context"
	"fmt"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// TruncateCascadeCheck detects TRUNCATE CASCADE and RESTART IDENTITY in pg_stat_statements.
type TruncateCascadeCheck struct{}

func init() {
	check.Register(TruncateCascadeCheck{})
}

func (TruncateCascadeCheck) Name() string        { return "truncate_cascade" }
func (TruncateCascadeCheck) Category() string     { return "sql_patterns" }
func (TruncateCascadeCheck) Mode() string         { return "scan" }
func (TruncateCascadeCheck) Description() string {
	return "TRUNCATE ... CASCADE and RESTART IDENTITY — replication behaviour caveats"
}

func (c TruncateCascadeCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	// Check for TRUNCATE CASCADE.
	cascadeRows, err := conn.Query(ctx, `
		SELECT query, calls
		FROM pg_stat_statements
		WHERE query ~* 'TRUNCATE.*CASCADE'
		ORDER BY calls DESC;
	`)
	if err != nil {
		// pg_stat_statements not available.
		return []models.Finding{{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      "Cannot check TRUNCATE patterns — pg_stat_statements unavailable",
			Detail:     "pg_stat_statements is not available. Cannot check for TRUNCATE CASCADE or RESTART IDENTITY usage.",
			ObjectName: "pg_stat_statements",
		}}, nil
	}
	defer cascadeRows.Close()

	var findings []models.Finding
	for cascadeRows.Next() {
		var queryText string
		var calls int64
		if err := cascadeRows.Scan(&queryText, &calls); err != nil {
			return nil, fmt.Errorf("truncate_cascade cascade scan failed: %w", err)
		}

		truncated := queryText
		if len(truncated) > 200 {
			truncated = truncated[:200]
		}
		querySnippet := queryText
		if len(querySnippet) > 500 {
			querySnippet = querySnippet[:500]
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("TRUNCATE CASCADE detected (%d call(s))", calls),
			Detail: fmt.Sprintf(
				"Query executed %d time(s): %s...\n\n"+
					"TRUNCATE ... CASCADE will only apply the CASCADE option on the "+
					"provider. The subscriber ALWAYS applies TRUNCATE with DROP_RESTRICT "+
					"behavior — this is hardcoded in spock_apply.c:1707. Only the "+
					"explicitly named table(s) will be truncated on subscribers; "+
					"cascaded truncates of referencing tables will NOT propagate.\n\n"+
					"Note: TRUNCATE is replicated through replication sets (not AutoDDL). "+
					"AutoDDL does not handle TRUNCATE — PostgreSQL classifies it as "+
					"LOGSTMT_MISC, not LOGSTMT_DDL.",
				calls, truncated,
			),
			ObjectName: "(query)",
			Remediation: "Explicitly TRUNCATE all related tables rather than relying on CASCADE. " +
				"List every table that CASCADE would affect:\n" +
				"  TRUNCATE parent_table, child_table1, child_table2;",
			Metadata: map[string]any{"calls": calls, "query": querySnippet},
		})
	}
	if err := cascadeRows.Err(); err != nil {
		return nil, fmt.Errorf("truncate_cascade cascade rows error: %w", err)
	}

	// Check for TRUNCATE RESTART IDENTITY.
	restartRows, err := conn.Query(ctx, `
		SELECT query, calls
		FROM pg_stat_statements
		WHERE query ~* 'TRUNCATE.*RESTART'
		ORDER BY calls DESC;
	`)
	if err != nil {
		return findings, nil
	}
	defer restartRows.Close()

	for restartRows.Next() {
		var queryText string
		var calls int64
		if err := restartRows.Scan(&queryText, &calls); err != nil {
			return nil, fmt.Errorf("truncate_cascade restart scan failed: %w", err)
		}

		truncated := queryText
		if len(truncated) > 200 {
			truncated = truncated[:200]
		}
		querySnippet := queryText
		if len(querySnippet) > 500 {
			querySnippet = querySnippet[:500]
		}

		findings = append(findings, models.Finding{
			Severity:  models.SeverityConsider,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("TRUNCATE RESTART IDENTITY detected (%d call(s))", calls),
			Detail: fmt.Sprintf(
				"Query executed %d time(s): %s...\n\n"+
					"TRUNCATE ... RESTART IDENTITY resets the sequence(s) associated "+
					"with the truncated table. The Spock source code "+
					"(spock_apply_heap.c) passes the restart_seqs flag through to "+
					"ExecuteTruncateGuts(), so this IS replicated — despite Spock "+
					"documentation stating otherwise. However, in multi-master setups "+
					"resetting sequences on all nodes could cause ID collisions unless "+
					"pgEdge Snowflake sequences are in use.",
				calls, truncated,
			),
			ObjectName: "(query)",
			Remediation: "If using standard PostgreSQL sequences, avoid RESTART IDENTITY " +
				"across replicated nodes or switch to pgEdge Snowflake IDs first. " +
				"If already using Snowflake IDs, this pattern is safe.",
			Metadata: map[string]any{"calls": calls, "query": querySnippet},
		})
	}
	if err := restartRows.Err(); err != nil {
		return nil, fmt.Errorf("truncate_cascade restart rows error: %w", err)
	}

	return findings, nil
}
