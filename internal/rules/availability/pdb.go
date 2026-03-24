package availability

import (
	"fmt"
	"sort"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&PodDisruptionBudgetRule{})
}

// PodDisruptionBudgetRule flags supported workloads that have no matching rendered PDB.
type PodDisruptionBudgetRule struct{}

func (r *PodDisruptionBudgetRule) ID() string                { return "AVL001" }
func (r *PodDisruptionBudgetRule) Category() models.Category { return models.CategoryAvailability }
func (r *PodDisruptionBudgetRule) Severity() models.Severity { return models.SeverityWarning }
func (r *PodDisruptionBudgetRule) Title() string {
	return "Workload has no matching PodDisruptionBudget"
}

func (r *PodDisruptionBudgetRule) Check(ctx rules.AnalysisContext) []models.Finding {
	selectorsByNamespace := collectPDBSelectors(ctx.RenderedResources)
	findings := make([]models.Finding, 0)

	for _, templatePath := range availabilitySortedTemplatePaths(ctx.RenderedResources) {
		for _, resource := range ctx.RenderedResources[templatePath] {
			if !supportsAvailabilityRule(resource.Kind) {
				continue
			}
			if workloadHasMatchingPDB(resource, selectorsByNamespace[resource.Namespace]) {
				continue
			}

			resourceIdentity := availabilityResourceIdentity(resource)
			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s has no matching PodDisruptionBudget in %s.", resourceIdentity, availabilityNamespaceDisplay(resource.Namespace)),
				Remediation: fmt.Sprintf("Render a PodDisruptionBudget whose selector matches %s in %s.", resourceIdentity, availabilityNamespaceDisplay(resource.Namespace)),
				Path:        templatePath,
				Resource:    resourceIdentity,
			})
		}
	}

	return findings
}

type pdbSelector struct {
	matchLabels map[string]string
	selectsAll  bool
}

func collectPDBSelectors(rendered map[string][]models.K8sResource) map[string][]pdbSelector {
	selectors := make(map[string][]pdbSelector)
	for _, templatePath := range availabilitySortedTemplatePaths(rendered) {
		for _, resource := range rendered[templatePath] {
			if resource.Kind != "PodDisruptionBudget" {
				continue
			}

			selector := pdbSelector{}
			selectorMap, ok := resource.GetNestedMap("spec", "selector")
			if ok {
				matchLabels := availabilityStringMap(selectorMap["matchLabels"])
				if len(matchLabels) > 0 {
					selector.matchLabels = matchLabels
				} else {
					selector.selectsAll = true
				}
			} else {
				selector.selectsAll = true
			}

			selectors[resource.Namespace] = append(selectors[resource.Namespace], selector)
		}
	}
	return selectors
}

func workloadHasMatchingPDB(resource models.K8sResource, selectors []pdbSelector) bool {
	if len(selectors) == 0 {
		return false
	}

	labels := workloadPodTemplateLabels(resource)
	for _, selector := range selectors {
		if selector.selectsAll {
			return true
		}
		if labelsMatch(selector.matchLabels, labels) {
			return true
		}
	}
	return false
}

func supportsAvailabilityRule(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet":
		return true
	default:
		return false
	}
}

func workloadPodTemplateLabels(resource models.K8sResource) map[string]string {
	labels, ok := resource.GetNestedMap("spec", "template", "metadata", "labels")
	if !ok {
		return nil
	}
	return availabilityStringMap(labels)
}

func labelsMatch(selector map[string]string, labels map[string]string) bool {
	if len(selector) == 0 || len(labels) == 0 {
		return false
	}
	for key, value := range selector {
		if labels[key] != value {
			return false
		}
	}
	return true
}

func availabilityStringMap(value any) map[string]string {
	raw, ok := value.(map[string]any)
	if !ok {
		return nil
	}
	result := make(map[string]string, len(raw))
	for key, nested := range raw {
		stringValue, ok := nested.(string)
		if !ok {
			return nil
		}
		result[key] = stringValue
	}
	return result
}

func availabilitySortedTemplatePaths(rendered map[string][]models.K8sResource) []string {
	paths := make([]string, 0, len(rendered))
	for path := range rendered {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func availabilityNamespaceDisplay(namespace string) string {
	if namespace == "" {
		return "the release namespace"
	}
	return fmt.Sprintf("namespace %q", namespace)
}

func availabilityResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
