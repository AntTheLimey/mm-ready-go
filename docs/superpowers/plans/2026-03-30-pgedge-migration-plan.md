# pgEdge Migration & Feature Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform mm-ready from a personal AntTheLimey project into a pgEdge company tool with full feature parity against the Python version.

**Architecture:** Mechanical module path rename first, then layer on missing features (SSL, config files, check filtering, report customization, LOLOR check), bug fixes, CI/CD, and documentation. Each task is a logical commit.

**Tech Stack:** Go 1.25, pgx/v5, cobra, gopkg.in/yaml.v3, golangci-lint, GitHub Actions

---

## File Structure

### Files to Create
- `internal/config/config.go` — YAML config loader, matching Python format
- `internal/config/config_test.go` — Unit tests for config
- `internal/checks/extensions/lolor_check.go` — LOLOR extension check
- `.github/workflows/ci.yml` — CI pipeline
- `.golangci.yml` — Linter config
- `.github/CODEOWNERS` — Code ownership
- `.github/PULL_REQUEST_TEMPLATE.md` — PR template
- `.github/ISSUE_TEMPLATE/bug_report.md` — Bug report template
- `.github/ISSUE_TEMPLATE/feature_request.md` — Feature request template
- `.pre-commit-config.yaml` — Pre-commit hooks
- `.codacy.yaml` — Codacy config
- `.coderabbit.yaml` — CodeRabbit config
- `mkdocs.yml` — MkDocs site config
- `docs/overrides/partials/logo.html` — Logo toggle
- `docs/stylesheets/extra.css` — Theme CSS
- `docs/tutorial.md` — Tutorial page

### Files to Modify
- `go.mod` — Module path rename, add yaml.v3 dependency
- `main.go` — Import path update
- `Makefile` — Binary name to mm-ready-go
- `LICENSE.md` — Copyright to pgEdge, Inc
- `internal/connection/connection.go` — Add SSL fields, PG* env vars, TLS config
- `internal/cmd/root.go` — Binary name, SSL flag helpers, config/filtering flags
- `internal/cmd/scan.go` — Wire SSL, filtering, config, report options
- `internal/cmd/audit.go` — Wire SSL, filtering, config, report options
- `internal/cmd/analyze.go` — Wire filtering, config, report options
- `internal/cmd/monitor.go` — Wire SSL, filtering, config
- `internal/cmd/listchecks.go` — Wire filtering flags
- `internal/check/registry.go` — Add exclude/includeOnly params
- `internal/scanner/scanner.go` — Pass filtering through Options
- `internal/analyzer/analyzer.go` — Pass filtering, add LOLOR to skipped checks
- `internal/analyzer/checks.go` — Fix snowflake.nextval() skip in checkSequencePKs
- `internal/reporter/reporter.go` — Add ReportOptions param
- `internal/reporter/html.go` — Respect --no-todo, --todo-include-consider; update version strings
- `internal/reporter/markdown.go` — Update version string
- `internal/reporter/json.go` — Update tool name and version
- `internal/checks/functions/views_audit.go` — Mode "scan" -> "both"
- `internal/checks/registry_test.go` — Update expected counts
- `internal/checks/register.go` — Import path update
- All ~79 Go files — Import path `AntTheLimey/mm-ready` -> `pgEdge/mm-ready-go`
- `README.md` — Full rewrite
- `CLAUDE.md` — Update paths, binary name, features
- `docs/index.md` — Adapt for Go
- `docs/quickstart.md` — Adapt for Go
- `docs/architecture.md` — Update paths
- `docs/checks-reference.md` — Add LOLOR check

### Files to Copy from Python repo
- `docs/img/logo-dark.png`
- `docs/img/logo-light.png`
- `docs/img/favicon.ico`

---

### Task 1: Module Path Rename

**Files:**
- Modify: `go.mod:1`
- Modify: All ~79 `.go` files (import paths)
- Modify: `Makefile:3`
- Modify: `internal/cmd/root.go:13`

- [ ] **Step 1: Update go.mod module path**

In `go.mod`, change line 1:

```
module github.com/pgEdge/mm-ready-go
```

- [ ] **Step 2: Replace all import paths in Go source files**

Run:
```bash
find . -name '*.go' -exec sed -i '' 's|github.com/AntTheLimey/mm-ready|github.com/pgEdge/mm-ready-go|g' {} +
```

- [ ] **Step 3: Update binary name in Makefile**

Replace the full Makefile content:

```makefile
.PHONY: build test test-integration lint clean build-all

BINARY := mm-ready-go
VERSION := 0.1.0

build:
	go build -o bin/$(BINARY) .

test:
	go test ./...

test-integration:
	go test -tags integration ./tests/ -v

lint:
	go vet ./...

clean:
	rm -rf bin/

build-all:
	GOOS=linux   GOARCH=amd64 go build -o bin/$(BINARY)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -o bin/$(BINARY)-linux-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -o bin/$(BINARY)-darwin-amd64 .
	GOOS=darwin  GOARCH=arm64 go build -o bin/$(BINARY)-darwin-arm64 .
	GOOS=windows GOARCH=amd64 go build -o bin/$(BINARY)-windows-amd64.exe .
```

- [ ] **Step 4: Update cobra command Use field**

In `internal/cmd/root.go:13`, change:
```go
Use:   "mm-ready-go",
```

- [ ] **Step 5: Verify build succeeds**

Run: `go build -o bin/mm-ready-go .`
Expected: Clean build, no errors

- [ ] **Step 6: Run unit tests**

Run: `go test ./internal/...`
Expected: All tests pass

- [ ] **Step 7: Commit**

```bash
git add -A
git commit -m "Rename module path to github.com/pgEdge/mm-ready-go"
```

---

### Task 2: License & Copyright

**Files:**
- Modify: `LICENSE.md`

- [ ] **Step 1: Update LICENSE.md copyright**

Replace `Copyright (c) 2025, Antony Pegg` with `Copyright (c) 2025, pgEdge, Inc`

- [ ] **Step 2: Commit**

```bash
git add LICENSE.md
git commit -m "Update license copyright to pgEdge, Inc"
```

---

### Task 3: SSL/TLS Connection Support

**Files:**
- Modify: `internal/connection/connection.go`
- Modify: `internal/cmd/root.go:51-73`
- Modify: `internal/cmd/scan.go:41-48`
- Modify: `internal/cmd/audit.go`
- Modify: `internal/cmd/monitor.go:35-42`

- [ ] **Step 1: Add SSL fields to connection Config and build TLS config**

In `internal/connection/connection.go`, replace the full file with:

