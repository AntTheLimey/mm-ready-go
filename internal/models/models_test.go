package models

import (
	"sort"
	"testing"
	"time"
)

// -- Severity ordering --------------------------------------------------------

func TestCriticalLessThanWarning(t *testing.T) {
	if !(SeverityCritical < SeverityWarning) {
		t.Error("CRITICAL should be less than WARNING")
	}
}

func TestWarningLessThanConsider(t *testing.T) {
	if !(SeverityWarning < SeverityConsider) {
		t.Error("WARNING should be less than CONSIDER")
	}
}

func TestConsiderLessThanInfo(t *testing.T) {
	if !(SeverityConsider < SeverityInfo) {
		t.Error("CONSIDER should be less than INFO")
	}
}

func TestCriticalLessThanInfo(t *testing.T) {
	if !(SeverityCritical < SeverityInfo) {
		t.Error("CRITICAL should be less than INFO")
	}
}

func TestSameSeverityNotLess(t *testing.T) {
	if SeverityWarning < SeverityWarning {
		t.Error("WARNING should not be less than WARNING")
	}
}

func TestInfoNotLessThanCritical(t *testing.T) {
	if SeverityInfo < SeverityCritical {
		t.Error("INFO should not be less than CRITICAL")
	}
}

func TestSortedOrder(t *testing.T) {
	severities := []Severity{SeverityInfo, SeverityCritical, SeverityConsider, SeverityWarning}
	sort.Slice(severities, func(i, j int) bool { return severities[i] < severities[j] })
	expected := []Severity{SeverityCritical, SeverityWarning, SeverityConsider, SeverityInfo}
	for i, s := range severities {
		if s != expected[i] {
			t.Errorf("sorted[%d] = %s, want %s", i, s, expected[i])
		}
	}
}

// -- Finding defaults ---------------------------------------------------------

func TestFindingDefaults(t *testing.T) {
	f := Finding{
		Severity:  SeverityWarning,
		CheckName: "test",
		Category:  "schema",
		Title:     "title",
		Detail:    "detail",
	}
	if f.ObjectName != "" {
		t.Error("ObjectName should default to empty string")
	}
	if f.Remediation != "" {
		t.Error("Remediation should default to empty string")
	}
	if f.Metadata != nil {
		t.Error("Metadata should default to nil")
	}
}

func TestMetadataIndependent(t *testing.T) {
	f1 := makeFinding()
	f2 := makeFinding()
	f1.Metadata["key"] = "value"
	if _, exists := f2.Metadata["key"]; exists {
		t.Error("f2.Metadata should be independent from f1.Metadata")
	}
}

// -- ScanReport properties ----------------------------------------------------

func TestEmptyReportCounts(t *testing.T) {
	r := emptyReport()
	if r.CriticalCount() != 0 {
		t.Errorf("CriticalCount = %d, want 0", r.CriticalCount())
	}
	if r.WarningCount() != 0 {
		t.Errorf("WarningCount = %d, want 0", r.WarningCount())
	}
	if r.ConsiderCount() != 0 {
		t.Errorf("ConsiderCount = %d, want 0", r.ConsiderCount())
	}
	if r.InfoCount() != 0 {
		t.Errorf("InfoCount = %d, want 0", r.InfoCount())
	}
	if r.ChecksTotal() != 0 {
		t.Errorf("ChecksTotal = %d, want 0", r.ChecksTotal())
	}
	if r.ChecksPassed() != 0 {
		t.Errorf("ChecksPassed = %d, want 0", r.ChecksPassed())
	}
	if len(r.Findings()) != 0 {
		t.Errorf("Findings = %d, want 0", len(r.Findings()))
	}
}

func TestCriticalCount(t *testing.T) {
	r := sampleReport()
	if r.CriticalCount() != 1 {
		t.Errorf("CriticalCount = %d, want 1", r.CriticalCount())
	}
}

func TestWarningCount(t *testing.T) {
	r := sampleReport()
	if r.WarningCount() != 1 {
		t.Errorf("WarningCount = %d, want 1", r.WarningCount())
	}
}

func TestConsiderCount(t *testing.T) {
	r := sampleReport()
	if r.ConsiderCount() != 1 {
		t.Errorf("ConsiderCount = %d, want 1", r.ConsiderCount())
	}
}

func TestInfoCount(t *testing.T) {
	r := sampleReport()
	if r.InfoCount() != 1 {
		t.Errorf("InfoCount = %d, want 1", r.InfoCount())
	}
}

func TestChecksTotal(t *testing.T) {
	r := sampleReport()
	if r.ChecksTotal() != 7 {
		t.Errorf("ChecksTotal = %d, want 7", r.ChecksTotal())
	}
}

