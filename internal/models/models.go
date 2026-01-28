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
	SeverityCritical Severity = iota
	SeverityWarning
	SeverityConsider
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

func (s Severity) String() string {
	if name, ok := severityNames[s]; ok {
		return name
	}
	return fmt.Sprintf("Severity(%d)", int(s))
}

func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

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
	Severity    Severity       `json:"severity"`
	CheckName   string         `json:"check_name"`
	Category    string         `json:"category"`
	Title       string         `json:"title"`
	Detail      string         `json:"detail"`
	ObjectName  string         `json:"object_name,omitempty"`
	Remediation string         `json:"remediation,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// CheckResult holds the outcome of running a single check.
type CheckResult struct {
	CheckName   string    `json:"check_name"`
	Category    string    `json:"category"`
	Description string    `json:"description"`
	Findings    []Finding `json:"findings"`
	Error       string    `json:"error,omitempty"`
	Skipped     bool      `json:"skipped,omitempty"`
	SkipReason  string    `json:"skip_reason,omitempty"`
}

// ScanReport is the top-level result of scanning a database.
type ScanReport struct {
	Database    string        `json:"database"`
	Host        string        `json:"host"`
	Port        int           `json:"port"`
	Timestamp   time.Time     `json:"timestamp"`
	Results     []CheckResult `json:"results"`
	PGVersion   string        `json:"pg_version"`
	SpockTarget string        `json:"spock_target"`
	ScanMode    string        `json:"scan_mode"`
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

// ChecksTotal returns the total number of check results.
func (r *ScanReport) ChecksTotal() int {
	return len(r.Results)
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
