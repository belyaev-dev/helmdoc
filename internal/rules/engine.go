// Package rules defines the rule engine interface and registration.
package rules

import (
	"sort"

	scanconfig "github.com/belyaev-dev/helmdoc/internal/config"
	"github.com/belyaev-dev/helmdoc/pkg/models"
	helmchart "helm.sh/helm/v3/pkg/chart"
)

// AnalysisContext is the complete input surface available to every rule.
type AnalysisContext struct {
	Chart             *helmchart.Chart
	RenderedResources map[string][]models.K8sResource
	ValuesSurface     *models.ValuesSurface
}

// Rule is the interface all checks must implement.
type Rule interface {
	// ID returns the unique rule identifier (e.g. "SEC001").
	ID() string

	// Category returns which category this rule belongs to.
	Category() models.Category

	// Severity returns the default severity of this rule.
	Severity() models.Severity

	// Title returns a short human-readable title.
	Title() string

	// Check evaluates the analysis context and returns findings (empty = pass).
	Check(ctx AnalysisContext) []models.Finding
}

// registry holds all registered rules.
var registry []Rule

// Register adds a rule to the global registry.
func Register(r Rule) {
	if r == nil {
		return
	}
	registry = append(registry, r)
}

// All returns all registered rules in stable category/id order.
func All() []Rule {
	if len(registry) == 0 {
		return nil
	}

	rules := append([]Rule(nil), registry...)
	sort.SliceStable(rules, func(i, j int) bool {
		left := rules[i]
		right := rules[j]

		leftRank := categoryRank(left.Category())
		rightRank := categoryRank(right.Category())
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if left.ID() != right.ID() {
			return left.ID() < right.ID()
		}
		return left.Title() < right.Title()
	})

	return rules
}

// ByCategory returns rules filtered by category.
func ByCategory(cat models.Category) []Rule {
	allRules := All()
	if len(allRules) == 0 {
		return nil
	}

	result := make([]Rule, 0, len(allRules))
	for _, r := range allRules {
		if r.Category() == cat {
			result = append(result, r)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// RunAll executes every registered rule against the provided analysis context.
func RunAll(ctx AnalysisContext) []models.Finding {
	return RunAllWithConfig(ctx, nil)
}

// RunAllWithConfig executes registered rules while honoring config-driven policy.
func RunAllWithConfig(ctx AnalysisContext, cfg *scanconfig.Config) []models.Finding {
	allRules := All()
	if len(allRules) == 0 {
		return nil
	}

	findings := make([]models.Finding, 0)
	for _, rule := range allRules {
		if cfg != nil && !cfg.RuleEnabled(rule.ID(), rule.Category()) {
			continue
		}

		effectiveSeverity := rule.Severity()
		if cfg != nil {
			effectiveSeverity = cfg.EffectiveSeverity(rule.ID(), effectiveSeverity)
		}

		ruleFindings := rule.Check(ctx)
		for _, finding := range ruleFindings {
			finding.RuleID = rule.ID()
			finding.Category = rule.Category()
			finding.Severity = effectiveSeverity
			if finding.Title == "" {
				finding.Title = rule.Title()
			}
			findings = append(findings, finding)
		}
	}

	sortFindings(findings)
	return findings
}

func sortFindings(findings []models.Finding) {
	sort.SliceStable(findings, func(i, j int) bool {
		left := findings[i]
		right := findings[j]

		leftRank := categoryRank(left.Category)
		rightRank := categoryRank(right.Category)
		if leftRank != rightRank {
			return leftRank < rightRank
		}
		if left.RuleID != right.RuleID {
			return left.RuleID < right.RuleID
		}
		if left.Path != right.Path {
			return left.Path < right.Path
		}
		if left.Resource != right.Resource {
			return left.Resource < right.Resource
		}
		if left.Title != right.Title {
			return left.Title < right.Title
		}
		if left.Description != right.Description {
			return left.Description < right.Description
		}
		return left.Remediation < right.Remediation
	})
}

func categoryRank(category models.Category) int {
	for index, candidate := range models.AllCategories() {
		if candidate == category {
			return index
		}
	}
	return len(models.AllCategories())
}
