//go:build integration

// Package tests contains integration tests that run against a live PostgreSQL database.
//
// These tests require the mmready-test Docker container:
//
//	docker run -d --name mmready-test \
//	  -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=mmready \
//	  -p 5499:5432 \
//	  ghcr.io/pgedge/pgedge-postgres:18.1-spock5.0.4-standard-1
//
// Run with: go test -tags integration ./tests/
package tests

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/AntTheLimey/mm-ready/internal/connection"
	"github.com/AntTheLimey/mm-ready/internal/reporter"
	"github.com/AntTheLimey/mm-ready/internal/scanner"

	_ "github.com/AntTheLimey/mm-ready/internal/checks" // trigger all check registrations
)


func TestFullScanCheckCount(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.Connect(ctx, connection.Config{
		Host: "localhost", Port: 5499, DBName: "mmready",
		User: "postgres", Password: "postgres",
	})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	defer conn.Close(ctx)

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host: "localhost", Port: 5499, DBName: "mmready", Mode: "scan",
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if report.ChecksTotal() < 40 {
		t.Errorf("expected at least 40 checks, got %d", report.ChecksTotal())
	}
}

func TestFullScanNoErrors(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.Connect(ctx, connection.Config{
		Host: "localhost", Port: 5499, DBName: "mmready",
		User: "postgres", Password: "postgres",
	})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	defer conn.Close(ctx)

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host: "localhost", Port: 5499, DBName: "mmready", Mode: "scan",
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	var errors []string
	for _, r := range report.Results {
		if r.Error != "" {
			errors = append(errors, r.CheckName+": "+r.Error)
		}
	}
	if len(errors) > 0 {
		t.Errorf("checks with errors: %v", errors)
	}
}

func TestFullScanHasFindings(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.Connect(ctx, connection.Config{
		Host: "localhost", Port: 5499, DBName: "mmready",
		User: "postgres", Password: "postgres",
	})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	defer conn.Close(ctx)

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host: "localhost", Port: 5499, DBName: "mmready", Mode: "scan",
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if len(report.Findings()) == 0 {
		t.Error("expected findings, got none")
	}
}

func TestFullScanPGVersion(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.Connect(ctx, connection.Config{
		Host: "localhost", Port: 5499, DBName: "mmready",
		User: "postgres", Password: "postgres",
	})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	defer conn.Close(ctx)

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host: "localhost", Port: 5499, DBName: "mmready", Mode: "scan",
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if report.PGVersion == "" {
		t.Error("pg_version should not be empty")
	}
}

func TestFullScanMode(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.Connect(ctx, connection.Config{
		Host: "localhost", Port: 5499, DBName: "mmready",
		User: "postgres", Password: "postgres",
	})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	defer conn.Close(ctx)

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host: "localhost", Port: 5499, DBName: "mmready", Mode: "scan",
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	if report.ScanMode != "scan" {
		t.Errorf("scan_mode = %q, want %q", report.ScanMode, "scan")
	}
}

func TestJSONRendersFromScan(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.Connect(ctx, connection.Config{
		Host: "localhost", Port: 5499, DBName: "mmready",
		User: "postgres", Password: "postgres",
	})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	defer conn.Close(ctx)

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host: "localhost", Port: 5499, DBName: "mmready", Mode: "scan",
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	output, err := reporter.Render(report, "json")
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	var data map[string]any
	if err := json.Unmarshal([]byte(output), &data); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
}

func TestMarkdownRendersFromScan(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.Connect(ctx, connection.Config{
		Host: "localhost", Port: 5499, DBName: "mmready",
		User: "postgres", Password: "postgres",
	})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	defer conn.Close(ctx)

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host: "localhost", Port: 5499, DBName: "mmready", Mode: "scan",
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	output, err := reporter.Render(report, "markdown")
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if len(output) < 100 {
		t.Error("markdown output too short")
	}
}

func TestHTMLRendersFromScan(t *testing.T) {
	ctx := context.Background()
	conn, err := connection.Connect(ctx, connection.Config{
		Host: "localhost", Port: 5499, DBName: "mmready",
		User: "postgres", Password: "postgres",
	})
	if err != nil {
		t.Skipf("Test database not available: %v", err)
	}
	defer conn.Close(ctx)

	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host: "localhost", Port: 5499, DBName: "mmready", Mode: "scan",
	})
	if err != nil {
		t.Fatalf("scan failed: %v", err)
	}

	output, err := reporter.Render(report, "html")
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "<!doctype html>") {
		t.Error("HTML should contain DOCTYPE")
	}
	if len(output) < 1000 {
		t.Error("HTML output too short")
	}
}
