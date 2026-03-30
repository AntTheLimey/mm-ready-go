# mm-ready-go: pgEdge Migration & Feature Parity Design

**Date:** 2026-03-30
**Status:** Approved

## Goal

Transform MM_Ready_Go from a personal project (`github.com/AntTheLimey/mm-ready`)
into a pgEdge company tool (`github.com/pgEdge/mm-ready-go`) with full feature
parity against the Python version (`github.com/pgEdge/mm-ready`), production CI/CD,
and pgEdge-branded documentation.

## Decisions

- **Separate repository:** Go version lives at `github.com/pgEdge/mm-ready-go`,
  not replacing the Python version.
- **Module path:** `github.com/pgEdge/mm-ready-go`
- **Binary name:** `mm-ready-go` (avoids PATH conflict with Python `mm-ready`)
- **Config file format:** Identical to Python version (`mm-ready.yaml`) for
  transferability between tools.
- **CI integration tests:** Use `ghcr.io/pgedge/pgedge-postgres:18.1-spock5.0.4-standard-1`
  (publicly accessible, enables audit-mode testing). Falls back to `postgres:18` if
  image becomes unavailable.
- **Go linting:** `go vet` + `golangci-lint` (bundles staticcheck and others).
- **Docs:** Copy MkDocs Material setup from Python repo, adapt content for Go.
- **Deployment:** Work done locally, push final version to AntTheLimey as a last
  update, then push to new private `pgEdge/mm-ready-go` repo (no repo transfer).

## Section 1: Module Path & Binary Rename

### Module path

- `github.com/AntTheLimey/mm-ready` -> `github.com/pgEdge/mm-ready-go`
- Affects `go.mod` and all ~79 Go source files with import statements.
- Mechanical find-and-replace in a single commit.

### Binary

- `mm-ready` -> `mm-ready-go` in Makefile, README, docs, CLI help text.
- `cobra.Command.Use` in `root.go` updated.
- Cross-compilation outputs: `mm-ready-go-linux-amd64`, `mm-ready-go-darwin-arm64`, etc.

### License

- Copyright: "Antony Pegg" -> "pgEdge, Inc"

### Git remote

- Not changed during this work. User will manually create `pgEdge/mm-ready-go`
  and push when ready.

## Section 2: Feature Implementations

### 2a. SSL/TLS Connection Support

**File:** `internal/connection/connection.go`

- Add fields to `Config`: `SSLMode`, `SSLCert`, `SSLKey`, `SSLRootCert`.
- Build pgx TLS config based on sslmode (disable, require, verify-ca, verify-full).
- Explicit PG* environment variable fallback: `PGHOST`, `PGPORT`, `PGDATABASE`,
  `PGUSER`, `PGPASSWORD`, `PGSSLMODE`, `PGSSLCERT`, `PGSSLKEY`, `PGSSLROOTCERT`.
- Precedence: CLI flags > PG* env vars > DSN components > defaults.

**CLI changes:** Add `--sslmode`, `--sslcert`, `--sslkey`, `--sslrootcert` flags
to `scan.go`, `audit.go`, `monitor.go`.

### 2b. Check Filtering

**File:** `internal/check/registry.go`

- `GetChecks()` gains `exclude []string` and `includeOnly []string` parameters.
- `includeOnly` is a whitelist (only run named checks).
- `exclude` is a blacklist (skip named checks).
- `includeOnly` takes precedence if both specified.

**CLI changes:** Add `--exclude` and `--include-only` flags (comma-separated)
to `scan.go`, `audit.go`, `analyze.go`, `monitor.go`, `listchecks.go`.

### 2c. YAML Configuration File Support

**New file:** `internal/config/config.go`

- Config struct matching Python's YAML format exactly.
- `LoadConfig(path string)` - load explicit path.
- `DiscoverConfig()` - search `./mm-ready.yaml`, then `~/.mm-ready.yaml`.
- Merges with CLI flags (CLI takes precedence).
- New dependency: `gopkg.in/yaml.v3`.

**CLI changes:** Add `--config` and `--no-config` flags to scan/audit/analyze/monitor.

### 2d. Report Customization

- `--no-todo` flag: omit To Do list from HTML/Markdown reports.
- `--todo-include-consider` flag: include CONSIDER items in To Do list.

