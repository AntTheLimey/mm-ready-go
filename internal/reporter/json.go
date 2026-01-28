package reporter

import (
	"encoding/json"

	"github.com/AntTheLimey/mm-ready/internal/models"
)

type jsonReport struct {
	Meta    jsonMeta     `json:"meta"`
	Summary jsonSummary  `json:"summary"`
	Results []jsonResult `json:"results"`
}

type jsonMeta struct {
	Tool        string `json:"tool"`
	Version     string `json:"version"`
	Timestamp   string `json:"timestamp"`
	Database    string `json:"database"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	PGVersion   string `json:"pg_version"`
	SpockTarget string `json:"spock_target"`
}

type jsonSummary struct {
	TotalChecks  int `json:"total_checks"`
	ChecksPassed int `json:"checks_passed"`
	Critical     int `json:"critical"`
	Warnings     int `json:"warnings"`
	Consider     int `json:"consider"`
	Info         int `json:"info"`
}

type jsonResult struct {
	CheckName   string        `json:"check_name"`
	Category    string        `json:"category"`
	Description string        `json:"description"`
	Passed      bool          `json:"passed"`
	Skipped     bool          `json:"skipped"`
	Error       *string       `json:"error"`
	SkipReason  string        `json:"skip_reason,omitempty"`
	Findings    []jsonFinding `json:"findings"`
}

type jsonFinding struct {
	Severity    string         `json:"severity"`
	Title       string         `json:"title"`
	Detail      string         `json:"detail"`
	ObjectName  string         `json:"object_name"`
	Remediation string         `json:"remediation"`
	Metadata    map[string]any `json:"metadata"`
}

// RenderJSON renders the report as a JSON string.
func RenderJSON(report *models.ScanReport) string {
	data := jsonReport{
		Meta: jsonMeta{
			Tool:        "mm-ready",
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
