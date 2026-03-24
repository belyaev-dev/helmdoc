// Package rules defines the rule engine interface and registration.
package rules

import (
	"helm.sh/helm/v3/pkg/chart"

	"github.com/belyaev-dev/helmdoc/pkg/models"
)

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

	// Check evaluates the chart and returns findings (empty = pass).
	Check(c *chart.Chart) []models.Finding
}

// registry holds all registered rules.
var registry []Rule

// Register adds a rule to the global registry.
func Register(r Rule) {
	registry = append(registry, r)
}

// All returns all registered rules.
func All() []Rule {
	return registry
}

// ByCategory returns rules filtered by category.
func ByCategory(cat models.Category) []Rule {
	var result []Rule
	for _, r := range registry {
		if r.Category() == cat {
			result = append(result, r)
		}
	}
	return result
}
