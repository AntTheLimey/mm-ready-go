// Package models defines data types for findings, check results, and scan reports.
package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// Severity represents the impact level of a finding.
// The ordering is CRITICAL < WARNING < CONSIDER < INFO (by numeric value).
type Severity int

const (
	// SeverityCritical indicates an issue that must be fixed before Spock installation.
	SeverityCritical Severity = iota
	// SeverityWarning indicates an issue that should be fixed or reviewed.
	SeverityWarning
	// SeverityConsider indicates an issue that may need action depending on context.
	SeverityConsider
	// SeverityInfo indicates a purely informational finding requiring no action.
	SeverityInfo
)

var severityNames = map[Severity]string{
	SeverityCritical: "CRITICAL",
	SeverityWarning:  "WARNING",
	SeverityConsider: "CONSIDER",
	SeverityInfo:     "INFO",
}

var severityFromName = map[string]Severity{
	"CRITICAL": SeverityCritical,
	"WARNING":  SeverityWarning,
	"CONSIDER": SeverityConsider,
	"INFO":     SeverityInfo,
}

// String returns the human-readable name of the severity level.
func (s Severity) String() string {
	if name, ok := severityNames[s]; ok {
		return name
	}
	return fmt.Sprintf("Severity(%d)", int(s))
}

// MarshalJSON encodes the severity as its string name for JSON output.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON decodes a severity string name from JSON input.
func (s *Severity) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	sev, ok := severityFromName[name]
	if !ok {
		return fmt.Errorf("unknown severity: %s", name)
	}
	*s = sev
	return nil
}

// ParseSeverity converts a string to a Severity value.
func ParseSeverity(s string) (Severity, error) {
	sev, ok := severityFromName[s]
	if !ok {
		return 0, fmt.Errorf("unknown severity: %s", s)
	}
	return sev, nil
}

// Finding represents a single issue discovered by a check.
type Finding struct {
	// Severity is the impact level of this finding.
	Severity Severity `json:"severity"`
	// CheckName identifies which check produced this finding.
	CheckName string `json:"check_name"`
	// Category is the check category.
	Category string `json:"category"`
	// Title is a short summary of the finding.
	Title string `json:"title"`
	// Detail is the full description of the finding.
	Detail string `json:"detail"`
	// ObjectName is the database object this finding relates to.
	ObjectName string `json:"object_name,omitempty"`
	// Remediation describes how to fix this finding.
	Remediation string `json:"remediation,omitempty"`
	// Metadata holds additional key-value data for this finding.
	Metadata map[string]any `json:"metadata,omitempty"`
}

// CheckResult holds the outcome of running a single check.
type CheckResult struct {
	// CheckName identifies which check produced this finding.
	CheckName string `json:"check_name"`
	// Category is the check category.
	Category string `json:"category"`
	// Description is a human-readable summary.
	Description string `json:"description"`
	// Findings holds all findings from this check.
	Findings []Finding `json:"findings"`
	// Error holds the error message if the check failed.
	Error string `json:"error,omitempty"`
	// Skipped indicates whether the check was skipped.
	Skipped bool `json:"skipped,omitempty"`
	// SkipReason explains why the check was skipped.
	SkipReason string `json:"skip_reason,omitempty"`
}

// ScanReport is the top-level result of scanning a database.
type ScanReport struct {
	// Database is the database name.
	Database string `json:"database"`
	// Host is the database server hostname.
	Host string `json:"host"`
	// Port is the database server port.
	Port int `json:"port"`
	// Timestamp is when the scan was performed.
	Timestamp time.Time `json:"timestamp"`
	// Results holds all check results.
	Results []CheckResult `json:"results"`
	// PGVersion is the PostgreSQL server version.
	PGVersion string `json:"pg_version"`
	// SpockTarget is the target Spock version.
	SpockTarget string `json:"spock_target"`
	// ScanMode is the mode used for this scan.
	ScanMode string `json:"scan_mode"`
}

// NewScanReport creates a ScanReport with sensible defaults.
func NewScanReport(database, host string, port int) *ScanReport {
	return &ScanReport{
		Database:    database,
		Host:        host,
		Port:        port,
		Timestamp:   time.Now().UTC(),
		SpockTarget: "5.0",
		ScanMode:    "scan",
	}
}

// Findings returns all findings from all check results, flattened.
func (r *ScanReport) Findings() []Finding {
	var all []Finding
	for _, cr := range r.Results {
		all = append(all, cr.Findings...)
	}
	return all
}

// CriticalCount returns the number of CRITICAL findings.
func (r *ScanReport) CriticalCount() int {
	return r.countBySeverity(SeverityCritical)
}

// WarningCount returns the number of WARNING findings.
func (r *ScanReport) WarningCount() int {
	return r.countBySeverity(SeverityWarning)
}

// ConsiderCount returns the number of CONSIDER findings.
func (r *ScanReport) ConsiderCount() int {
	return r.countBySeverity(SeverityConsider)
}

// InfoCount returns the number of INFO findings.
func (r *ScanReport) InfoCount() int {
	return r.countBySeverity(SeverityInfo)
}

// ChecksPassed returns the number of checks with no findings, no error, and not skipped.
func (r *ScanReport) ChecksPassed() int {
	count := 0
	for _, cr := range r.Results {
		if len(cr.Findings) == 0 && cr.Error == "" && !cr.Skipped {
			count++
		}
	}
	return count
}

// ChecksTotal returns the number of checks that actually ran (excludes skipped).
func (r *ScanReport) ChecksTotal() int {
	count := 0
	for _, cr := range r.Results {
		if !cr.Skipped {
			count++
		}
	}
	return count
}

// ChecksSkipped returns the number of skipped checks.
func (r *ScanReport) ChecksSkipped() int {
	count := 0
	for _, cr := range r.Results {
		if cr.Skipped {
			count++
		}
	}
	return count
}

func (r *ScanReport) countBySeverity(sev Severity) int {
	count := 0
	for _, f := range r.Findings() {
		if f.Severity == sev {
			count++
		}
	}
	return count
}
