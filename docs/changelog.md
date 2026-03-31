# Changelog

All notable changes to mm-ready-go are documented in this file.

The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to
[Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-03-31

This is the initial release of mm-ready-go under the pgEdge
organization.

### Added

- 57 automated checks across 7 categories (schema,
  replication, config, extensions, SQL patterns, functions,
  sequences).
- Four operational modes: scan, audit, analyze, monitor.
- Three output formats: HTML, Markdown, JSON.
- SSL/TLS connection support with `--sslmode`, `--sslcert`,
  `--sslkey`, and `--sslrootcert` flags.
- PG* environment variable fallback for all connection
  parameters.
- YAML configuration file support with `--config` and
  `--no-config` flags.
- Check filtering with `--exclude` and `--include-only`
  flags.
- Report customization with `--no-todo` and
  `--todo-include-consider` flags.
- LOLOR extension check for large object replication.
- CI pipeline with lint, unit test, and integration test
  jobs.
- MkDocs Material documentation site with pgEdge branding.
