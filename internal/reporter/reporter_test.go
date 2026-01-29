package reporter

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/AntTheLimey/mm-ready/internal/models"
)

// -- Test helpers -------------------------------------------------------------

func makeFinding(opts ...func(*models.Finding)) models.Finding {
	f := models.Finding{
		Severity:  models.SeverityInfo,
		CheckName: "test_check",
		Category:  "schema",
		Title:     "Test finding",
		Detail:    "Test detail",
	}
	for _, opt := range opts {
		opt(&f)
	}
	return f
}

func sampleReport() *models.ScanReport {
	r := &models.ScanReport{
		Database:    "testdb",
		Host:        "localhost",
		Port:        5432,
		Timestamp:   time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC),
		PGVersion:   "PostgreSQL 17.0",
		SpockTarget: "5.0",
		ScanMode:    "scan",
	}

	// CRITICAL finding
	r.Results = append(r.Results, models.CheckResult{
		CheckName:   "wal_level",
		Category:    "replication",
		Description: "WAL level check",
		Findings: []models.Finding{{
			Severity:    models.SeverityCritical,
			CheckName:   "wal_level",
			Category:    "replication",
			Title:       "wal_level is not 'logical'",
			Detail:      "Current value: replica",
			Remediation: "ALTER SYSTEM SET wal_level = 'logical';",
		}},
	})

	// WARNING finding
	r.Results = append(r.Results, models.CheckResult{
		CheckName:   "primary_keys",
		Category:    "schema",
		Description: "Primary key check",
		Findings: []models.Finding{{
			Severity:    models.SeverityWarning,
			CheckName:   "primary_keys",
			Category:    "schema",
			Title:       "Table missing primary key",
			Detail:      "public.orders has no PK",
			ObjectName:  "public.orders",
			Remediation: "Add a primary key to public.orders.",
		}},
	})

	// CONSIDER finding
	r.Results = append(r.Results, models.CheckResult{
		CheckName:   "enum_types",
		Category:    "schema",
		Description: "Enum type check",
		Findings: []models.Finding{{
			Severity:    models.SeverityConsider,
			CheckName:   "enum_types",
			Category:    "schema",
			Title:       "ENUM type found",
			Detail:      "public.status_type has 3 values",
			ObjectName:  "public.status_type",
			Remediation: "Use Spock DDL replication for enum modifications.",
		}},
	})

	// INFO finding (no remediation)
	r.Results = append(r.Results, models.CheckResult{
		CheckName:   "pg_version",
		Category:    "config",
		Description: "PG version check",
		Findings: []models.Finding{{
			Severity:  models.SeverityInfo,
			CheckName: "pg_version",
			Category:  "config",
			Title:     "PostgreSQL 17.0",
			Detail:    "Supported by Spock 5",
		}},
	})

	// Passing check (no findings)
	r.Results = append(r.Results, models.CheckResult{
		CheckName:   "exclusion_constraints",
		Category:    "schema",
		Description: "Exclusion constraint check",
	})

	// Errored check
	r.Results = append(r.Results, models.CheckResult{
		CheckName:   "hba_config",
		Category:    "replication",
		Description: "HBA config check",
		Error:       "PermissionError: pg_hba_file_rules not accessible",
	})

	// Skipped check
	r.Results = append(r.Results, models.CheckResult{
		CheckName:   "pgstat_observation",
		Category:    "monitor",
		Description: "pg_stat_statements observation",
		Skipped:     true,
		SkipReason:  "pg_stat_statements not available",
	})

	return r
}

func makeReportWithSeverities(severities ...models.Severity) *models.ScanReport {
	r := &models.ScanReport{
		Database:    "testdb",
		Host:        "localhost",
		Port:        5432,
		Timestamp:   time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC),
		PGVersion:   "PostgreSQL 17.0",
		SpockTarget: "5.0",
		ScanMode:    "scan",
	}
	for i, sev := range severities {
		r.Results = append(r.Results, models.CheckResult{
			CheckName:   fmt.Sprintf("check_%d", i),
			Category:    "schema",
			Description: fmt.Sprintf("Check %d", i),
			Findings: []models.Finding{makeFinding(func(f *models.Finding) {
				f.Severity = sev
				f.CheckName = fmt.Sprintf("check_%d", i)
				f.Remediation = "Fix it."
			})},
		})
	}
	return r
}

