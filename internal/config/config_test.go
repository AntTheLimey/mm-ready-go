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
	// Resolve symlinks so the comparison works on macOS (/var -> /private/var)
	dir, _ = filepath.EvalSymlinks(dir)
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
