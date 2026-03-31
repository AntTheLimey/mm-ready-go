package reporter

import (
	"encoding/json"

	"github.com/pgEdge/mm-ready-go/internal/models"
)

type jsonReport struct {
	// Meta holds report metadata.
	Meta jsonMeta `json:"meta"`
	// Summary holds aggregated check counts.
	Summary jsonSummary `json:"summary"`
	// Results holds all check results.
	Results []jsonResult `json:"results"`
}

type jsonMeta struct {
	// Tool is the tool name.
	Tool string `json:"tool"`
	// Version is the tool version.
	Version string `json:"version"`
	// Timestamp is when the scan was performed.
	Timestamp string `json:"timestamp"`
	// Database is the database name.
	Database string `json:"database"`
	// Host is the database server hostname.
	Host string `json:"host"`
	// Port is the database server port.
	Port int `json:"port"`
	// PGVersion is the PostgreSQL server version.
	PGVersion string `json:"pg_version"`
	// SpockTarget is the target Spock version.
	SpockTarget string `json:"spock_target"`
}

type jsonSummary struct {
	// TotalChecks is the number of checks that ran.
	TotalChecks int `json:"total_checks"`
	// ChecksPassed is the number of checks with no findings.
	ChecksPassed int `json:"checks_passed"`
	// Critical is the count of critical findings.
	Critical int `json:"critical"`
	// Warnings is the count of warning findings.
	Warnings int `json:"warnings"`
	// Consider is the count of consider findings.
	Consider int `json:"consider"`
	// Info is the count of info findings.
	Info int `json:"info"`
}

type jsonResult struct {
	// CheckName identifies which check produced this finding.
	CheckName string `json:"check_name"`
	// Category is the check category.
	Category string `json:"category"`
	// Description is a human-readable summary.
	Description string `json:"description"`
	// Passed indicates whether the check passed.
	Passed bool `json:"passed"`
	// Skipped indicates whether the check was skipped.
	Skipped bool `json:"skipped"`
	// Error holds the error message if the check failed.
	Error *string `json:"error"`
	// SkipReason explains why the check was skipped.
	SkipReason string `json:"skip_reason,omitempty"`
	// Findings holds all findings from this check.
	Findings []jsonFinding `json:"findings"`
}

type jsonFinding struct {
	// Severity is the impact level of this finding.
	Severity string `json:"severity"`
	// Title is a short summary of the finding.
	Title string `json:"title"`
	// Detail is the full description of the finding.
	Detail string `json:"detail"`
	// ObjectName is the database object this finding relates to.
	ObjectName string `json:"object_name"`
	// Remediation describes how to fix this finding.
	Remediation string `json:"remediation"`
	// Metadata holds additional key-value data for this finding.
	Metadata map[string]any `json:"metadata"`
}

// RenderJSON renders the report as a JSON string.
func RenderJSON(report *models.ScanReport) string {
	data := jsonReport{
		Meta: jsonMeta{
			Tool:        "mm-ready-go",
			Version:     "0.1.0",
			Timestamp:   report.Timestamp.Format("2006-01-02T15:04:05-07:00"),
			Database:    report.Database,
			Host:        report.Host,
			Port:        report.Port,
			PGVersion:   report.PGVersion,
			SpockTarget: report.SpockTarget,
		},
		Summary: jsonSummary{
			TotalChecks:  report.ChecksTotal(),
			ChecksPassed: report.ChecksPassed(),
			Critical:     report.CriticalCount(),
			Warnings:     report.WarningCount(),
			Consider:     report.ConsiderCount(),
			Info:         report.InfoCount(),
		},
		Results: make([]jsonResult, 0, len(report.Results)),
	}

	for _, r := range report.Results {
		entry := jsonResult{
			CheckName:   r.CheckName,
			Category:    r.Category,
			Description: r.Description,
			Passed:      len(r.Findings) == 0 && r.Error == "",
			Skipped:     r.Skipped,
			Findings:    make([]jsonFinding, 0, len(r.Findings)),
		}

		if r.Error != "" {
			errStr := r.Error
			entry.Error = &errStr
		}
		if r.Skipped {
			entry.SkipReason = r.SkipReason
		}

		for _, f := range r.Findings {
			meta := f.Metadata
			if meta == nil {
				meta = make(map[string]any)
			}
			entry.Findings = append(entry.Findings, jsonFinding{
				Severity:    f.Severity.String(),
				Title:       f.Title,
				Detail:      f.Detail,
				ObjectName:  f.ObjectName,
				Remediation: f.Remediation,
				Metadata:    meta,
			})
		}

		data.Results = append(data.Results, entry)
	}

	out, _ := json.MarshalIndent(data, "", "  ")
	return string(out)
}