### 2e. LOLOR Check

**New file:** `internal/checks/extensions/lolor_check.go`

- Check for LOLOR extension availability for large object replication.
- Mode: "scan".
- Mirrors Python's `lolor_check.py`.

### 2f. Bug Fixes

- `views_audit` mode: change from "scan" to "both".
- Analyzer `sequence_pks`: skip sequences with `snowflake.nextval()` defaults
  to avoid false positives.

## Section 3: CI/CD & Code Quality

### GitHub Actions (`.github/workflows/ci.yml`)

**Triggers:** push to main, pull requests to main, workflow_dispatch.

**Jobs:**

1. **lint** (Ubuntu, Go 1.25): `go vet ./...` + `golangci-lint run`
2. **test** (Ubuntu, Go 1.25): `go test ./internal/...` (unit tests)
3. **integration** (Ubuntu, Go 1.25, Spock container):
   - Service: `ghcr.io/pgedge/pgedge-postgres:18.1-spock5.0.4-standard-1`
   - Load test schema + workload
   - `go test -tags integration ./tests/ -v`
   - Build binary, run scan, verify JSON output
   - Test analyze mode (pg_dump + analyze)
   - Upload report artifacts

### golangci-lint (`.golangci.yml`)

Linters: govet, errcheck, staticcheck, gosimple, ineffassign, unused, gocritic,
gofmt, goimports. Timeout: 5 minutes.

### GitHub Config

- `.github/CODEOWNERS` -> `@AntTheLimey`
- `.github/PULL_REQUEST_TEMPLATE.md` - adapted for Go
- `.github/ISSUE_TEMPLATE/bug_report.md` - Go version, PG version, OS
- `.github/ISSUE_TEMPLATE/feature_request.md`

### Code Quality

- `.pre-commit-config.yaml`: trailing-whitespace, EOF fixer, check-yaml,
  golangci-lint, gitleaks
- `.codacy.yaml`: exclude test SQL, docs
- `.coderabbit.yaml`: auto-review, Go tools

## Section 4: Documentation & Branding

### MkDocs

Copied from Python repo, adapted:

- `mkdocs.yml`: site_name `mm-ready-go`, repo_url `pgEdge/mm-ready-go`,
  copyright pgEdge Inc, same Material theme and plugins.
- `docs/overrides/partials/logo.html` - same logo toggle.
- `docs/stylesheets/extra.css` - same CSS.
- `docs/img/` - logo-dark.png, logo-light.png, favicon.ico (from Python repo).

### Doc Pages

Same navigation, Go-adapted content:

- `docs/index.md` - install via `go install`, feature overview
- `docs/quickstart.md` - 5-minute guide with Go binary
- `docs/tutorial.md` - walkthrough with `mm-ready-go` binary
- `docs/checks-reference.md` - all 57 checks (56 existing + LOLOR)
- `docs/architecture.md` - Go architecture, updated paths

### README.md

Full rewrite matching Python structure: features, `go install`, usage for all
4 modes, config file docs, check filtering, categories table, architecture,
building, contributing, pgEdge branding.

### CLAUDE.md

Update module paths, binary name, add new features (SSL, config, filtering),
update architecture tree.

## Section 5: Commit Ordering

| # | Commit | Scope |
|---|--------|-------|
| 1 | Module path rename | go.mod, all imports, binary name in Makefile |
| 2 | License & copyright | LICENSE.md -> pgEdge, Inc |
| 3 | SSL/TLS connection support | connection.go, CLI flags, PG* env vars |
| 4 | Check filtering | registry.go, --exclude/--include-only flags |
| 5 | YAML config file support | new internal/config/, gopkg.in/yaml.v3 |
| 6 | Report customization | --no-todo, --todo-include-consider |
| 7 | LOLOR check | new check file in extensions/ |
| 8 | Bug fixes | views_audit mode, snowflake analyzer skip |
| 9 | CI workflow | ci.yml, .golangci.yml |
| 10 | GitHub templates & code quality | CODEOWNERS, templates, pre-commit, codacy, coderabbit |
| 11 | MkDocs & documentation | mkdocs.yml, theme, all doc pages |
| 12 | README & CLAUDE.md | final rewrite |
| 13 | Registry test update | expected check count -> 57 (56 + LOLOR) |
