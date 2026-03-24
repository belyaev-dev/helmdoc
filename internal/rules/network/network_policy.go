package network

import (
	"fmt"
	"sort"
	"strings"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&NetworkPolicyRule{})
}

// NetworkPolicyRule flags long-lived workloads whose namespace renders no NetworkPolicy.
type NetworkPolicyRule struct{}

func (r *NetworkPolicyRule) ID() string                { return "NET001" }
func (r *NetworkPolicyRule) Category() models.Category { return models.CategoryNetwork }
func (r *NetworkPolicyRule) Severity() models.Severity { return models.SeverityWarning }
func (r *NetworkPolicyRule) Title() string             { return "Workload namespace has no NetworkPolicy" }

func (r *NetworkPolicyRule) Check(ctx rules.AnalysisContext) []models.Finding {
	namespacesWithPolicy := renderedPolicyNamespaces(ctx.RenderedResources)
	exposesNetworkPolicy := networkValuesSurfaceExposesPolicy(ctx.ValuesSurface)
	findings := make([]models.Finding, 0)

	for _, templatePath := range networkSortedTemplatePaths(ctx.RenderedResources) {
		for _, resource := range ctx.RenderedResources[templatePath] {
			if !supportsNetworkRule(resource.Kind) {
				continue
			}
			if namespacesWithPolicy[resource.Namespace] {
				continue
			}

			resourceIdentity := networkResourceIdentity(resource)
			remediation := fmt.Sprintf("Add a NetworkPolicy manifest for %s so %s is covered.", networkNamespaceDisplay(resource.Namespace), resourceIdentity)
			if exposesNetworkPolicy {
				remediation = fmt.Sprintf("Enable a chart networkPolicy setting in values.yaml so %s renders at least one NetworkPolicy for %s.", networkNamespaceDisplay(resource.Namespace), resourceIdentity)
			}

			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s is rendered in %s, but no NetworkPolicy is rendered for that namespace.", resourceIdentity, networkNamespaceDisplay(resource.Namespace)),
				Remediation: remediation,
				Path:        templatePath,
				Resource:    resourceIdentity,
			})
		}
	}

	return findings
}

func renderedPolicyNamespaces(rendered map[string][]models.K8sResource) map[string]bool {
	namespaces := make(map[string]bool)
	for _, templatePath := range networkSortedTemplatePaths(rendered) {
		for _, resource := range rendered[templatePath] {
			if resource.Kind == "NetworkPolicy" {
				namespaces[resource.Namespace] = true
			}
		}
	}
	return namespaces
}

func supportsNetworkRule(kind string) bool {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet":
		return true
	default:
		return false
	}
}

func networkValuesSurfaceExposesPolicy(surface *models.ValuesSurface) bool {
	if surface == nil {
		return false
	}

	for _, candidate := range surface.AllPaths() {
		if candidate == "networkPolicy" || candidate == "networkPolicy.enabled" {
			return true
		}
		if strings.HasSuffix(candidate, ".networkPolicy") || strings.HasSuffix(candidate, ".networkPolicy.enabled") {
			return true
		}
	}
	return false
}

func networkSortedTemplatePaths(rendered map[string][]models.K8sResource) []string {
	paths := make([]string, 0, len(rendered))
	for path := range rendered {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func networkNamespaceDisplay(namespace string) string {
	if namespace == "" {
		return "the release namespace"
	}
	return fmt.Sprintf("namespace %q", namespace)
}

func networkResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
