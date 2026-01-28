package cmd

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

var tsPattern = regexp.MustCompile(`\d{8}_\d{6}`)

// -- MakeDefaultOutputPath ----------------------------------------------------

func TestDefaultOutputPathHTML(t *testing.T) {
	path := MakeDefaultOutputPath("html", "mydb")
	if !strings.HasPrefix(path, "reports/mydb_") && !strings.HasPrefix(path, `reports\mydb_`) {
		t.Errorf("path %q should start with reports/mydb_", path)
	}
	if !strings.HasSuffix(path, ".html") {
		t.Errorf("path %q should end with .html", path)
	}
}

func TestDefaultOutputPathJSON(t *testing.T) {
	path := MakeDefaultOutputPath("json", "mydb")
	if !strings.Contains(path, "mydb_") {
		t.Errorf("path %q should contain mydb_", path)
	}
	if !strings.HasSuffix(path, ".json") {
		t.Errorf("path %q should end with .json", path)
	}
}

func TestDefaultOutputPathMarkdown(t *testing.T) {
	path := MakeDefaultOutputPath("markdown", "mydb")
	if !strings.Contains(path, "mydb_") {
		t.Errorf("path %q should contain mydb_", path)
	}
	if !strings.HasSuffix(path, ".md") {
		t.Errorf("path %q should end with .md", path)
	}
}

func TestDefaultOutputPathEmptyDBName(t *testing.T) {
	path := MakeDefaultOutputPath("html", "")
	if !strings.Contains(path, "mm-ready_") {
		t.Errorf("path %q should contain mm-ready_", path)
	}
}

func TestDefaultOutputPathTimestamp(t *testing.T) {
	path := MakeDefaultOutputPath("html", "db")
	if !tsPattern.MatchString(path) {
		t.Errorf("path %q should contain timestamp pattern YYYYMMDD_HHMMSS", path)
	}
}

// -- MakeOutputPath -----------------------------------------------------------

func TestOutputPathInsertsTimestamp(t *testing.T) {
	path := MakeOutputPath("report.html", "html", "db")
	if !strings.HasPrefix(path, "report_") {
		t.Errorf("path %q should start with report_", path)
	}
	if !strings.HasSuffix(path, ".html") {
		t.Errorf("path %q should end with .html", path)
	}
	if !tsPattern.MatchString(path) {
		t.Errorf("path %q should contain timestamp", path)
	}
}

func TestOutputPathNoExtUsesFormat(t *testing.T) {
	path := MakeOutputPath("report", "json", "db")
	if !strings.HasSuffix(path, ".json") {
		t.Errorf("path %q should end with .json", path)
	}
}

func TestOutputPathDirectory(t *testing.T) {
	dir, err := os.MkdirTemp("", "mmready-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := MakeOutputPath(dir, "html", "mydb")
	if !strings.HasPrefix(path, dir) {
		t.Errorf("path %q should start with %q", path, dir)
	}
	if !strings.Contains(path, "mydb_") {
		t.Errorf("path %q should contain mydb_", path)
	}
	if !strings.HasSuffix(path, ".html") {
		t.Errorf("path %q should end with .html", path)
	}
}

func TestOutputPathPreservesUserExtension(t *testing.T) {
	path := MakeOutputPath("output.txt", "html", "db")
	if !strings.HasSuffix(path, ".txt") {
		t.Errorf("path %q should end with .txt", path)
	}
}