```go
// Package connection provides PostgreSQL database connection management.
package connection

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"

	"github.com/jackc/pgx/v5"
)

// Config holds the parameters needed to connect to a PostgreSQL database.
type Config struct {
	Host       string
	Port       int
	DBName     string
	User       string
	Password   string
	DSN        string
	SSLMode    string
	SSLCert    string
	SSLKey     string
	SSLRootCert string
}

// Connect creates a database connection from a Config.
// If DSN is provided, it is used directly. Otherwise individual parameters are
// used, falling back to standard PG* environment variables (handled by pgx).
func Connect(ctx context.Context, cfg Config) (*pgx.Conn, error) {
	var connStr string

	if cfg.DSN != "" {
		connStr = cfg.DSN
	} else {
		connStr = buildConnString(cfg)
	}

	// Parse the connection string and add read-only runtime param
	connConfig, err := pgx.ParseConfig(connStr)
	if err != nil {
		return nil, fmt.Errorf("parse connection config: %w", err)
	}
	connConfig.RuntimeParams["default_transaction_read_only"] = "on"

	// Apply TLS configuration if sslmode is set
	if cfg.SSLMode != "" && cfg.SSLMode != "disable" {
		tlsConfig, tlsErr := buildTLSConfig(cfg)
		if tlsErr != nil {
			return nil, fmt.Errorf("configure TLS: %w", tlsErr)
		}
		connConfig.TLSConfig = tlsConfig
	}

	conn, err := pgx.ConnectConfig(ctx, connConfig)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	return conn, nil
}

// GetPGVersion returns the PostgreSQL server version string.
func GetPGVersion(ctx context.Context, conn *pgx.Conn) (string, error) {
	var version string
	err := conn.QueryRow(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("get pg version: %w", err)
	}
	return version, nil
}

func buildConnString(cfg Config) string {
	parts := ""
	if cfg.Host != "" {
		parts += fmt.Sprintf("host=%s ", cfg.Host)
	}
	if cfg.Port != 0 {
		parts += fmt.Sprintf("port=%d ", cfg.Port)
	}
	if cfg.DBName != "" {
		parts += fmt.Sprintf("dbname=%s ", cfg.DBName)
	}
	if cfg.User != "" {
		parts += fmt.Sprintf("user=%s ", cfg.User)
	}
	if cfg.Password != "" {
		parts += fmt.Sprintf("password=%s ", cfg.Password)
	}
	if cfg.SSLMode != "" {
		parts += fmt.Sprintf("sslmode=%s ", cfg.SSLMode)
	}
	return parts
}

func buildTLSConfig(cfg Config) (*tls.Config, error) {
	tlsCfg := &tls.Config{}

	switch cfg.SSLMode {
	case "require":
		tlsCfg.InsecureSkipVerify = true
	case "verify-ca", "verify-full":
		tlsCfg.InsecureSkipVerify = false
		if cfg.SSLMode == "verify-full" {
			tlsCfg.ServerName = cfg.Host
		}
	default:
		tlsCfg.InsecureSkipVerify = true
	}

	if cfg.SSLRootCert != "" {
		caCert, err := os.ReadFile(cfg.SSLRootCert)
		if err != nil {
			return nil, fmt.Errorf("read SSL root cert: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse SSL root cert")
		}
		tlsCfg.RootCAs = pool
	}

	if cfg.SSLCert != "" && cfg.SSLKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.SSLCert, cfg.SSLKey)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{cert}
	}

	return tlsCfg, nil
}
```

- [ ] **Step 2: Add SSL fields to connFlags and helper functions in root.go**

In `internal/cmd/root.go`, replace the `connFlags` struct and `addConnFlags` function (lines 51-73):

```go
// Connection flags shared by scan, audit, and monitor commands.
type connFlags struct {
	DSN         string
	Host        string
	Port        int
	DBName      string
	User        string
	Password    string
	SSLMode     string
	SSLCert     string
	SSLKey      string
	SSLRootCert string
}

// Output flags shared by scan, audit, and monitor commands.
type outputFlags struct {
	Format string
	Output string
}

func addConnFlags(cmd *cobra.Command, f *connFlags) {
	cmd.Flags().StringVar(&f.DSN, "dsn", "", "PostgreSQL connection URI (postgres://...)")
	cmd.Flags().StringVarP(&f.Host, "host", "H", envOrDefault("PGHOST", ""), "Database host")
	cmd.Flags().IntVarP(&f.Port, "port", "p", envIntOrDefault("PGPORT", 5432), "Database port")
	cmd.Flags().StringVarP(&f.DBName, "dbname", "d", envOrDefault("PGDATABASE", ""), "Database name")
	cmd.Flags().StringVarP(&f.User, "user", "U", envOrDefault("PGUSER", ""), "Database user")
	cmd.Flags().StringVarP(&f.Password, "password", "W", envOrDefault("PGPASSWORD", ""), "Database password")
	cmd.Flags().StringVar(&f.SSLMode, "sslmode", envOrDefault("PGSSLMODE", ""), "SSL mode (disable, require, verify-ca, verify-full)")
	cmd.Flags().StringVar(&f.SSLCert, "sslcert", envOrDefault("PGSSLCERT", ""), "Path to SSL client certificate")
	cmd.Flags().StringVar(&f.SSLKey, "sslkey", envOrDefault("PGSSLKEY", ""), "Path to SSL client key")
	cmd.Flags().StringVar(&f.SSLRootCert, "sslrootcert", envOrDefault("PGSSLROOTCERT", ""), "Path to SSL root certificate")
}

func addOutputFlags(cmd *cobra.Command, f *outputFlags) {
	cmd.Flags().StringVarP(&f.Format, "format", "f", "html", "Report format (json, markdown, html)")
	cmd.Flags().StringVarP(&f.Output, "output", "o", "", "Output file path (default: ./reports/<dbname>_<timestamp>.<ext>)")
}

func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
```

Also add `"os"` and `"strconv"` to the imports in root.go.

- [ ] **Step 3: Wire SSL fields in connection.Config construction**

In `internal/cmd/scan.go`, update the `runMode` function's `connection.Connect` call (lines 41-48) to pass SSL fields:

```go
	conn, err := connection.Connect(ctx, connection.Config{
		Host:        cf.Host,
		Port:        cf.Port,
		DBName:      cf.DBName,
		User:        cf.User,
		Password:    cf.Password,
		DSN:         cf.DSN,
		SSLMode:     cf.SSLMode,
		SSLCert:     cf.SSLCert,
		SSLKey:      cf.SSLKey,
		SSLRootCert: cf.SSLRootCert,
	})
```

Do the same in `internal/cmd/monitor.go` (lines 35-42):

```go
	conn, err := connection.Connect(ctx, connection.Config{
		Host:        monitorConn.Host,
		Port:        monitorConn.Port,
		DBName:      monitorConn.DBName,
		User:        monitorConn.User,
		Password:    monitorConn.Password,
		DSN:         monitorConn.DSN,
		SSLMode:     monitorConn.SSLMode,
		SSLCert:     monitorConn.SSLCert,
		SSLKey:      monitorConn.SSLKey,
		SSLRootCert: monitorConn.SSLRootCert,
	})
```

- [ ] **Step 4: Verify build**

Run: `go build -o bin/mm-ready-go .`
Expected: Clean build

- [ ] **Step 5: Verify tests pass**

Run: `go test ./internal/...`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add internal/connection/connection.go internal/cmd/root.go internal/cmd/scan.go internal/cmd/monitor.go
git commit -m "Add SSL/TLS connection support and PG* environment variable handling"
```

---

### Task 4: Check Filtering

**Files:**
- Modify: `internal/check/registry.go`
- Modify: `internal/scanner/scanner.go:17-24`
- Modify: `internal/cmd/scan.go`
- Modify: `internal/cmd/audit.go`
- Modify: `internal/cmd/analyze.go`
- Modify: `internal/cmd/monitor.go`
- Modify: `internal/cmd/listchecks.go`
- Modify: `internal/analyzer/analyzer.go:99-192`
- Modify: `internal/checks/registry_test.go`

- [ ] **Step 1: Add exclude/includeOnly to GetChecks**

Replace `internal/check/registry.go` entirely:

```go
package check

import "sort"