func TestChecksPassed(t *testing.T) {
	r := sampleReport()
	// Only exclusion_constraints has no findings, no error, not skipped
	if r.ChecksPassed() != 1 {
		t.Errorf("ChecksPassed = %d, want 1", r.ChecksPassed())
	}
}

func TestFindingsFlattened(t *testing.T) {
	r := sampleReport()
	if len(r.Findings()) != 4 {
		t.Errorf("len(Findings) = %d, want 4", len(r.Findings()))
	}
}

func TestChecksPassedExcludesErrored(t *testing.T) {
	r := &ScanReport{
		Database:  "db",
		Host:      "h",
		Port:      5432,
		Timestamp: time.Now().UTC(),
	}
	r.Results = append(r.Results, CheckResult{
		CheckName:   "x",
		Category:    "c",
		Description: "d",
		Error:       "something failed",
	})
	if r.ChecksPassed() != 0 {
		t.Errorf("ChecksPassed = %d, want 0", r.ChecksPassed())
	}
}

func TestChecksPassedExcludesSkipped(t *testing.T) {
	r := &ScanReport{
		Database:  "db",
		Host:      "h",
		Port:      5432,
		Timestamp: time.Now().UTC(),
	}
	r.Results = append(r.Results, CheckResult{
		CheckName:   "x",
		Category:    "c",
		Description: "d",
		Skipped:     true,
	})
	if r.ChecksPassed() != 0 {
		t.Errorf("ChecksPassed = %d, want 0", r.ChecksPassed())
	}
}

// -- Test helpers -------------------------------------------------------------

func makeFinding(opts ...func(*Finding)) Finding {
	f := Finding{
		Severity:  SeverityInfo,
		CheckName: "test_check",
		Category:  "schema",
		Title:     "Test finding",
		Detail:    "Test detail",
		Metadata:  make(map[string]any),
	}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

func emptyReport() *ScanReport {
	return &ScanReport{
		Database:    "testdb",
		Host:        "localhost",
		Port:        5432,
		Timestamp:   time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC),
		PGVersion:   "PostgreSQL 17.0",
		SpockTarget: "5.0",
		ScanMode:    "scan",
	}
}

func sampleReport() *ScanReport {
	r := emptyReport()

	// CRITICAL finding
	r.Results = append(r.Results, CheckResult{
		CheckName:   "wal_level",
		Category:    "replication",
		Description: "WAL level check",
		Findings: []Finding{{
			Severity:    SeverityCritical,
			CheckName:   "wal_level",
			Category:    "replication",
			Title:       "wal_level is not 'logical'",
			Detail:      "Current value: replica",
			Remediation: "ALTER SYSTEM SET wal_level = 'logical';",
		}},
	})

	// WARNING finding
	r.Results = append(r.Results, CheckResult{
		CheckName:   "primary_keys",
		Category:    "schema",
		Description: "Primary key check",
		Findings: []Finding{{
			Severity:    SeverityWarning,
			CheckName:   "primary_keys",
			Category:    "schema",
			Title:       "Table missing primary key",
			Detail:      "public.orders has no PK",
			ObjectName:  "public.orders",
			Remediation: "Add a primary key to public.orders.",
		}},
	})

	// CONSIDER finding
	r.Results = append(r.Results, CheckResult{
		CheckName:   "enum_types",
		Category:    "schema",
		Description: "Enum type check",
		Findings: []Finding{{
			Severity:    SeverityConsider,
			CheckName:   "enum_types",
			Category:    "schema",
			Title:       "ENUM type found",
			Detail:      "public.status_type has 3 values",
			ObjectName:  "public.status_type",
			Remediation: "Use Spock DDL replication for enum modifications.",
		}},
	})

	// INFO finding (no remediation)
	r.Results = append(r.Results, CheckResult{
		CheckName:   "pg_version",
		Category:    "config",
		Description: "PG version check",
		Findings: []Finding{{
			Severity:  SeverityInfo,
			CheckName: "pg_version",
			Category:  "config",
			Title:     "PostgreSQL 17.0",
			Detail:    "Supported by Spock 5",
		}},
	})

	// Passing check (no findings)
	r.Results = append(r.Results, CheckResult{
		CheckName:   "exclusion_constraints",
		Category:    "schema",
		Description: "Exclusion constraint check",
	})

	// Errored check
	r.Results = append(r.Results, CheckResult{
		CheckName:   "hba_config",
		Category:    "replication",
		Description: "HBA config check",
		Error:       "PermissionError: pg_hba_file_rules not accessible",
	})

	// Skipped check
	r.Results = append(r.Results, CheckResult{
		CheckName:   "pgstat_observation",
		Category:    "monitor",
		Description: "pg_stat_statements observation",
		Skipped:     true,
		SkipReason:  "pg_stat_statements not available",
	})

	return r
}
