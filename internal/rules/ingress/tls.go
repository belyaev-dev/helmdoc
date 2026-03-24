package ingress

import (
	"fmt"
	"sort"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&TLSRule{})
}

// TLSRule flags rendered Ingress resources that do not define TLS blocks.
type TLSRule struct{}

func (r *TLSRule) ID() string                { return "ING001" }
func (r *TLSRule) Category() models.Category { return models.CategoryIngress }
func (r *TLSRule) Severity() models.Severity { return models.SeverityWarning }
func (r *TLSRule) Title() string             { return "Ingress TLS is not configured" }

func (r *TLSRule) Check(ctx rules.AnalysisContext) []models.Finding {
	findings := make([]models.Finding, 0)
	for _, templatePath := range ingressSortedTemplatePaths(ctx.RenderedResources) {
		for _, resource := range ctx.RenderedResources[templatePath] {
			if resource.Kind != "Ingress" {
				continue
			}
			tlsEntries, ok := resource.GetNestedSlice("spec", "tls")
			if ok && len(tlsEntries) > 0 {
				continue
			}

			resourceIdentity := ingressResourceIdentity(resource)
			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s does not define spec.tls.", resourceIdentity),
				Remediation: fmt.Sprintf("Add spec.tls to %s or expose a values.yaml path that renders the TLS block.", resourceIdentity),
				Path:        templatePath,
				Resource:    resourceIdentity,
			})
		}
	}
	return findings
}

func ingressSortedTemplatePaths(rendered map[string][]models.K8sResource) []string {
	paths := make([]string, 0, len(rendered))
	for path := range rendered {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func ingressResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
