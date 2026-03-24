package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestLoadConfig(t *testing.T) {
	t.Run("missing_file_returns_empty_config", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), ".helmdoc.yaml")

		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig(%q) error = %v", configPath, err)
		}

		if cfg == nil {
			t.Fatal("LoadConfig() returned nil config")
		}
		if !cfg.RuleEnabled("SEC001", models.CategorySecurity) {
			t.Fatal("missing config disabled SEC001, want enabled")
		}
		if got := cfg.EffectiveSeverity("SEC001", models.SeverityWarning); got != models.SeverityWarning {
			t.Fatalf("EffectiveSeverity() = %v, want %v", got, models.SeverityWarning)
		}
	})

	t.Run("empty_file_returns_empty_config", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), ".helmdoc.yaml")
		writeConfigFile(t, configPath, "\n")

		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig(%q) error = %v", configPath, err)
		}

		if cfg == nil {
			t.Fatal("LoadConfig() returned nil config")
		}
		if len(cfg.Rules) != 0 {
			t.Fatalf("len(cfg.Rules) = %d, want 0", len(cfg.Rules))
		}
		if len(cfg.Categories) != 0 {
			t.Fatalf("len(cfg.Categories) = %d, want 0", len(cfg.Categories))
		}
	})

	t.Run("parses_rule_and_category_policy", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), ".helmdoc.yaml")
		writeConfigFile(t, configPath, strings.TrimSpace(`
rules:
  SEC001:
    enabled: false
  RES001:
    severity: error
categories:
  health:
    enabled: false
`))

		cfg, err := LoadConfig(configPath)
		if err != nil {
			t.Fatalf("LoadConfig(%q) error = %v", configPath, err)
		}

		if cfg.Rules["SEC001"].Enabled == nil || *cfg.Rules["SEC001"].Enabled {
			t.Fatalf("cfg.Rules[SEC001].Enabled = %#v, want explicit false", cfg.Rules["SEC001"].Enabled)
		}
		if cfg.Rules["RES001"].Enabled != nil {
			t.Fatalf("cfg.Rules[RES001].Enabled = %#v, want nil", cfg.Rules["RES001"].Enabled)
		}
		if cfg.Categories["health"].Enabled == nil || *cfg.Categories["health"].Enabled {
			t.Fatalf("cfg.Categories[health].Enabled = %#v, want explicit false", cfg.Categories["health"].Enabled)
		}
		if cfg.Rules["RES001"].Severity == nil || *cfg.Rules["RES001"].Severity != "error" {
			t.Fatalf("cfg.Rules[RES001].Severity = %#v, want error", cfg.Rules["RES001"].Severity)
		}
		if cfg.RuleEnabled("SEC001", models.CategorySecurity) {
			t.Fatal("RuleEnabled(SEC001) = true, want false")
		}
		if cfg.RuleEnabled("HLT001", models.CategoryHealth) {
			t.Fatal("RuleEnabled(HLT001, health) = true, want false")
		}
		if got := cfg.EffectiveSeverity("RES001", models.SeverityWarning); got != models.SeverityError {
			t.Fatalf("EffectiveSeverity(RES001) = %v, want %v", got, models.SeverityError)
		}
	})

	t.Run("invalid_severity_reports_file_path_and_field", func(t *testing.T) {
		configPath := filepath.Join(t.TempDir(), ".helmdoc.yaml")
		writeConfigFile(t, configPath, strings.TrimSpace(`
rules:
  SEC001:
    severity: urgent
`))

		_, err := LoadConfig(configPath)
		if err == nil {
			t.Fatal("LoadConfig() error = nil, want non-nil")
		}

		for _, want := range []string{configPath, "rules.SEC001.severity", "urgent"} {
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("error %q does not include %q", err, want)
			}
		}
	})
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("os.WriteFile(%q) error = %v", path, err)
	}
}
