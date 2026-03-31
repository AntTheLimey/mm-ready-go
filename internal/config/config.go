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
	// Exclude lists check names to skip.
	Exclude     []string
	IncludeOnly []string // nil = no whitelist
}

// ReportConfig holds report generation options.
type ReportConfig struct {
	// TodoList controls whether the To Do list is included.
	TodoList bool
	// TodoIncludeConsider includes CONSIDER items in the To Do list.
	TodoIncludeConsider bool
}

// Config is the complete configuration for mm-ready-go.
type Config struct {
	// Checks holds global check configuration.
	Checks CheckConfig
	// ModeChecks holds per-mode check configurations.
	ModeChecks map[string]CheckConfig
	// Report holds report generation options.
	Report ReportConfig
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		Report: ReportConfig{TodoList: true},
	}
}

// GetCheckConfig returns the merged check config for a specific mode.
func (c Config) GetCheckConfig(mode string) CheckConfig {
	global := c.Checks
	modeCfg, ok := c.ModeChecks[mode]
	if !ok {
		return global
	}

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
	// Checks holds global check configuration.
	Checks yamlCheckConfig `yaml:"checks"`
	// Report holds report generation options.
	Report yamlReportConfig `yaml:"report"`
	// Scan holds scan-mode check configuration.
	Scan *yamlModeConfig `yaml:"scan"`
	// Audit holds audit-mode check configuration.
	Audit *yamlModeConfig `yaml:"audit"`
	// Analyze holds analyze-mode check configuration.
	Analyze *yamlModeConfig `yaml:"analyze"`
	// Monitor holds monitor-mode check configuration.
	Monitor *yamlModeConfig `yaml:"monitor"`
}

type yamlCheckConfig struct {
	// Exclude lists check names to skip.
	Exclude []string `yaml:"exclude"`
	// IncludeOnly lists check names to run exclusively.
	IncludeOnly []string `yaml:"include_only"`
}

type yamlReportConfig struct {
	// TodoList controls whether the To Do list is included.
	TodoList *bool `yaml:"todo_list"`
	// TodoIncludeConsider includes CONSIDER items in the To Do list.
	TodoIncludeConsider *bool `yaml:"todo_include_consider"`
}

type yamlModeConfig struct {
	// Checks holds global check configuration.
	Checks yamlCheckConfig `yaml:"checks"`
}

func (y yamlConfig) toConfig() Config {
	cfg := Default()

	cfg.Checks.Exclude = y.Checks.Exclude
	if y.Checks.IncludeOnly != nil {
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
			if mc.Checks.IncludeOnly != nil {
				cc.IncludeOnly = mc.Checks.IncludeOnly
			}
			cfg.ModeChecks[mode] = cc
		}
	}

	return cfg
}
