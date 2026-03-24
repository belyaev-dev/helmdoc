// Package config loads and validates .helmdoc.yaml policy files.
package config

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/belyaev-dev/helmdoc/pkg/models"
	"sigs.k8s.io/yaml"
)

// Config contains scan policy overrides loaded from .helmdoc.yaml.
type Config struct {
	Rules      map[string]RuleConfig     `yaml:"rules"`
	Categories map[string]CategoryConfig `yaml:"categories"`
}

// RuleConfig configures a single rule by ID.
type RuleConfig struct {
	Enabled  *bool   `yaml:"enabled"`
	Severity *string `yaml:"severity"`
}

// CategoryConfig configures a whole rule category.
type CategoryConfig struct {
	Enabled *bool `yaml:"enabled"`
}

// LoadConfig loads a policy file from disk. Missing files behave like an empty config.
func LoadConfig(path string) (*Config, error) {
	if strings.TrimSpace(path) == "" {
		return &Config{}, nil
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}

	if len(bytes.TrimSpace(contents)) == 0 {
		return &Config{}, nil
	}

	var cfg Config
	if err := yaml.UnmarshalStrict(contents, &cfg); err != nil {
		return nil, fmt.Errorf("load config %q: %w", path, err)
	}
	if err := cfg.validate(path); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// RuleEnabled reports whether the given rule should run.
func (c *Config) RuleEnabled(ruleID string, category models.Category) bool {
	if c == nil {
		return true
	}

	if categoryConfig, ok := c.Categories[string(category)]; ok && categoryConfig.Enabled != nil && !*categoryConfig.Enabled {
		return false
	}
	if ruleConfig, ok := c.Rules[ruleID]; ok && ruleConfig.Enabled != nil {
		return *ruleConfig.Enabled
	}

	return true
}

// EffectiveSeverity returns the configured severity override or the default.
func (c *Config) EffectiveSeverity(ruleID string, defaultSeverity models.Severity) models.Severity {
	if c == nil {
		return defaultSeverity
	}

	ruleConfig, ok := c.Rules[ruleID]
	if !ok || ruleConfig.Severity == nil {
		return defaultSeverity
	}

	severity, ok := parseSeverity(*ruleConfig.Severity)
	if !ok {
		return defaultSeverity
	}
	return severity
}

func (c *Config) validate(path string) error {
	for category := range c.Categories {
		if !isKnownCategory(category) {
			return fmt.Errorf("load config %q: invalid categories.%s: unknown category", path, category)
		}
	}

	for ruleID, ruleConfig := range c.Rules {
		if ruleConfig.Severity == nil {
			continue
		}
		if _, ok := parseSeverity(*ruleConfig.Severity); !ok {
			return fmt.Errorf(
				"load config %q: invalid rules.%s.severity %q (want one of %s)",
				path,
				ruleID,
				*ruleConfig.Severity,
				strings.Join(validSeverityNames(), ", "),
			)
		}
	}

	return nil
}

func parseSeverity(raw string) (models.Severity, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case models.SeverityInfo.String():
		return models.SeverityInfo, true
	case models.SeverityWarning.String():
		return models.SeverityWarning, true
	case models.SeverityError.String():
		return models.SeverityError, true
	case models.SeverityCritical.String():
		return models.SeverityCritical, true
	default:
		return 0, false
	}
}

func validSeverityNames() []string {
	names := []string{
		models.SeverityInfo.String(),
		models.SeverityWarning.String(),
		models.SeverityError.String(),
		models.SeverityCritical.String(),
	}
	sort.Strings(names)
	return names
}

func isKnownCategory(category string) bool {
	for _, candidate := range models.AllCategories() {
		if category == string(candidate) {
			return true
		}
	}
	return false
}
