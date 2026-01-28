// Check PostgreSQL version compatibility with Spock 5.
package config

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/AntTheLimey/mm-ready/internal/check"
	"github.com/AntTheLimey/mm-ready/internal/models"
	"github.com/jackc/pgx/v5"
)

// PgVersionCheck verifies that the PostgreSQL major version is supported by Spock 5.
type PgVersionCheck struct{}

func init() {
	check.Register(PgVersionCheck{})
}

// Spock 5.x supports PostgreSQL 15, 16, 17, 18
// (PG 18 added in Spock 5.0.3; confirmed via src/compat/ directories)
var supportedMajors = map[int]bool{
	15: true,
	16: true,
	17: true,
	18: true,
}

func (PgVersionCheck) Name() string        { return "pg_version" }
func (PgVersionCheck) Category() string     { return "config" }
func (PgVersionCheck) Description() string  { return "PostgreSQL version compatibility with Spock 5" }
func (PgVersionCheck) Mode() string         { return "scan" }

func (c PgVersionCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	var versionStr string
	var versionNum int
	err := conn.QueryRow(ctx, "SELECT version(), current_setting('server_version_num')::int;").Scan(&versionStr, &versionNum)
	if err != nil {
		return nil, fmt.Errorf("pg_version query failed: %w", err)
	}

	major := versionNum / 10000

	sortedMajors := sortedSupportedMajors()
	majorsList := make([]string, len(sortedMajors))
	for i, m := range sortedMajors {
		majorsList[i] = fmt.Sprintf("%d", m)
	}
	majorsStr := strings.Join(majorsList, ", ")

	var findings []models.Finding

	if !supportedMajors[major] {
		findings = append(findings, models.Finding{
			Severity:  models.SeverityCritical,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     fmt.Sprintf("PostgreSQL %d is not supported by Spock 5", major),
			Detail: fmt.Sprintf(
				"Server is running PostgreSQL %d (%s). "+
					"Spock 5 supports PostgreSQL versions: %s. "+
					"A PostgreSQL upgrade is required before Spock can be installed.",
				major, versionStr, majorsStr,
			),
			ObjectName: "pg_version",
			Remediation: fmt.Sprintf(
				"Upgrade PostgreSQL to version %d (recommended) or any of: %s.",
				sortedMajors[len(sortedMajors)-1], majorsStr,
			),
			Metadata: map[string]any{"major": major, "version_num": versionNum},
		})
	} else {
		findings = append(findings, models.Finding{
			Severity:   models.SeverityInfo,
			CheckName:  c.Name(),
			Category:   c.Category(),
			Title:      fmt.Sprintf("PostgreSQL %d is supported by Spock 5", major),
			Detail:     fmt.Sprintf("Server is running %s, which is compatible with Spock 5.", versionStr),
			ObjectName: "pg_version",
			Metadata:   map[string]any{"major": major, "version_num": versionNum},
		})
	}

	return findings, nil
}

func sortedSupportedMajors() []int {
	majors := make([]int, 0, len(supportedMajors))
	for m := range supportedMajors {
		majors = append(majors, m)
	}
	sort.Ints(majors)
	return majors
}
