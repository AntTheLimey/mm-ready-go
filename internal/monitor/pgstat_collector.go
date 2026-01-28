// Package monitor implements time-based SQL activity observation.
package monitor

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
)

// StatementSnapshot holds a point-in-time snapshot of a single statement's stats.
type StatementSnapshot struct {
	Query         string
	Calls         int64
	TotalExecTime float64
	Rows          int64
	QueryID       *int64
}

// ChangedQuery represents a query that had increased activity between snapshots.
type ChangedQuery struct {
	Query      string
	DeltaCalls int64
	DeltaTime  float64
	DeltaRows  int64
}

// StatsDelta is the difference between two pg_stat_statements snapshots.
type StatsDelta struct {
	NewQueries     []StatementSnapshot
	ChangedQueries []ChangedQuery
	DurationSecs   float64
}

// IsAvailable checks if pg_stat_statements is queryable.
func IsAvailable(ctx context.Context, conn *pgx.Conn) bool {
	var one int
	err := conn.QueryRow(ctx, "SELECT 1 FROM pg_stat_statements LIMIT 1;").Scan(&one)
	return err == nil
}

// TakeSnapshot takes a snapshot of pg_stat_statements, keyed by queryid or query text.
func TakeSnapshot(ctx context.Context, conn *pgx.Conn) (map[string]StatementSnapshot, error) {
	rows, err := conn.Query(ctx, `
		SELECT queryid, query, calls, total_exec_time, rows
		FROM pg_stat_statements
		ORDER BY calls DESC;
	`)
	if err != nil {
		return nil, fmt.Errorf("pg_stat_statements query failed: %w", err)
	}
	defer rows.Close()

	snapshots := make(map[string]StatementSnapshot)
	for rows.Next() {
		var queryID *int64
		var query string
		var calls, rowCount int64
		var totalTime float64
		if err := rows.Scan(&queryID, &query, &calls, &totalTime, &rowCount); err != nil {
			return nil, fmt.Errorf("pg_stat_statements scan failed: %w", err)
		}
		key := query
		if len(key) > 200 {
			key = key[:200]
		}
		if queryID != nil {
			key = fmt.Sprintf("%d", *queryID)
		}
		snapshots[key] = StatementSnapshot{
			Query:         query,
			Calls:         calls,
			TotalExecTime: totalTime,
			Rows:          rowCount,
			QueryID:       queryID,
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("pg_stat_statements rows error: %w", err)
	}
	return snapshots, nil
}

// CollectOverDuration takes two snapshots separated by duration seconds and computes the delta.
func CollectOverDuration(ctx context.Context, conn *pgx.Conn, duration int, verbose bool) (*StatsDelta, error) {
	if verbose {
		fmt.Fprintln(os.Stderr, "  Taking initial pg_stat_statements snapshot...")
	}

	snapBefore, err := TakeSnapshot(ctx, conn)
	if err != nil {
		return nil, err
	}
	beforeTime := time.Now()

	if verbose {
		fmt.Fprintf(os.Stderr, "  Waiting %d seconds for observation window...\n", duration)
	}

	// Wait, checking periodically
	interval := 60
	if duration < interval {
		interval = duration
	}
	elapsed := 0
	for elapsed < duration {
		sleepTime := interval
		if duration-elapsed < sleepTime {
			sleepTime = duration - elapsed
		}
		time.Sleep(time.Duration(sleepTime) * time.Second)
		elapsed = int(time.Since(beforeTime).Seconds())
		if verbose && elapsed < duration {
			remaining := duration - elapsed
			fmt.Fprintf(os.Stderr, "    %ds remaining...\n", remaining)
		}
	}

	if verbose {
		fmt.Fprintln(os.Stderr, "  Taking final pg_stat_statements snapshot...")
	}

	snapAfter, err := TakeSnapshot(ctx, conn)
	if err != nil {
		return nil, err
	}
	actualDuration := time.Since(beforeTime).Seconds()

	// Compute delta
	delta := &StatsDelta{DurationSecs: actualDuration}

	for key, after := range snapAfter {
		before, existed := snapBefore[key]
		if !existed {
			delta.NewQueries = append(delta.NewQueries, after)
		} else {
			callDiff := after.Calls - before.Calls
			timeDiff := after.TotalExecTime - before.TotalExecTime
			if callDiff > 0 {
				delta.ChangedQueries = append(delta.ChangedQueries, ChangedQuery{
					Query:      after.Query,
					DeltaCalls: callDiff,
					DeltaTime:  timeDiff,
					DeltaRows:  after.Rows - before.Rows,
				})
			}
		}
	}

	// Sort by activity
	sort.Slice(delta.ChangedQueries, func(i, j int) bool {
		return delta.ChangedQueries[i].DeltaCalls > delta.ChangedQueries[j].DeltaCalls
	})

	return delta, nil
}
