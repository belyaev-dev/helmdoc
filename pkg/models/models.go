// Package models defines the core data types for helmdoc.
package models

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
	GradeA  Grade = "A"
	GradeB  Grade = "B"
	GradeC  Grade = "C"
	GradeD  Grade = "D"
	GradeF  Grade = "F"
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

// Finding represents a single rule violation found during analysis.
type Finding struct {
	RuleID      string   `json:"rule_id"`
	Category    Category `json:"category"`
	Severity    Severity `json:"severity"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Remediation string   `json:"remediation"`
	Path        string   `json:"path,omitempty"`   // template file path
	Resource    string   `json:"resource,omitempty"` // e.g. "Deployment/nginx"
}

// CategoryScore holds the score for a single category.
type CategoryScore struct {
	Category  Category  `json:"category"`
	Score     float64   `json:"score"`
	Grade     Grade     `json:"grade"`
	Passed    int       `json:"passed"`
	Failed    int       `json:"failed"`
	Findings  []Finding `json:"findings"`
}

// Report is the complete scan result for a chart.
type Report struct {
	ChartName      string          `json:"chart_name"`
	ChartVersion   string          `json:"chart_version"`
	OverallScore   float64         `json:"overall_score"`
	OverallGrade   Grade           `json:"overall_grade"`
	Categories     []CategoryScore `json:"categories"`
	TotalFindings  int             `json:"total_findings"`
}