// -- JSON Reporter ------------------------------------------------------------

func TestJSONValidJSON(t *testing.T) {
	output := RenderJSON(sampleReport())
	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestJSONHasRequiredKeys(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(RenderJSON(sampleReport())), &data)
	for _, key := range []string{"meta", "summary", "results"} {
		if _, ok := data[key]; !ok {
			t.Errorf("missing key %q", key)
		}
	}
}

func TestJSONSummaryCounts(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(RenderJSON(sampleReport())), &data)
	s := data["summary"].(map[string]any)
	checks := map[string]float64{
		"critical":     1,
		"warnings":     1,
		"consider":     1,
		"info":         1,
		"total_checks": 6, // 7 total minus 1 skipped
	}
	for k, want := range checks {
		got := s[k].(float64)
		if got != want {
			t.Errorf("summary[%q] = %v, want %v", k, got, want)
		}
	}
}

func TestJSONResultsCount(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(RenderJSON(sampleReport())), &data)
	results := data["results"].([]any)
	if len(results) != 7 {
		t.Errorf("results count = %d, want 7", len(results))
	}
}

func TestJSONFindingFields(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(RenderJSON(sampleReport())), &data)
	results := data["results"].([]any)
	wal := results[0].(map[string]any)
	if wal["check_name"] != "wal_level" {
		t.Errorf("check_name = %v, want wal_level", wal["check_name"])
	}
	findings := wal["findings"].([]any)
	f := findings[0].(map[string]any)
	if f["severity"] != "CRITICAL" {
		t.Errorf("severity = %v, want CRITICAL", f["severity"])
	}
	if f["title"] != "wal_level is not 'logical'" {
		t.Errorf("title = %v", f["title"])
	}
}

func TestJSONErrorReported(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(RenderJSON(sampleReport())), &data)
	results := data["results"].([]any)
	for _, r := range results {
		m := r.(map[string]any)
		if m["check_name"] == "hba_config" {
			errVal := m["error"]
			if errVal == nil {
				t.Fatal("error should not be nil for hba_config")
			}
			if !strings.Contains(errVal.(string), "PermissionError") {
				t.Errorf("error = %v, should contain PermissionError", errVal)
			}
			return
		}
	}
	t.Error("hba_config result not found")
}

func TestJSONMetaFields(t *testing.T) {
	var data map[string]any
	json.Unmarshal([]byte(RenderJSON(sampleReport())), &data)
	meta := data["meta"].(map[string]any)
	if meta["database"] != "testdb" {
		t.Errorf("database = %v", meta["database"])
	}
	if meta["pg_version"] != "PostgreSQL 17.0" {
		t.Errorf("pg_version = %v", meta["pg_version"])
	}
}

// -- Markdown Reporter --------------------------------------------------------

func TestMarkdownContainsHeader(t *testing.T) {
	output := RenderMarkdown(sampleReport())
	if !strings.Contains(output, "testdb") {
		t.Error("markdown should contain database name")
	}
	upper := strings.ToUpper(output)
	if !strings.Contains(upper, "MM-READY") {
		t.Error("markdown should contain MM-Ready header")
	}
}

func TestMarkdownContainsSeveritySections(t *testing.T) {
	output := RenderMarkdown(sampleReport())
	if !strings.Contains(output, "CRITICAL") {
		t.Error("markdown should contain CRITICAL")
	}
	if !strings.Contains(output, "WARNING") {
		t.Error("markdown should contain WARNING")
	}
}

func TestMarkdownContainsFindingTitles(t *testing.T) {
	output := RenderMarkdown(sampleReport())
	if !strings.Contains(output, "wal_level is not 'logical'") {
		t.Error("markdown should contain wal_level finding title")
	}
	if !strings.Contains(output, "Table missing primary key") {
		t.Error("markdown should contain primary_keys finding title")
	}
}

// -- HTML Reporter ------------------------------------------------------------

