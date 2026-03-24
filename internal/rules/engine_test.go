package rules

import (
	"fmt"
	"reflect"
	"testing"

	scanconfig "github.com/belyaev-dev/helmdoc/internal/config"
	"github.com/belyaev-dev/helmdoc/pkg/models"
	helmchart "helm.sh/helm/v3/pkg/chart"
)

type stubRule struct {
	id       string
	category models.Category
	severity models.Severity
	title    string
	findings []models.Finding
	seen     []AnalysisContext
}

func (r *stubRule) ID() string                { return r.id }
func (r *stubRule) Category() models.Category { return r.category }
func (r *stubRule) Severity() models.Severity { return r.severity }
func (r *stubRule) Title() string             { return r.title }
func (r *stubRule) Check(ctx AnalysisContext) []models.Finding {
	r.seen = append(r.seen, ctx)
	return append([]models.Finding(nil), r.findings...)
}

func TestRunAllUsesRegisteredRules(t *testing.T) {
	savedRegistry := append([]Rule(nil), registry...)
	registry = nil
	t.Cleanup(func() {
		registry = savedRegistry
	})

	chartRef := &helmchart.Chart{Metadata: &helmchart.Metadata{Name: "demo", Version: "0.1.0"}}
	rendered := map[string][]models.K8sResource{
		"templates/workload.yaml": {{Kind: "Deployment", Name: "demo", Raw: map[string]any{}}},
	}
	values := models.NewValuesSurface(map[string]models.ValuePath{
		"controller.resources": {Type: "object", Default: map[string]any{}},
	})
	ctx := AnalysisContext{Chart: chartRef, RenderedResources: rendered, ValuesSurface: values}

	resourcesRule := &stubRule{
		id:       "RES999",
		category: models.CategoryResources,
		severity: models.SeverityWarning,
		title:    "Resources missing",
		findings: []models.Finding{{Description: "missing requests", Path: "templates/resources.yaml", Resource: "Deployment/demo"}},
	}
	securityRule := &stubRule{
		id:       "SEC999",
		category: models.CategorySecurity,
		severity: models.SeverityCritical,
		title:    "Security context missing",
		findings: []models.Finding{{Description: "container lacks hardening", Path: "templates/security.yaml", Resource: "Deployment/demo"}},
	}

	Register(resourcesRule)
	Register(securityRule)

	got := RunAll(ctx)
	want := []models.Finding{
		{
			RuleID:      "SEC999",
			Category:    models.CategorySecurity,
			Severity:    models.SeverityCritical,
			Title:       "Security context missing",
			Description: "container lacks hardening",
			Path:        "templates/security.yaml",
			Resource:    "Deployment/demo",
		},
		{
			RuleID:      "RES999",
			Category:    models.CategoryResources,
			Severity:    models.SeverityWarning,
			Title:       "Resources missing",
			Description: "missing requests",
			Path:        "templates/resources.yaml",
			Resource:    "Deployment/demo",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RunAll() = %#v, want %#v", got, want)
	}

	if gotRules := All(); len(gotRules) != 2 || gotRules[0].ID() != "SEC999" || gotRules[1].ID() != "RES999" {
		t.Fatalf("All() order = [%v %v], want [SEC999 RES999]", ruleIDAt(gotRules, 0), ruleIDAt(gotRules, 1))
	}
	if gotRules := ByCategory(models.CategorySecurity); len(gotRules) != 1 || gotRules[0].ID() != "SEC999" {
		t.Fatalf("ByCategory(security) = %#v, want SEC999 only", gotRules)
	}

	for _, rule := range []*stubRule{securityRule, resourcesRule} {
		if len(rule.seen) != 1 {
			t.Fatalf("rule %s saw %d contexts, want 1", rule.ID(), len(rule.seen))
		}
		if rule.seen[0].Chart != chartRef {
			t.Fatalf("rule %s Chart pointer mismatch", rule.ID())
		}
		if !reflect.DeepEqual(rule.seen[0].RenderedResources, rendered) {
			t.Fatalf("rule %s RenderedResources mismatch", rule.ID())
		}
		if !reflect.DeepEqual(rule.seen[0].ValuesSurface.AllPaths(), values.AllPaths()) {
			t.Fatalf("rule %s ValuesSurface mismatch", rule.ID())
		}
	}
}

func TestRunAllWithConfig(t *testing.T) {
	savedRegistry := append([]Rule(nil), registry...)
	registry = nil
	t.Cleanup(func() {
		registry = savedRegistry
	})

	ctx := AnalysisContext{Chart: &helmchart.Chart{Metadata: &helmchart.Metadata{Name: "demo"}}}

	securityRule := &stubRule{
		id:       "SEC999",
		category: models.CategorySecurity,
		severity: models.SeverityError,
		title:    "Security finding",
		findings: []models.Finding{{Description: "security issue", Path: "templates/security.yaml", Resource: "Deployment/demo"}},
	}
	resourcesRule := &stubRule{
		id:       "RES999",
		category: models.CategoryResources,
		severity: models.SeverityWarning,
		title:    "Resources finding",
		findings: []models.Finding{{Description: "resources issue", Path: "templates/resources.yaml", Resource: "Deployment/demo"}},
	}
	healthRule := &stubRule{
		id:       "HLT999",
		category: models.CategoryHealth,
		severity: models.SeverityWarning,
		title:    "Health finding",
		findings: []models.Finding{{Description: "health issue", Path: "templates/health.yaml", Resource: "Deployment/demo"}},
	}

	Register(resourcesRule)
	Register(healthRule)
	Register(securityRule)

	policy := &scanconfig.Config{
		Rules: map[string]scanconfig.RuleConfig{
			"SEC999": {Severity: stringPtr(models.SeverityCritical.String())},
			"HLT999": {Enabled: boolPtr(false)},
		},
		Categories: map[string]scanconfig.CategoryConfig{
			string(models.CategoryResources): {Enabled: boolPtr(false)},
		},
	}

	got := RunAllWithConfig(ctx, policy)
	want := []models.Finding{{
		RuleID:      "SEC999",
		Category:    models.CategorySecurity,
		Severity:    models.SeverityCritical,
		Title:       "Security finding",
		Description: "security issue",
		Path:        "templates/security.yaml",
		Resource:    "Deployment/demo",
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("RunAllWithConfig() = %#v, want %#v", got, want)
	}

	if len(securityRule.seen) != 1 {
		t.Fatalf("security rule executions = %d, want 1", len(securityRule.seen))
	}
	if len(resourcesRule.seen) != 0 {
		t.Fatalf("resources rule executions = %d, want 0 because category is disabled", len(resourcesRule.seen))
	}
	if len(healthRule.seen) != 0 {
		t.Fatalf("health rule executions = %d, want 0 because rule is disabled", len(healthRule.seen))
	}
}

func TestIterateWorkloadContainersIncludesInitContainersAndCronJobs(t *testing.T) {
	rendered := map[string][]models.K8sResource{
		"templates/z-job.yaml": {
			makeWorkloadResource("Job", "admission-create", nil, []string{"create"}),
		},
		"templates/a-deployment.yaml": {
			makeWorkloadResource("Deployment", "controller", []string{"copy-config"}, []string{"controller"}),
			{Kind: "ConfigMap", Name: "ignored", Raw: map[string]any{}},
		},
		"templates/m-statefulset.yaml": {
			makeWorkloadResource("StatefulSet", "stateful", nil, []string{"stateful"}),
		},
		"templates/n-daemonset.yaml": {
			makeWorkloadResource("DaemonSet", "agents", []string{"install"}, []string{"agent"}),
		},
		"templates/b-cronjob.yaml": {
			makeWorkloadResource("CronJob", "cleanup", []string{"prepare"}, []string{"cleanup"}),
		},
	}

	var got []string
	IterateWorkloadContainers(rendered, func(container WorkloadContainer) bool {
		if container.Container["name"] != container.Name {
			t.Fatalf("container name mismatch: map=%v struct=%q", container.Container["name"], container.Name)
		}
		got = append(got, fmt.Sprintf("%s|%s|init=%t|name=%s", container.TemplatePath, workloadResourceIdentity(container.Resource), container.IsInit, container.Name))
		return true
	})

	want := []string{
		"templates/a-deployment.yaml|Deployment/controller|init=true|name=copy-config",
		"templates/a-deployment.yaml|Deployment/controller|init=false|name=controller",
		"templates/b-cronjob.yaml|CronJob/cleanup|init=true|name=prepare",
		"templates/b-cronjob.yaml|CronJob/cleanup|init=false|name=cleanup",
		"templates/m-statefulset.yaml|StatefulSet/stateful|init=false|name=stateful",
		"templates/n-daemonset.yaml|DaemonSet/agents|init=true|name=install",
		"templates/n-daemonset.yaml|DaemonSet/agents|init=false|name=agent",
		"templates/z-job.yaml|Job/admission-create|init=false|name=create",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("IterateWorkloadContainers() = %#v, want %#v", got, want)
	}
}

func makeWorkloadResource(kind, name string, initContainers, containers []string) models.K8sResource {
	podSpec := map[string]any{}
	if len(initContainers) > 0 {
		podSpec["initContainers"] = namedContainers(initContainers)
	}
	if len(containers) > 0 {
		podSpec["containers"] = namedContainers(containers)
	}

	raw := map[string]any{
		"apiVersion": "v1",
		"kind":       kind,
		"metadata": map[string]any{
			"name": name,
		},
	}

	switch kind {
	case "CronJob":
		raw["spec"] = map[string]any{
			"jobTemplate": map[string]any{
				"spec": map[string]any{
					"template": map[string]any{
						"spec": podSpec,
					},
				},
			},
		}
	default:
		raw["spec"] = map[string]any{
			"template": map[string]any{
				"spec": podSpec,
			},
		}
	}

	return models.K8sResource{
		APIVersion: "apps/v1",
		Kind:       kind,
		Name:       name,
		Raw:        raw,
	}
}

func namedContainers(names []string) []any {
	containers := make([]any, 0, len(names))
	for _, name := range names {
		containers = append(containers, map[string]any{"name": name})
	}
	return containers
}

func ruleIDAt(rules []Rule, index int) string {
	if index < 0 || index >= len(rules) {
		return "<missing>"
	}
	return rules[index].ID()
}

func boolPtr(value bool) *bool {
	return &value
}

func stringPtr(value string) *string {
	return &value
}
