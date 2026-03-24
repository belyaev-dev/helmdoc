package scaling

import (
	"fmt"
	"sort"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&HPARule{})
}

// HPARule flags supported workloads that have no matching rendered HPA.
type HPARule struct{}

func (r *HPARule) ID() string                { return "SCL001" }
func (r *HPARule) Category() models.Category { return models.CategoryScaling }
func (r *HPARule) Severity() models.Severity { return models.SeverityWarning }
func (r *HPARule) Title() string             { return "Workload has no matching HorizontalPodAutoscaler" }

func (r *HPARule) Check(ctx rules.AnalysisContext) []models.Finding {
	targetsByNamespace := collectHPATargets(ctx.RenderedResources)
	findings := make([]models.Finding, 0)

	for _, templatePath := range scalingSortedTemplatePaths(ctx.RenderedResources) {
		for _, resource := range ctx.RenderedResources[templatePath] {
			if !supportsScalingRule(resource.Kind) {
				continue
			}
			if workloadHasMatchingHPA(resource, targetsByNamespace[resource.Namespace]) {
				continue
			}

			resourceIdentity := scalingResourceIdentity(resource)
			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s has no matching HorizontalPodAutoscaler in %s.", resourceIdentity, scalingNamespaceDisplay(resource.Namespace)),
				Remediation: fmt.Sprintf("Render a HorizontalPodAutoscaler with scaleTargetRef matching %s in %s.", resourceIdentity, scalingNamespaceDisplay(resource.Namespace)),
				Path:        templatePath,
				Resource:    resourceIdentity,
			})
		}
	}

	return findings
}

type hpaTarget struct {
	kind string
	name string
}

func collectHPATargets(rendered map[string][]models.K8sResource) map[string][]hpaTarget {
	targets := make(map[string][]hpaTarget)
	for _, templatePath := range scalingSortedTemplatePaths(rendered) {
		for _, resource := range rendered[templatePath] {
			if resource.Kind != "HorizontalPodAutoscaler" {
				continue
			}

			ref, ok := resource.GetNestedMap("spec", "scaleTargetRef")
			if !ok {
				continue
			}
			kind, _ := ref["kind"].(string)
			name, _ := ref["name"].(string)
			if kind == "" || name == "" {
				continue
			}

			targets[resource.Namespace] = append(targets[resource.Namespace], hpaTarget{kind: kind, name: name})
		}
	}
	return targets
}

func workloadHasMatchingHPA(resource models.K8sResource, targets []hpaTarget) bool {
	for _, target := range targets {
		if target.kind == resource.Kind && target.name == resource.Name {
			return true
		}
	}
	return false
}

func supportsScalingRule(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet":
		return true
	default:
		return false
	}
}

func scalingSortedTemplatePaths(rendered map[string][]models.K8sResource) []string {
	paths := make([]string, 0, len(rendered))
	for path := range rendered {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func scalingNamespaceDisplay(namespace string) string {
	if namespace == "" {
		return "the release namespace"
	}
	return fmt.Sprintf("namespace %q", namespace)
}

func scalingResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