func TestHTMLValidStructure(t *testing.T) {
	output := RenderHTML(sampleReport())
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "<!doctype html>") {
		t.Error("HTML should contain DOCTYPE")
	}
	if !strings.Contains(output, "</html>") {
		t.Error("HTML should contain </html>")
	}
}

func TestHTMLContainsSeverityBadges(t *testing.T) {
	output := RenderHTML(sampleReport())
	for _, badge := range []string{"badge-critical", "badge-warning", "badge-consider", "badge-info"} {
		if !strings.Contains(output, badge) {
			t.Errorf("HTML should contain %s", badge)
		}
	}
}

func TestHTMLContainsFindingContent(t *testing.T) {
	output := RenderHTML(sampleReport())
	// Go's html.EscapeString turns ' into &#39;
	if !strings.Contains(output, "wal_level is not &#39;logical&#39;") && !strings.Contains(output, "wal_level is not 'logical'") {
		t.Error("HTML should contain wal_level finding title")
	}
}

func TestHTMLTodoSectionPresent(t *testing.T) {
	output := RenderHTML(sampleReport())
	if !strings.Contains(output, "To Do") {
		t.Error("HTML should contain To Do section")
	}
	if !strings.Contains(output, "todo-group") {
		t.Error("HTML should contain todo-group elements")
	}
}

func TestHTMLSidebarPresent(t *testing.T) {
	output := RenderHTML(sampleReport())
	if !strings.Contains(output, "sidebar") {
		t.Error("HTML should contain sidebar")
	}
}

// -- Verdict Logic ------------------------------------------------------------

func TestVerdictReadyNoFindings(t *testing.T) {
	r := &models.ScanReport{
		Database:    "db",
		Host:        "h",
		Port:        5432,
		Timestamp:   time.Date(2026, 1, 27, 12, 0, 0, 0, time.UTC),
		PGVersion:   "PostgreSQL 17.0",
		SpockTarget: "5.0",
		ScanMode:    "scan",
	}
	r.Results = append(r.Results, models.CheckResult{
		CheckName:   "x",
		Category:    "c",
		Description: "d",
	})
	md := RenderMarkdown(r)
	if !strings.Contains(md, "READY") {
		t.Error("should contain READY")
	}
	if strings.Contains(md, "NOT READY") {
		t.Error("should not contain NOT READY")
	}
	if strings.Contains(md, "CONDITIONALLY") {
		t.Error("should not contain CONDITIONALLY")
	}
}

func TestVerdictNotReadyWithCritical(t *testing.T) {
	r := makeReportWithSeverities(models.SeverityCritical)
	md := RenderMarkdown(r)
	if !strings.Contains(md, "NOT READY") {
		t.Error("should contain NOT READY")
	}
}

func TestVerdictConditionallyReadyWithWarning(t *testing.T) {
	r := makeReportWithSeverities(models.SeverityWarning)
	md := RenderMarkdown(r)
	if !strings.Contains(md, "CONDITIONALLY READY") {
		t.Error("should contain CONDITIONALLY READY")
	}
}

func TestVerdictReadyWithOnlyConsiderAndInfo(t *testing.T) {
	r := makeReportWithSeverities(models.SeverityConsider, models.SeverityInfo)
	md := RenderMarkdown(r)
	upper := strings.ToUpper(md)
	if strings.Contains(upper, "NOT READY") {
		t.Error("should not contain NOT READY")
	}
	if strings.Contains(upper, "CONDITIONALLY") {
		t.Error("should not contain CONDITIONALLY")
	}
}

func TestVerdictInHTMLToo(t *testing.T) {
	r := makeReportWithSeverities(models.SeverityCritical, models.SeverityWarning)
	h := RenderHTML(r)
	if !strings.Contains(h, "NOT READY") {
		t.Error("HTML should contain NOT READY")
	}
}

func TestVerdictNotInJSON(t *testing.T) {
	r := makeReportWithSeverities(models.SeverityCritical)
	var data map[string]any
	json.Unmarshal([]byte(RenderJSON(r)), &data)
	summary := data["summary"].(map[string]any)
	if _, ok := summary["verdict"]; ok {
		t.Error("JSON summary should not contain verdict field")
	}
}
