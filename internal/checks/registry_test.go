package checks_test

import (
	"testing"

	"github.com/AntTheLimey/mm-ready/internal/check"
	_ "github.com/AntTheLimey/mm-ready/internal/checks" // triggers all init() registrations
)

func TestTotalCheckCount(t *testing.T) {
	all := check.AllRegistered()
	if len(all) != 56 {
		// List what we have for debugging
		cats := make(map[string]int)
		for _, c := range all {
			cats[c.Category()]++
		}
		t.Errorf("expected 56 checks, got %d. By category: %v", len(all), cats)
	}
}

func TestAllChecksHaveRequiredFields(t *testing.T) {
	for _, c := range check.AllRegistered() {
		if c.Name() == "" {
			t.Errorf("check with empty name (category=%s)", c.Category())
		}
		if c.Category() == "" {
			t.Errorf("check %s has empty category", c.Name())
		}
		if c.Description() == "" {
			t.Errorf("check %s has empty description", c.Name())
		}
		mode := c.Mode()
		if mode != "scan" && mode != "audit" && mode != "both" {
			t.Errorf("check %s has invalid mode %q", c.Name(), mode)
		}
	}
}

func TestUniqueCheckNames(t *testing.T) {
	seen := make(map[string]bool)
	for _, c := range check.AllRegistered() {
		if seen[c.Name()] {
			t.Errorf("duplicate check name: %s", c.Name())
		}
		seen[c.Name()] = true
	}
}

func TestCategoryCounts(t *testing.T) {
	cats := make(map[string]int)
	for _, c := range check.AllRegistered() {
		cats[c.Category()]++
	}
	expected := map[string]int{
		"config":       8,
		"replication":  12,
		"schema":       22,
		"extensions":   4,
		"functions":    3,
		"sequences":    2,
		"sql_patterns": 5,
	}
	for cat, want := range expected {
		got := cats[cat]
		if got != want {
			t.Errorf("category %s: got %d checks, want %d", cat, got, want)
		}
	}
	if len(cats) != len(expected) {
		t.Errorf("expected %d categories, got %d: %v", len(expected), len(cats), cats)
	}
}

func TestGetChecksScanMode(t *testing.T) {
	checks := check.GetChecks("scan", nil)
	for _, c := range checks {
		if c.Mode() != "scan" && c.Mode() != "both" {
			t.Errorf("scan mode returned check %s with mode %q", c.Name(), c.Mode())
		}
	}
	if len(checks) == 0 {
		t.Error("no scan-mode checks found")
	}
}

func TestGetChecksAuditMode(t *testing.T) {
	checks := check.GetChecks("audit", nil)
	for _, c := range checks {
		if c.Mode() != "audit" && c.Mode() != "both" {
			t.Errorf("audit mode returned check %s with mode %q", c.Name(), c.Mode())
		}
	}
	if len(checks) == 0 {
		t.Error("no audit-mode checks found")
	}
}

func TestGetChecksCategoryFilter(t *testing.T) {
	checks := check.GetChecks("", []string{"schema"})
	for _, c := range checks {
		if c.Category() != "schema" {
			t.Errorf("category filter returned check %s with category %q", c.Name(), c.Category())
		}
	}
	if len(checks) != 22 {
		t.Errorf("expected 22 schema checks, got %d", len(checks))
	}
}

func TestGetChecksSorted(t *testing.T) {
	checks := check.GetChecks("", nil)
	for i := 1; i < len(checks); i++ {
		prev := checks[i-1]
		curr := checks[i]
		if prev.Category() > curr.Category() {
			t.Errorf("checks not sorted by category: %s/%s came before %s/%s",
				prev.Category(), prev.Name(), curr.Category(), curr.Name())
		}
		if prev.Category() == curr.Category() && prev.Name() > curr.Name() {
			t.Errorf("checks not sorted by name within category %s: %s came before %s",
				prev.Category(), prev.Name(), curr.Name())
		}
	}
}

func TestGetChecksEmptyModeReturnsAll(t *testing.T) {
	all := check.GetChecks("", nil)
	if len(all) != 56 {
		t.Errorf("empty mode should return all 56 checks, got %d", len(all))
	}
}

func TestGetChecksMultipleCategories(t *testing.T) {
	checks := check.GetChecks("", []string{"config", "replication"})
	for _, c := range checks {
		if c.Category() != "config" && c.Category() != "replication" {
			t.Errorf("multi-category filter returned check %s with category %q", c.Name(), c.Category())
		}
	}
	if len(checks) != 20 {
		t.Errorf("expected 20 config+replication checks, got %d", len(checks))
	}
}

func TestGetChecksNonexistentCategory(t *testing.T) {
	checks := check.GetChecks("", []string{"nonexistent"})
	if len(checks) != 0 {
		t.Errorf("expected 0 checks for nonexistent category, got %d", len(checks))
	}
}

func TestScanAndAuditCountsAddUp(t *testing.T) {
	scan := check.GetChecks("scan", nil)
	audit := check.GetChecks("audit", nil)

	// Count "both" mode checks
	bothCount := 0
	for _, c := range check.AllRegistered() {
		if c.Mode() == "both" {
			bothCount++
		}
	}

	total := len(scan) + len(audit) - bothCount
	if total != 56 {
		t.Errorf("scan(%d) + audit(%d) - both(%d) = %d, want 56",
			len(scan), len(audit), bothCount, total)
	}
}