// GetChecks returns all registered checks filtered by mode, categories,
// exclude list, and include-only list. Sorted by (category, name).
// If includeOnly is non-empty, only checks whose Name() is in includeOnly are returned.
// If exclude is non-empty, checks whose Name() is in exclude are skipped.
func GetChecks(mode string, categories []string, exclude []string, includeOnly []string) []Check {
	catSet := make(map[string]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}

	exclSet := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		exclSet[e] = true
	}

	inclSet := make(map[string]bool, len(includeOnly))
	for _, i := range includeOnly {
		inclSet[i] = true
	}

	var result []Check
	for _, c := range registry {
		// Filter by mode
		if mode != "" && c.Mode() != mode && c.Mode() != "both" {
			continue
		}
		// Filter by categories
		if len(catSet) > 0 && !catSet[c.Category()] {
			continue
		}
		// Filter by include-only (whitelist takes precedence)
		if len(inclSet) > 0 && !inclSet[c.Name()] {
			continue
		}
		// Filter by exclude (blacklist)
		if exclSet[c.Name()] {
			continue
		}
		result = append(result, c)
	}

	sort.Slice(result, func(i, j int) bool {
		if result[i].Category() != result[j].Category() {
			return result[i].Category() < result[j].Category()
		}
		return result[i].Name() < result[j].Name()
	})

	return result
}
```

- [ ] **Step 2: Update scanner Options to include filtering**

In `internal/scanner/scanner.go`, add Exclude and IncludeOnly to the Options struct (after line 22):

```go
type Options struct {
	Host        string
	Port        int
	DBName      string
	Categories  []string
	Exclude     []string
	IncludeOnly []string
	Mode        string
	Verbose     bool
}
```

And update the call to `check.GetChecks` on line 53:

```go
	checks := check.GetChecks(mode, opts.Categories, opts.Exclude, opts.IncludeOnly)
