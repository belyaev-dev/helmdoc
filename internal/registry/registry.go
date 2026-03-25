package registry

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/belyaev-dev/helmdoc/pkg/models"
	"sigs.k8s.io/yaml"
)

// Registry holds curated chart routing knowledge loaded from chart-local YAML files.
type Registry struct {
	charts     map[string]Chart
	chartNames []string
}

// Chart describes one curated chart file and its rule/template mappings.
type Chart struct {
	Name       string    `json:"chart" yaml:"chart"`
	Mappings   []Mapping `json:"mappings" yaml:"mappings"`
	SourceFile string    `json:"-" yaml:"-"`

	mappingIndex map[string]int `json:"-" yaml:"-"`
}

// Mapping groups ordered candidate routes for a single rule/template pair.
type Mapping struct {
	RuleID       string      `json:"rule" yaml:"rule"`
	TemplatePath string      `json:"template" yaml:"template"`
	Candidates   []Candidate `json:"candidates" yaml:"candidates"`
}

// Candidate is one chart-authored route candidate. Exactly one of ValuesPath or
// ValuesBase must be set.
type Candidate struct {
	ValuesPath          string `json:"values_path,omitempty" yaml:"values_path,omitempty"`
	ValuesBase          string `json:"values_base,omitempty" yaml:"values_base,omitempty"`
	Summary             string `json:"summary" yaml:"summary"`
	RequiresRelatedRule string `json:"requires_related_rule,omitempty" yaml:"requires_related_rule,omitempty"`
}

// LoadFromFS loads all curated chart files from charts/*.yaml in the provided filesystem.
func LoadFromFS(fsys fs.FS) (*Registry, error) {
	files, err := fs.Glob(fsys, "charts/*.yaml")
	if err != nil {
		return nil, fmt.Errorf("glob charts/*.yaml: %w", err)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("glob charts/*.yaml: no chart registry files found")
	}
	sort.Strings(files)

	registry := &Registry{
		charts:     make(map[string]Chart, len(files)),
		chartNames: make([]string, 0, len(files)),
	}

	for _, file := range files {
		data, err := fs.ReadFile(fsys, file)
		if err != nil {
			return nil, fmt.Errorf("load %s: read file: %w", file, err)
		}

		var chart Chart
		if err := yaml.Unmarshal(data, &chart); err != nil {
			return nil, fmt.Errorf("load %s: decode yaml: %w", file, err)
		}
		if err := chart.validate(file); err != nil {
			return nil, err
		}

		if existing, ok := registry.charts[chart.Name]; ok {
			return nil, fmt.Errorf("load %s: chart %q already registered by %s", file, chart.Name, existing.SourceFile)
		}

		registry.charts[chart.Name] = chart
		registry.chartNames = append(registry.chartNames, chart.Name)
	}

	sort.Strings(registry.chartNames)
	return registry, nil
}

// ChartNames returns the curated chart names in stable order.
func (r *Registry) ChartNames() []string {
	if r == nil || len(r.chartNames) == 0 {
		return nil
	}
	return append([]string(nil), r.chartNames...)
}

// LookupChart returns a chart by name.
func (r *Registry) LookupChart(chartName string) (Chart, bool) {
	if r == nil {
		return Chart{}, false
	}
	chart, ok := r.charts[strings.TrimSpace(chartName)]
	return chart, ok
}

// LookupCandidatesForFinding returns the ordered candidate list for a finding after
// applying any candidate-level related-rule predicates.
func (r *Registry) LookupCandidatesForFinding(chartName string, findings []models.Finding, finding models.Finding) []Candidate {
	chart, ok := r.LookupChart(chartName)
	if !ok {
		return nil
	}

	mapping, ok := chart.lookupMapping(finding.RuleID, finding.Path)
	if !ok {
		return nil
	}

	candidates := make([]Candidate, 0, len(mapping.Candidates))
	for _, candidate := range mapping.Candidates {
		if !candidate.matches(findings, finding) {
			continue
		}
		candidates = append(candidates, candidate)
	}
	if len(candidates) == 0 {
		return nil
	}
	return candidates
}

func (c *Chart) validate(sourceFile string) error {
	c.Name = strings.TrimSpace(c.Name)
	c.SourceFile = sourceFile
	if c.Name == "" {
		return fmt.Errorf("load %s: chart name is required", sourceFile)
	}
	if len(c.Mappings) == 0 {
		return fmt.Errorf("load %s: chart %q must define at least one mapping", sourceFile, c.Name)
	}

	c.mappingIndex = make(map[string]int, len(c.Mappings))
	for i := range c.Mappings {
		mapping := &c.Mappings[i]
		if err := mapping.validate(sourceFile); err != nil {
			return err
		}

		key := mapping.lookupKey()
		if _, exists := c.mappingIndex[key]; exists {
			return fmt.Errorf("load %s: duplicate mapping %s", sourceFile, mapping.displayKey())
		}
		c.mappingIndex[key] = i
	}

	return nil
}

func (c Chart) lookupMapping(ruleID, templatePath string) (Mapping, bool) {
	if len(c.mappingIndex) == 0 {
		return Mapping{}, false
	}
	index, ok := c.mappingIndex[mappingLookupKey(ruleID, templatePath)]
	if !ok {
		return Mapping{}, false
	}
	return c.Mappings[index], true
}

func (m *Mapping) validate(sourceFile string) error {
	m.RuleID = strings.TrimSpace(m.RuleID)
	m.TemplatePath = strings.TrimSpace(m.TemplatePath)
	if m.RuleID == "" || m.TemplatePath == "" {
		return fmt.Errorf("load %s: mapping must include both rule and template", sourceFile)
	}
	if len(m.Candidates) == 0 {
		return fmt.Errorf("load %s: mapping %s must define at least one candidate", sourceFile, m.displayKey())
	}

	for i := range m.Candidates {
		if err := m.Candidates[i].validate(sourceFile, m.displayKey(), i+1); err != nil {
			return err
		}
	}

	return nil
}

func (m Mapping) lookupKey() string {
	return mappingLookupKey(m.RuleID, m.TemplatePath)
}

func (m Mapping) displayKey() string {
	return fmt.Sprintf("%s @ %s", m.RuleID, m.TemplatePath)
}

func mappingLookupKey(ruleID, templatePath string) string {
	return strings.TrimSpace(ruleID) + "|" + strings.TrimSpace(templatePath)
}

func (c *Candidate) validate(sourceFile, mappingKey string, ordinal int) error {
	c.ValuesPath = strings.TrimSpace(c.ValuesPath)
	c.ValuesBase = strings.TrimSpace(c.ValuesBase)
	c.Summary = strings.TrimSpace(c.Summary)
	c.RequiresRelatedRule = strings.TrimSpace(c.RequiresRelatedRule)

	if c.Summary == "" {
		return fmt.Errorf("load %s: mapping %s candidate %d must include a summary", sourceFile, mappingKey, ordinal)
	}

	hasPath := c.ValuesPath != ""
	hasBase := c.ValuesBase != ""
	if hasPath == hasBase {
		return fmt.Errorf("load %s: mapping %s candidate %d must set exactly one of values_path or values_base", sourceFile, mappingKey, ordinal)
	}

	return nil
}

func (c Candidate) matches(findings []models.Finding, finding models.Finding) bool {
	requiredRule := strings.TrimSpace(c.RequiresRelatedRule)
	if requiredRule == "" {
		return true
	}

	for _, related := range findings {
		if related.RuleID == requiredRule && related.Path == finding.Path && related.Resource == finding.Resource {
			return true
		}
	}

	return false
}
