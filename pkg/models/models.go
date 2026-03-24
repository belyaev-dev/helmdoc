// Package models defines the core data types for helmdoc.
package models

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Severity represents the severity of a rule violation.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityError
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityInfo:
		return "info"
	case SeverityWarning:
		return "warning"
	case SeverityError:
		return "error"
	case SeverityCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// MarshalText renders the severity as its stable string form.
func (s Severity) MarshalText() ([]byte, error) {
	switch s {
	case SeverityInfo, SeverityWarning, SeverityError, SeverityCritical:
		return []byte(s.String()), nil
	default:
		return nil, fmt.Errorf("unknown severity: %d", s)
	}
}

// UnmarshalText parses the stable string form of a severity.
func (s *Severity) UnmarshalText(text []byte) error {
	if s == nil {
		return fmt.Errorf("cannot unmarshal severity into nil receiver")
	}

	switch strings.ToLower(strings.TrimSpace(string(text))) {
	case SeverityInfo.String():
		*s = SeverityInfo
	case SeverityWarning.String():
		*s = SeverityWarning
	case SeverityError.String():
		*s = SeverityError
	case SeverityCritical.String():
		*s = SeverityCritical
	default:
		return fmt.Errorf("unknown severity: %q", string(text))
	}

	return nil
}

// MarshalJSON renders the severity as a JSON string.
func (s Severity) MarshalJSON() ([]byte, error) {
	text, err := s.MarshalText()
	if err != nil {
		return nil, err
	}
	return json.Marshal(string(text))
}

// UnmarshalJSON parses a JSON string severity.
func (s *Severity) UnmarshalJSON(data []byte) error {
	var raw string
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	return s.UnmarshalText([]byte(raw))
}

// Category represents a check category.
type Category string

const (
	CategoryStorage      Category = "storage"
	CategoryResources    Category = "resources"
	CategorySecurity     Category = "security"
	CategoryHealth       Category = "health"
	CategoryImages       Category = "images"
	CategoryAvailability Category = "availability"
	CategoryNetwork      Category = "network"
	CategoryIngress      Category = "ingress"
	CategoryScaling      Category = "scaling"
	CategoryConfig       Category = "config"
)

// AllCategories returns all check categories in display order.
func AllCategories() []Category {
	return []Category{
		CategorySecurity,
		CategoryResources,
		CategoryStorage,
		CategoryHealth,
		CategoryAvailability,
		CategoryNetwork,
		CategoryImages,
		CategoryIngress,
		CategoryScaling,
		CategoryConfig,
	}
}

// Grade represents a letter grade A-F.
type Grade string

const (
	GradeA Grade = "A"
	GradeB Grade = "B"
	GradeC Grade = "C"
	GradeD Grade = "D"
	GradeF Grade = "F"
)

// GradeFromScore converts a 0-100 score to a letter grade.
func GradeFromScore(score float64) Grade {
	switch {
	case score >= 90:
		return GradeA
	case score >= 80:
		return GradeB
	case score >= 70:
		return GradeC
	case score >= 60:
		return GradeD
	default:
		return GradeF
	}
}

// Rank returns the semantic order used by min-score comparisons.
func (g Grade) Rank() int {
	switch strings.ToUpper(strings.TrimSpace(string(g))) {
	case string(GradeA):
		return 4
	case string(GradeB):
		return 3
	case string(GradeC):
		return 2
	case string(GradeD):
		return 1
	case string(GradeF):
		return 0
	default:
		return -1
	}
}

// Finding represents a single rule violation found during analysis.
type Finding struct {
	RuleID      string   `json:"rule_id"`
	Category    Category `json:"category"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Remediation string   `json:"remediation"`
	Path        string   `json:"path,omitempty"`     // template file path
	Resource    string   `json:"resource,omitempty"` // e.g. "Deployment/nginx"
}

// CategoryScore holds the score for a single category.
type CategoryScore struct {
	Category Category  `json:"category"`
	Score    float64   `json:"score"`
	Grade    Grade     `json:"grade"`
	Weight   float64   `json:"weight"`
	Findings []Finding `json:"findings"`
}

// Report is the complete scan result for a chart.
type Report struct {
	ChartName     string          `json:"chart_name"`
	ChartVersion  string          `json:"chart_version"`
	OverallScore  float64         `json:"overall_score"`
	OverallGrade  Grade           `json:"overall_grade"`
	Categories    []CategoryScore `json:"categories"`
	TotalFindings int             `json:"total_findings"`
}