```

- [ ] **Step 3: Update analyzer to support filtering**

In `internal/analyzer/analyzer.go`, update `RunAnalyze` signature (line 100) to accept filtering:

```go
func RunAnalyze(schema *parser.ParsedSchema, filePath string, categories []string, exclude []string, includeOnly []string, verbose bool) (*models.ScanReport, error) {
```

Add exclude/includeOnly filtering logic inside the function, after the category filter (around line 132):

```go
	// Build exclude/includeOnly sets
	var exclSet map[string]bool
	if len(exclude) > 0 {
		exclSet = make(map[string]bool)
		for _, e := range exclude {
			exclSet[e] = true
		}
	}
	var inclSet map[string]bool
	if len(includeOnly) > 0 {
		inclSet = make(map[string]bool)
		for _, i := range includeOnly {
			inclSet[i] = true
		}
	}

	// Filter checks by category, exclude, includeOnly
	var checksToRun []CheckDef
	for _, check := range StaticChecks {
		if catFilter != nil && !catFilter[check.Category] {
			continue
		}
		if len(inclSet) > 0 && !inclSet[check.Name] {
			continue
		}
		if exclSet[check.Name] {
			continue
		}
		checksToRun = append(checksToRun, check)
	}
```

Also filter skipped checks the same way (around line 172):

```go
	for _, skip := range SkippedChecks {
		if catFilter != nil && !catFilter[skip.Category] {
			continue
		}
		if len(inclSet) > 0 && !inclSet[skip.Name] {
			continue
		}
		if exclSet[skip.Name] {
			continue
		}
		report.Results = append(report.Results, models.CheckResult{
			CheckName:   skip.Name,
			Category:    skip.Category,
			Description: skip.Description,
			Skipped:     true,
			SkipReason:  "Requires live database connection",
		})
	}
```

- [ ] **Step 4: Add --exclude and --include-only flags to CLI commands**

In `internal/cmd/scan.go`, add filtering vars and flags:

```go
var scanExclude string
var scanIncludeOnly string
```

In the scan `init()`, add:
```go
	scanCmd.Flags().StringVar(&scanExclude, "exclude", "", "Comma-separated list of check names to skip")
	scanCmd.Flags().StringVar(&scanIncludeOnly, "include-only", "", "Comma-separated list of check names to run (whitelist)")
```

Update the `runMode` function signature to accept exclude/includeOnly:

```go
func runMode(cf connFlags, of outputFlags, categories string, exclude string, includeOnly string, verbose bool, mode string) error {
```

Pass them through to scanner.Options:

```go
	report, err := scanner.RunScan(ctx, conn, scanner.Options{
		Host:        cf.Host,
		Port:        cf.Port,
		DBName:      cf.DBName,
		Categories:  cats,
		Exclude:     splitComma(exclude),
		IncludeOnly: splitComma(includeOnly),
		Mode:        mode,
		Verbose:     verbose,
	})
```

Update `runScan`:
```go
func runScan(cmd *cobra.Command, args []string) error {
	return runMode(scanConn, scanOut, scanCategories, scanExclude, scanIncludeOnly, scanVerbose, "scan")
}
```

Do the same for `audit.go`:
```go
var auditExclude string
var auditIncludeOnly string
```
Add flags in init(), update `runAudit` call.

Do the same for `monitor.go` (add vars, flags, pass through).

Do the same for `analyze.go`:
```go
var analyzeExclude string
var analyzeIncludeOnly string
```
Add flags in init(), update `runAnalyze` call to pass `splitComma(analyzeExclude)`, `splitComma(analyzeIncludeOnly)` to `analyzer.RunAnalyze`.

Do the same for `listchecks.go`:
```go
var listExclude string
var listIncludeOnly string
```
Add flags in init(), update `runListChecks` to call:
```go
	checks := check.GetChecks(mode, cats, splitComma(listExclude), splitComma(listIncludeOnly))
```

- [ ] **Step 5: Update all existing GetChecks call sites**

Search for any remaining `check.GetChecks` calls that still use the old 2-argument signature and update them to pass `nil, nil` for the new params. This includes the test file.

In `internal/checks/registry_test.go`, update all calls from:
```go
check.GetChecks("scan", nil)
```
to:
```go
check.GetChecks("scan", nil, nil, nil)
```

Do this for every `GetChecks` call in the test file.

- [ ] **Step 6: Add filtering tests to registry_test.go**

Add at the end of `internal/checks/registry_test.go`:

```go
func TestGetChecksExclude(t *testing.T) {
	all := check.GetChecks("", nil, nil, nil)
	excluded := check.GetChecks("", nil, []string{"primary_keys", "wal_level"}, nil)
	if len(excluded) != len(all)-2 {
		t.Errorf("expected %d checks after excluding 2, got %d", len(all)-2, len(excluded))
	}
	for _, c := range excluded {
		if c.Name() == "primary_keys" || c.Name() == "wal_level" {
			t.Errorf("excluded check %s still present", c.Name())
		}
	}
}

func TestGetChecksIncludeOnly(t *testing.T) {
	included := check.GetChecks("", nil, nil, []string{"primary_keys", "wal_level"})
	if len(included) != 2 {
		t.Errorf("expected 2 checks with include-only, got %d", len(included))
	}
}

func TestGetChecksIncludeOnlyTakesPrecedence(t *testing.T) {
	result := check.GetChecks("", nil, []string{"primary_keys"}, []string{"primary_keys"})
	if len(result) != 1 {
		t.Errorf("include-only should take precedence, expected 1 check, got %d", len(result))
	}
}
```

- [ ] **Step 7: Run tests**

Run: `go test ./internal/...`
Expected: All pass

- [ ] **Step 8: Commit**

```bash
git add internal/check/registry.go internal/scanner/scanner.go internal/analyzer/analyzer.go \
  internal/cmd/scan.go internal/cmd/audit.go internal/cmd/analyze.go internal/cmd/monitor.go \
  internal/cmd/listchecks.go internal/checks/registry_test.go
git commit -m "Add check filtering with --exclude and --include-only flags"
```

---

### Task 5: YAML Configuration File Support

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Modify: `go.mod` (add yaml.v3)
- Modify: `internal/cmd/root.go`
- Modify: `internal/cmd/scan.go`
- Modify: `internal/cmd/audit.go`
- Modify: `internal/cmd/analyze.go`
- Modify: `internal/cmd/monitor.go`

- [ ] **Step 1: Add yaml.v3 dependency**

Run: `go get gopkg.in/yaml.v3`

- [ ] **Step 2: Write config_test.go**

Create `internal/config/config_test.go`:

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigEmpty(t *testing.T) {
	cfg := Default()
	if len(cfg.Checks.Exclude) != 0 {
		t.Errorf("expected empty exclude, got %v", cfg.Checks.Exclude)
	}
	if cfg.Checks.IncludeOnly != nil {
		t.Errorf("expected nil include_only, got %v", cfg.Checks.IncludeOnly)
	}
	if !cfg.Report.TodoList {
		t.Error("expected todo_list=true by default")
	}
	if cfg.Report.TodoIncludeConsider {
		t.Error("expected todo_include_consider=false by default")
	}
}

func TestLoadConfigFromYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mm-ready.yaml")
	content := `
checks:
  exclude:
    - advisory_locks
    - temp_table_queries
report:
  todo_list: true
  todo_include_consider: true
scan:
  checks:
    include_only:
      - primary_keys
      - foreign_keys
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(cfg.Checks.Exclude) != 2 {
		t.Errorf("expected 2 global excludes, got %d", len(cfg.Checks.Exclude))
	}
	if !cfg.Report.TodoIncludeConsider {
		t.Error("expected todo_include_consider=true")
	}

	scanCfg := cfg.GetCheckConfig("scan")
	if scanCfg.IncludeOnly == nil || len(scanCfg.IncludeOnly) != 2 {
		t.Errorf("expected 2 scan include_only, got %v", scanCfg.IncludeOnly)
	}
	// Global excludes merge with mode config
	if len(scanCfg.Exclude) != 2 {
		t.Errorf("expected 2 merged excludes, got %d", len(scanCfg.Exclude))
	}
}

func TestMergeCLI(t *testing.T) {
	cfg := Default()
	check, report := MergeCLI(cfg, "scan", []string{"wal_level"}, nil, true, false)

	if len(check.Exclude) != 1 || check.Exclude[0] != "wal_level" {
		t.Errorf("expected CLI exclude, got %v", check.Exclude)
	}
	if report.TodoList {
		t.Error("expected todo_list=false with --no-todo")
	}
}

func TestMergeCLIIncludeOnlyOverrides(t *testing.T) {
	cfg := Default()
	check, _ := MergeCLI(cfg, "scan", nil, []string{"primary_keys"}, false, false)

	if check.IncludeOnly == nil || len(check.IncludeOnly) != 1 {
		t.Errorf("expected 1 include_only from CLI, got %v", check.IncludeOnly)
	}
}

func TestDiscoverConfigNoFile(t *testing.T) {
	// Change to temp dir with no config
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	path := DiscoverConfigFile()
	if path != "" {
		t.Errorf("expected empty path, got %s", path)
	}
}

func TestDiscoverConfigCwd(t *testing.T) {
	dir := t.TempDir()
	origDir, _ := os.Getwd()
	defer os.Chdir(origDir)
	os.Chdir(dir)

	cfgPath := filepath.Join(dir, "mm-ready.yaml")
	os.WriteFile(cfgPath, []byte("checks:\n  exclude: []\n"), 0o644)

	path := DiscoverConfigFile()
	if path != cfgPath {
		t.Errorf("expected %s, got %s", cfgPath, path)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/config/...`
Expected: FAIL (package doesn't exist yet)

- [ ] **Step 4: Write config.go**

Create `internal/config/config.go`:

```go
// Package config handles YAML configuration file loading for mm-ready-go.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// CheckConfig holds which checks to include/exclude.
type CheckConfig struct {
	Exclude     []string
	IncludeOnly []string // nil = no whitelist
}

// ReportConfig holds report generation options.
type ReportConfig struct {
	TodoList            bool
	TodoIncludeConsider bool
}

// Config is the complete configuration for mm-ready-go.
type Config struct {
	Checks     CheckConfig
	ModeChecks map[string]CheckConfig
	Report     ReportConfig
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Report: ReportConfig{TodoList: true},
	}
}

// GetCheckConfig returns the merged check config for a specific mode.
// Mode-specific excludes are unioned with global excludes.
// Mode-specific include_only overrides global if set.
func (c Config) GetCheckConfig(mode string) CheckConfig {
	global := c.Checks
	modeCfg, ok := c.ModeChecks[mode]
	if !ok {
		return global
	}

	// Merge excludes
	seen := make(map[string]bool)
	var merged []string
	for _, e := range global.Exclude {
		if !seen[e] {
			merged = append(merged, e)
			seen[e] = true
		}
	}
	for _, e := range modeCfg.Exclude {
		if !seen[e] {
			merged = append(merged, e)
			seen[e] = true
		}
	}

	// Mode include_only takes precedence
	includeOnly := global.IncludeOnly
	if modeCfg.IncludeOnly != nil {
		includeOnly = modeCfg.IncludeOnly
	}

	return CheckConfig{Exclude: merged, IncludeOnly: includeOnly}
}

// DiscoverConfigFile searches for mm-ready.yaml in cwd then home dir.
func DiscoverConfigFile() string {
	cwd, err := os.Getwd()
	if err == nil {
		p := filepath.Join(cwd, "mm-ready.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	home, err := os.UserHomeDir()
	if err == nil {
		p := filepath.Join(home, "mm-ready.yaml")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	return ""
}

// LoadFile loads configuration from a YAML file.
func LoadFile(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	var raw yamlConfig
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}

	return raw.toConfig(), nil
}

// MergeCLI merges CLI arguments with config file settings. CLI takes precedence.
func MergeCLI(cfg Config, mode string, cliExclude []string, cliIncludeOnly []string, cliNoTodo bool, cliTodoIncludeConsider bool) (CheckConfig, ReportConfig) {
	checkCfg := cfg.GetCheckConfig(mode)

	// CLI exclude adds to config exclude
	if len(cliExclude) > 0 {
		seen := make(map[string]bool)
		for _, e := range checkCfg.Exclude {
			seen[e] = true
		}
		for _, e := range cliExclude {
			if !seen[e] {
				checkCfg.Exclude = append(checkCfg.Exclude, e)
			}
		}
	}

	// CLI include_only completely overrides
	if len(cliIncludeOnly) > 0 {
		checkCfg.IncludeOnly = cliIncludeOnly
	}

	reportCfg := ReportConfig{
		TodoList:            !cliNoTodo && cfg.Report.TodoList,
		TodoIncludeConsider: cliTodoIncludeConsider || cfg.Report.TodoIncludeConsider,
	}

	return checkCfg, reportCfg
}

// YAML deserialization types (private)

type yamlConfig struct {
	Checks  yamlCheckConfig            `yaml:"checks"`
	Report  yamlReportConfig           `yaml:"report"`
	Scan    *yamlModeConfig            `yaml:"scan"`
	Audit   *yamlModeConfig            `yaml:"audit"`
	Analyze *yamlModeConfig            `yaml:"analyze"`
	Monitor *yamlModeConfig            `yaml:"monitor"`
}

type yamlCheckConfig struct {
	Exclude     []string `yaml:"exclude"`
	IncludeOnly []string `yaml:"include_only"`
}

type yamlReportConfig struct {
	TodoList            *bool `yaml:"todo_list"`
	TodoIncludeConsider *bool `yaml:"todo_include_consider"`
}

type yamlModeConfig struct {
	Checks yamlCheckConfig `yaml:"checks"`
}

func (y yamlConfig) toConfig() Config {
	cfg := Default()

	cfg.Checks.Exclude = y.Checks.Exclude
	if len(y.Checks.IncludeOnly) > 0 {
		cfg.Checks.IncludeOnly = y.Checks.IncludeOnly
	}

	if y.Report.TodoList != nil {
		cfg.Report.TodoList = *y.Report.TodoList
	}
	if y.Report.TodoIncludeConsider != nil {
		cfg.Report.TodoIncludeConsider = *y.Report.TodoIncludeConsider
	}

	cfg.ModeChecks = make(map[string]CheckConfig)
	for mode, mc := range map[string]*yamlModeConfig{
		"scan": y.Scan, "audit": y.Audit, "analyze": y.Analyze, "monitor": y.Monitor,
	} {
		if mc != nil {
			cc := CheckConfig{Exclude: mc.Checks.Exclude}
			if len(mc.Checks.IncludeOnly) > 0 {
				cc.IncludeOnly = mc.Checks.IncludeOnly
			}
			cfg.ModeChecks[mode] = cc
		}
	}

	return cfg
}
```

- [ ] **Step 5: Run config tests**

Run: `go test ./internal/config/... -v`
Expected: All pass

- [ ] **Step 6: Add config loading to CLI commands**

In `internal/cmd/root.go`, add config-related flags as shared variables:

```go
var configPath string
var noConfig bool
```

Create a helper function in `root.go`:

```go
func addConfigFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&configPath, "config", "", "Path to config file (default: auto-discover)")
	cmd.Flags().BoolVar(&noConfig, "no-config", false, "Skip config file loading")
}
```

In the scan/audit/analyze/monitor `init()` functions, add `addConfigFlags(scanCmd)` etc.

In `runMode`, load and merge config before running the scan:

```go
	// Load config
	var cfg config.Config
	if !noConfig {
		if configPath != "" {
			var err error
			cfg, err = config.LoadFile(configPath)
			if err != nil {
				return err
			}
		} else {
			path := config.DiscoverConfigFile()
			if path != "" {
				var err error
				cfg, err = config.LoadFile(path)
				if err != nil {
					return err
				}
				if verbose {
					fmt.Fprintf(os.Stderr, "Using config file: %s\n", path)
				}
			} else {
				cfg = config.Default()
			}
		}
	} else {
		cfg = config.Default()
	}

	checkCfg, reportCfg := config.MergeCLI(cfg, mode, splitComma(exclude), splitComma(includeOnly), false, false)
```

Then use `checkCfg.Exclude`, `checkCfg.IncludeOnly` when building scanner.Options (replace the CLI-only exclude/includeOnly). Pass `reportCfg` through to the reporter.

Do the same for analyze.go, adapting for its different flow.

- [ ] **Step 7: Run all tests**

Run: `go test ./internal/...`
Expected: All pass

- [ ] **Step 8: Commit**

```bash
git add internal/config/ go.mod go.sum internal/cmd/
git commit -m "Add YAML configuration file support"
```

---

### Task 6: Report Customization

**Files:**
- Modify: `internal/reporter/reporter.go`
- Modify: `internal/reporter/html.go:324-567`
- Modify: `internal/reporter/markdown.go`
- Modify: `internal/cmd/root.go`
- Modify: `internal/cmd/scan.go`
- Modify: `internal/cmd/audit.go`
- Modify: `internal/cmd/analyze.go`

- [ ] **Step 1: Add ReportOptions to reporter**

In `internal/reporter/reporter.go`, add options and update Render:

```go
package reporter

import (
	"fmt"

	"github.com/pgEdge/mm-ready-go/internal/models"
)

// ReportOptions controls report rendering behavior.
type ReportOptions struct {
	TodoList            bool
	TodoIncludeConsider bool
}

// DefaultReportOptions returns options with To Do list enabled.
func DefaultReportOptions() ReportOptions {
	return ReportOptions{TodoList: true}
}

// Render dispatches to the appropriate renderer based on format.
func Render(report *models.ScanReport, format string, opts ReportOptions) (string, error) {
	switch format {
	case "json":
		return RenderJSON(report), nil
	case "markdown":
		return RenderMarkdown(report), nil
	case "html":
		return RenderHTML(report, opts), nil
	default:
		return "", fmt.Errorf("unknown format: %s", format)
	}
}
```

- [ ] **Step 2: Update RenderHTML to accept ReportOptions**

In `internal/reporter/html.go`, change the signature (line 324):

```go
func RenderHTML(report *models.ScanReport, opts ReportOptions) string {
```

Update the To Do list collection (lines 337-344) to respect options:

```go
	// Collect to-do items
	var todoItems []models.Finding
	if opts.TodoList {
		todoSevs := []models.Severity{models.SeverityCritical, models.SeverityWarning}
		if opts.TodoIncludeConsider {
			todoSevs = append(todoSevs, models.SeverityConsider)
		}
		for _, sev := range todoSevs {
			for _, f := range allFindings {
				if f.Severity == sev && f.Remediation != "" {
					todoItems = append(todoItems, f)
				}
			}
		}
	}
```

- [ ] **Step 3: Add --no-todo and --todo-include-consider flags**

In `internal/cmd/root.go`, add shared report flag vars:

```go
var noTodo bool
var todoIncludeConsider bool
```

Add a helper:

```go
func addReportFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&noTodo, "no-todo", false, "Omit To Do list from report")
	cmd.Flags().BoolVar(&todoIncludeConsider, "todo-include-consider", false, "Include CONSIDER items in To Do list")
}
```

Add `addReportFlags()` calls in scan, audit, and analyze `init()` functions.

- [ ] **Step 4: Wire report options through to Render calls**

In `runMode` (scan.go), update the Render call:

```go
	reportOpts := reporter.ReportOptions{
		TodoList:            reportCfg.TodoList,
		TodoIncludeConsider: reportCfg.TodoIncludeConsider,
	}
	output, err := reporter.Render(report, of.Format, reportOpts)
```

Update the `MergeCLI` call to pass `noTodo` and `todoIncludeConsider`:

```go
	checkCfg, reportCfg := config.MergeCLI(cfg, mode, splitComma(exclude), splitComma(includeOnly), noTodo, todoIncludeConsider)
```

Do the same for analyze.go's render call.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/...`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add internal/reporter/ internal/cmd/
git commit -m "Add --no-todo and --todo-include-consider report options"
```

---

### Task 7: LOLOR Check

**Files:**
- Create: `internal/checks/extensions/lolor_check.go`
- Modify: `internal/checks/registry_test.go`
- Modify: `internal/analyzer/analyzer.go` (update SkippedChecks)

- [ ] **Step 1: Update registry_test.go expected counts first**

In `internal/checks/registry_test.go`:
- Change `if len(all) != 56` to `if len(all) != 57` (line 12)
- Change `if len(all) != 56` to `if len(all) != 57` (line 129)
- Change `total != 56` to `total != 57` (line 166)
- Change `"extensions": 4` to `"extensions": 5` (line 58)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/checks/... -v -run TestTotalCheckCount`
Expected: FAIL with "expected 57 checks, got 56"

- [ ] **Step 3: Create lolor_check.go**

Create `internal/checks/extensions/lolor_check.go`:

```go
// Check for LOLOR extension — required for replicating large objects.
package extensions

import (
	"context"
	"fmt"

	"github.com/pgEdge/mm-ready-go/internal/check"
	"github.com/pgEdge/mm-ready-go/internal/models"
	"github.com/jackc/pgx/v5"
)

// LolorCheck verifies whether the LOLOR extension is needed and configured.
type LolorCheck struct{}

func init() {
	check.Register(LolorCheck{})
}

func (LolorCheck) Name() string     { return "lolor_check" }
func (LolorCheck) Category() string  { return "extensions" }
func (LolorCheck) Mode() string      { return "scan" }
func (LolorCheck) Description() string {
	return "LOLOR extension — required for replicating large objects"
}

func (c LolorCheck) Run(ctx context.Context, conn *pgx.Conn) ([]models.Finding, error) {
	// Check if large objects exist
	var lobCount int
	err := conn.QueryRow(ctx, "SELECT count(*) FROM pg_catalog.pg_largeobject_metadata;").Scan(&lobCount)
	if err != nil {
		return nil, fmt.Errorf("lolor_check: query large objects: %w", err)
	}

	// Check for OID-typed columns
	var oidColCount int
	err = conn.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_catalog.pg_attribute a
		JOIN pg_catalog.pg_class c ON c.oid = a.attrelid
		JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relkind = 'r'
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		  AND n.nspname NOT IN ('pg_catalog', 'information_schema', 'spock', 'pg_toast')
		  AND a.atttypid = 'oid'::regtype;
	`).Scan(&oidColCount)
	if err != nil {
		return nil, fmt.Errorf("lolor_check: query OID columns: %w", err)
	}

	if lobCount == 0 && oidColCount == 0 {
		return nil, nil
	}

	// Check if LOLOR is installed
	var extVersion *string
	_ = conn.QueryRow(ctx,
		`SELECT extversion FROM pg_catalog.pg_extension WHERE extname = 'lolor';`,
	).Scan(&extVersion)

	if extVersion == nil {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "Large objects detected but LOLOR extension is not installed",
			Detail: fmt.Sprintf("Found %d large object(s) and %d OID-type column(s). "+
				"PostgreSQL's logical decoding does not support large objects. "+
				"The LOLOR (Large Object Logical Replication) extension is required "+
				"to replicate large objects with Spock.", lobCount, oidColCount),
			ObjectName: "lolor",
			Remediation: "Install and configure the LOLOR extension:\n" +
				"  CREATE EXTENSION lolor;\n" +
				"  ALTER SYSTEM SET lolor.node = <unique_node_id>;\n" +
				"  -- Restart PostgreSQL\n" +
				"Each node must have a unique lolor.node value (1 to 2^28). " +
				"Then add lolor tables to the replication set:\n" +
				"  SELECT spock.repset_add_table('default', 'lolor.pg_largeobject');\n" +
				"  SELECT spock.repset_add_table('default', 'lolor.pg_largeobject_metadata');",
			Metadata: map[string]any{"lob_count": lobCount, "oid_col_count": oidColCount},
		}}, nil
	}

	// LOLOR installed — check lolor.node
	var nodeVal *string
	_ = conn.QueryRow(ctx, "SELECT current_setting('lolor.node');").Scan(&nodeVal)

	if nodeVal == nil || *nodeVal == "0" {
		return []models.Finding{{
			Severity:  models.SeverityWarning,
			CheckName: c.Name(),
			Category:  c.Category(),
			Title:     "LOLOR installed but lolor.node is not configured",
			Detail: fmt.Sprintf("LOLOR extension v%s is installed, but lolor.node is not set "+
				"(or set to 0). Each node must have a unique lolor.node value for large "+
				"object replication to work correctly.", *extVersion),
			ObjectName: "lolor.node",
			Remediation: "Set a unique node identifier:\n" +
				"  ALTER SYSTEM SET lolor.node = <unique_id>;\n" +
				"  -- Restart PostgreSQL\n" +
				"The value must be unique across all nodes (1 to 2^28).",
		}}, nil
	}

	return []models.Finding{{
		Severity:  models.SeverityInfo,
		CheckName: c.Name(),
		Category:  c.Category(),
		Title:     fmt.Sprintf("LOLOR extension installed (v%s, node=%s)", *extVersion, *nodeVal),
		Detail: fmt.Sprintf("LOLOR is installed and lolor.node is set to %s. "+
			"Ensure this value is unique across all cluster nodes and that "+
			"lolor.pg_largeobject and lolor.pg_largeobject_metadata are members of a replication set.",
			*nodeVal),
		ObjectName: "lolor",
		Metadata:   map[string]any{"version": *extVersion, "node": *nodeVal},
	}}, nil
}
```

- [ ] **Step 4: Verify the SkippedChecks in analyzer already has lolor_check**

Check `internal/analyzer/analyzer.go` line 78: `{"lolor_check", "extensions", "LOLOR extension for large object replication"}` is already in the SkippedChecks list. No change needed.

- [ ] **Step 5: Run tests**

Run: `go test ./internal/...`
Expected: All pass (57 checks total)

- [ ] **Step 6: Commit**

```bash
git add internal/checks/extensions/lolor_check.go internal/checks/registry_test.go
git commit -m "Add LOLOR extension check for large object replication"
```

---

### Task 8: Bug Fixes

**Files:**
- Modify: `internal/checks/functions/views_audit.go:23`
- Modify: `internal/analyzer/checks.go:120-180`
- Modify: `internal/checks/registry_test.go`
- Modify: `internal/reporter/html.go` (version strings)
- Modify: `internal/reporter/markdown.go` (version string)
- Modify: `internal/reporter/json.go` (tool name and version)

- [ ] **Step 1: Fix views_audit mode to "both"**

In `internal/checks/functions/views_audit.go`, line 23, change:
```go
func (ViewsAuditCheck) Mode() string         { return "both" }
```

- [ ] **Step 2: Update registry_test expected counts for views_audit mode change**

In `internal/checks/registry_test.go`, the `TestScanAndAuditCountsAddUp` test (line 153) already counts "both" mode checks. The views_audit moving from "scan" to "both" means:
- It appears in both scan AND audit results
- The `bothCount` increases by 1
- `total = scan + audit - both` stays the same (57)

No test changes needed for this specific fix, but verify by running the tests.

- [ ] **Step 3: Fix analyzer checkSequencePKs to skip snowflake.nextval()**

In `internal/analyzer/checks.go`, in the `checkSequencePKs` function (around line 152), add a snowflake skip after the `nextval` extraction:

```go
			} else if col.DefaultExpr != "" && strings.Contains(strings.ToLower(col.DefaultExpr), "nextval(") {
				// Skip snowflake sequences — already globally unique
				if strings.Contains(strings.ToLower(col.DefaultExpr), "snowflake") {
					continue
				}
				isSequenceBacked = true
				if m := reNextval.FindStringSubmatch(col.DefaultExpr); m != nil {
					seqName = m[1]
				} else {
					seqName = col.DefaultExpr
				}
```

- [ ] **Step 4: Update version strings in reporters**

In `internal/reporter/html.go`:
- Line 408: change `mm-ready v0.1.0` to `mm-ready-go v0.1.0`
- Line 570: change `mm-ready v0.1.0` to `mm-ready-go v0.1.0`

In `internal/reporter/markdown.go`:
- Line 122: change `mm-ready v0.1.0` to `mm-ready-go v0.1.0`

In `internal/reporter/json.go`:
- Line 58: change `"mm-ready"` to `"mm-ready-go"`

- [ ] **Step 5: Run tests**

Run: `go test ./internal/...`
Expected: All pass

- [ ] **Step 6: Commit**

```bash
git add internal/checks/functions/views_audit.go internal/analyzer/checks.go \
  internal/reporter/html.go internal/reporter/markdown.go internal/reporter/json.go
git commit -m "Fix views_audit mode, skip snowflake sequences in analyzer, update tool name"
```

---

### Task 9: CI Workflow

**Files:**
- Create: `.github/workflows/ci.yml`
- Create: `.golangci.yml`

- [ ] **Step 1: Create .golangci.yml**

Create `.golangci.yml`:

```yaml
run:
  timeout: 5m
  go: "1.25"

linters:
  enable:
    - govet
    - errcheck
    - staticcheck
    - gosimple
    - ineffassign
    - unused
    - gocritic
    - gofmt
    - goimports

linters-settings:
  gocritic:
    enabled-tags:
      - diagnostic
      - style
```

- [ ] **Step 2: Create CI workflow**

Create `.github/workflows/ci.yml`:

```yaml
name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]
  workflow_dispatch:

jobs:
  lint:
    name: Lint
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.25"

      - name: Go vet
        run: go vet ./...

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v6
        with:
          version: latest

  test:
    name: Unit Tests
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.25"

      - name: Run unit tests
        run: go test ./internal/... -v

  integration:
    name: Integration Tests
    runs-on: ubuntu-latest
    services:
      postgres:
        image: ghcr.io/pgedge/pgedge-postgres:18.1-spock5.0.4-standard-1
        env:
          POSTGRES_PASSWORD: postgres
          POSTGRES_DB: mmready
        ports:
          - 5432:5432
        options: >-
          --health-cmd "pg_isready -h localhost"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.25"

      - name: Install PostgreSQL client
        run: |
          sudo apt-get install -y curl ca-certificates
          sudo install -d /usr/share/postgresql-common/pgdg
          sudo curl -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc --fail https://www.postgresql.org/media/keys/ACCC4CF8.asc
          echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] https://apt.postgresql.org/pub/repos/apt $(lsb_release -cs)-pgdg main" | sudo tee /etc/apt/sources.list.d/pgdg.list
          sudo apt-get update
          sudo apt-get install -y postgresql-client-18

      - name: Configure pg_stat_statements
        run: |
          PGPASSWORD=postgres psql -h localhost -U postgres -c "ALTER SYSTEM SET shared_preload_libraries = 'pg_stat_statements';"
          # Restart is handled by Docker service restart below

      - name: Load test schema
        run: |
          PGPASSWORD=postgres psql -h localhost -U postgres -d mmready -f tests/test_schema_setup.sql

      - name: Load test workload
        run: |
          PGPASSWORD=postgres psql -h localhost -U postgres -d mmready -f tests/test_workload.sql

      - name: Run integration tests
        env:
          PGHOST: localhost
          PGPORT: "5432"
          PGDATABASE: mmready
          PGUSER: postgres
          PGPASSWORD: postgres
        run: go test -tags integration ./tests/ -v

      - name: Build binary
        run: go build -o bin/mm-ready-go .

      - name: Run scan and verify output
        run: |
          ./bin/mm-ready-go scan \
            --host localhost --port 5432 \
            --dbname mmready --user postgres --password postgres \
            --format json --output scan-report.json -v
          python3 -c "
          import json
          with open('scan-report.json') as f:
              report = json.load(f)
          s = report['summary']
          print(f\"Checks run: {s['total_checks']}\")
          assert s['total_checks'] > 40, 'Expected 40+ checks to run'
          print('Scan verification passed!')
          "

      - name: Test analyze mode
        run: |
          PGPASSWORD=postgres /usr/lib/postgresql/18/bin/pg_dump -h localhost -U postgres -d mmready --schema-only > schema.sql
          ./bin/mm-ready-go analyze --file schema.sql --format json --output analyze-report.json -v
          python3 -c "
          import json
          with open('analyze-report.json') as f:
              report = json.load(f)
          s = report['summary']
          print(f\"Analyze checks: {s['total_checks']}\")
          assert s['total_checks'] >= 15, 'Expected 15+ analyze checks'
          print('Analyze verification passed!')
          "

      - name: Upload reports
        uses: actions/upload-artifact@v4
        if: always()
        with:
          name: test-reports
          path: |
            scan-report.json
            analyze-report.json
            schema.sql
```

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml .golangci.yml
git commit -m "Add CI workflow with lint, test, and integration jobs"
```

---

### Task 10: GitHub Templates & Code Quality

**Files:**
- Create: `.github/CODEOWNERS`
- Create: `.github/PULL_REQUEST_TEMPLATE.md`
- Create: `.github/ISSUE_TEMPLATE/bug_report.md`
- Create: `.github/ISSUE_TEMPLATE/feature_request.md`
- Create: `.pre-commit-config.yaml`
- Create: `.codacy.yaml`
- Create: `.coderabbit.yaml`

- [ ] **Step 1: Create CODEOWNERS**

Create `.github/CODEOWNERS`:
```
* @AntTheLimey
```

- [ ] **Step 2: Create PR template**

Create `.github/PULL_REQUEST_TEMPLATE.md`:
```markdown
## Summary
Brief description of changes.

## Test Plan
- [ ] Existing tests pass (`go test ./internal/...`)
- [ ] Linting passes (`go vet ./...`)
- [ ] Build succeeds (`go build -o bin/mm-ready-go .`)
- [ ] New tests added for new functionality (if applicable)

## Notes
Any additional context for reviewers.
```

- [ ] **Step 3: Create issue templates**

Create `.github/ISSUE_TEMPLATE/bug_report.md`:
```markdown
---
name: Bug Report
about: Report a bug
labels: bug
---

## Description
Clear description of the bug.

## Steps to Reproduce
1.
2.
3.

## Expected Behaviour
What you expected to happen.

## Actual Behaviour
What actually happened.

## Environment
- mm-ready-go version:
- Go version:
- PostgreSQL version:
- OS:
```

Create `.github/ISSUE_TEMPLATE/feature_request.md`:
```markdown
---
name: Feature Request
about: Suggest a new feature
labels: enhancement
---

## Problem
Describe the problem or need.

## Proposed Solution
What you'd like to happen.

## Alternatives Considered
Any alternative approaches you've considered.
```

- [ ] **Step 4: Create pre-commit config**

Create `.pre-commit-config.yaml`:
```yaml
repos:
  - repo: https://github.com/pre-commit/pre-commit-hooks
    rev: v5.0.0
    hooks:
      - id: trailing-whitespace
      - id: end-of-file-fixer
      - id: check-yaml

  - repo: https://github.com/golangci/golangci-lint
    rev: v2.1.6
    hooks:
      - id: golangci-lint

  - repo: https://github.com/gitleaks/gitleaks
    rev: v8.24.0
    hooks:
      - id: gitleaks
```

- [ ] **Step 5: Create codacy and coderabbit configs**

Create `.codacy.yaml`:
```yaml
exclude_paths:
  - tests/*.sql
  - docs/**
```

Create `.coderabbit.yaml`:
```yaml
language: go
reviews:
  auto_review:
    enabled: true
    incremental: true
  tools:
    golangci-lint:
      enabled: true
    gitleaks:
      enabled: true
    yamllint:
      enabled: true
    markdownlint:
      enabled: true
```

- [ ] **Step 6: Commit**

```bash
git add .github/ .pre-commit-config.yaml .codacy.yaml .coderabbit.yaml
git commit -m "Add GitHub templates, CODEOWNERS, and code quality configs"
```

---

### Task 11: MkDocs & Documentation

**Files:**
- Create: `mkdocs.yml`
- Create: `docs/overrides/partials/logo.html`
- Create: `docs/stylesheets/extra.css`
- Create: `docs/tutorial.md`
- Copy: `docs/img/logo-dark.png`, `docs/img/logo-light.png`, `docs/img/favicon.ico`
- Modify: `docs/index.md`
- Modify: `docs/quickstart.md`
- Modify: `docs/architecture.md`
- Modify: `docs/checks-reference.md`

- [ ] **Step 1: Copy images from Python repo**

```bash
mkdir -p docs/img docs/overrides/partials docs/stylesheets
cp /Users/apegg/PROJECTS/MM_Ready/docs/img/logo-dark.png docs/img/
cp /Users/apegg/PROJECTS/MM_Ready/docs/img/logo-light.png docs/img/
cp /Users/apegg/PROJECTS/MM_Ready/docs/img/favicon.ico docs/img/
```

- [ ] **Step 2: Copy theme overrides from Python repo**

```bash
cp /Users/apegg/PROJECTS/MM_Ready/docs/overrides/partials/logo.html docs/overrides/partials/
cp /Users/apegg/PROJECTS/MM_Ready/docs/stylesheets/extra.css docs/stylesheets/
```

- [ ] **Step 3: Create mkdocs.yml**

Create `mkdocs.yml`:
```yaml
site_name: mm-ready-go

extra_css:
  - stylesheets/extra.css

extra_javascript:
  - https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js

markdown_extensions:
  - admonition
  - pymdownx.details
  - pymdownx.superfences:
      custom_fences:
        - name: mermaid
          class: mermaid
          format: !!python/name:pymdownx.superfences.fence_code_format

plugins:
  - search
  - macros:
      j2_variable_start_string: "<<"
      j2_variable_end_string: ">>"
      j2_comment_start_string: "<#"
      j2_comment_end_string: "#>"

theme:
  name: material
  custom_dir: docs/overrides
  favicon: img/favicon.ico

  logo_dark_mode: 'img/logo-dark.png'
  logo_light_mode: 'img/logo-light.png'

  features:
    - navigation.instant
    - navigation.tracking
    - navigation.prune
    - navigation.top
    - toc.follow

  palette:
    - media: "(prefers-color-scheme: light)"
      scheme: default
      primary: white
      accent: cyan
      toggle:
        icon: material/brightness-7
        name: Switch to dark mode

    - media: "(prefers-color-scheme: dark)"
      scheme: slate
      primary: black
      accent: cyan
      toggle:
        icon: material/brightness-4
        name: Switch to system preference

extra:
  generator: false

copyright: Copyright &copy; 2025 - 2026 pgEdge, Inc
repo_url: https://github.com/pgEdge/mm-ready-go

nav:
  - About mm-ready-go:
      - Overview: index.md
      - Quickstart: quickstart.md
      - Tutorial: tutorial.md
  - Checks Reference:
      - All Checks: checks-reference.md
  - Architecture:
      - System Architecture: architecture.md
```

- [ ] **Step 4: Update docs/index.md**

Read the current `docs/index.md`, then rewrite it for Go. Replace Python install instructions with:
```
go install github.com/pgEdge/mm-ready-go@latest
```
Replace all `mm-ready` binary references with `mm-ready-go`. Add config file and filtering documentation. Update copyright to pgEdge.

- [ ] **Step 5: Update docs/quickstart.md**

Read the current file, then adapt all examples to use `mm-ready-go` binary name, `go install` for installation, and Go-specific build from source instructions.

- [ ] **Step 6: Create docs/tutorial.md**

Adapt from the Python version's tutorial, using `mm-ready-go` commands and Go-specific details. Include Docker setup, scan, audit, analyze, and monitor walkthroughs.

- [ ] **Step 7: Update docs/architecture.md**

Read the current file. Update module paths from `AntTheLimey` to `pgEdge`, binary name to `mm-ready-go`, and add new packages (`internal/config/`).

- [ ] **Step 8: Update docs/checks-reference.md**

Read the current file. Add the `lolor_check` entry to the extensions section. Ensure total count says 57 checks.

- [ ] **Step 9: Commit**

```bash
git add mkdocs.yml docs/
git commit -m "Add MkDocs documentation site with pgEdge branding"
```

---

### Task 12: README & CLAUDE.md

**Files:**
- Modify: `README.md`
- Modify: `CLAUDE.md`

- [ ] **Step 1: Rewrite README.md**

Read the current README.md and the Python README.md for reference. Rewrite to match the Python README structure:
- Project description referencing pgEdge Spock 5
- 57 automated checks, 7 categories
- 4 operational modes (scan, audit, analyze, monitor)
- 3 output formats
- Installation: `go install github.com/pgEdge/mm-ready-go@latest`
- Usage examples for all modes with `mm-ready-go` binary name
- Configuration file section (mm-ready.yaml format)
- Check filtering (--exclude, --include-only)
- SSL/TLS connection flags
- Report options (--no-todo, --todo-include-consider)
- Check categories table (all 57 checks)
- Architecture (Go directory tree including new files)
- Building from source (`make build`, `make build-all`)
- Contributing & development
- License: pgEdge, Inc, PostgreSQL License

- [ ] **Step 2: Update CLAUDE.md**

Read the current CLAUDE.md. Update:
- All module path references to `github.com/pgEdge/mm-ready-go`
- Binary name to `mm-ready-go`
- Add `internal/config/` to architecture tree
- Update check count to 57 (56 + LOLOR)
- Update extensions count to 5
- Add documentation for new features (SSL, config, filtering, report options)
- Update build and run examples to use `mm-ready-go`
- Update Docker integration test examples with `mm-ready-go`

- [ ] **Step 3: Commit**

```bash
git add README.md CLAUDE.md
git commit -m "Rewrite README and update CLAUDE.md for pgEdge mm-ready-go"
```

---

### Task 13: Final Verification

**Files:** None (verification only)

- [ ] **Step 1: Run full test suite**

Run: `go test ./...`
Expected: All pass

- [ ] **Step 2: Run go vet**

Run: `go vet ./...`
Expected: No issues

- [ ] **Step 3: Build binary**

Run: `go build -o bin/mm-ready-go .`
Expected: Clean build

- [ ] **Step 4: Verify help output**

Run: `./bin/mm-ready-go --help`
Expected: Shows "mm-ready-go" in usage, lists all subcommands

Run: `./bin/mm-ready-go scan --help`
Expected: Shows --sslmode, --exclude, --include-only, --config, --no-config, --no-todo, --todo-include-consider flags

- [ ] **Step 5: Verify no AntTheLimey references remain in Go source**

Run: `grep -r "AntTheLimey" --include="*.go" .`
Expected: No matches

Run: `grep -r "AntTheLimey" go.mod`
Expected: No matches

- [ ] **Step 6: Verify check count**

Run: `./bin/mm-ready-go list-checks | tail -1`
Expected: Shows checks from all 7 categories

- [ ] **Step 7: Spot check docs for stale references**

Run: `grep -r "AntTheLimey" docs/ README.md CLAUDE.md`
Expected: No matches (unless in git history context)
