package security

import (
	"fmt"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&SecurityContextRule{})
}

// SecurityContextRule flags containers without a meaningful securityContext or with explicitly unsafe settings.
type SecurityContextRule struct{}

func (r *SecurityContextRule) ID() string                { return "SEC001" }
func (r *SecurityContextRule) Category() models.Category { return models.CategorySecurity }
func (r *SecurityContextRule) Severity() models.Severity { return models.SeverityError }
func (r *SecurityContextRule) Title() string {
	return "Container security context is missing or unsafe"
}

func (r *SecurityContextRule) Check(ctx rules.AnalysisContext) []models.Finding {
	findings := make([]models.Finding, 0)

	rules.IterateWorkloadContainers(ctx.RenderedResources, func(container rules.WorkloadContainer) bool {
		resourceIdentity := containerResourceIdentity(container.Resource)
		containerLabel := containerDisplayName(container)
		securityContext, ok := containerView(container).GetNestedMap("securityContext")
		switch {
		case !ok || len(securityContext) == 0:
			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s in %s has no effective securityContext.", containerLabel, resourceIdentity),
				Remediation: fmt.Sprintf("Set %s securityContext explicitly and ensure allowPrivilegeEscalation is false.", containerLabel),
				Path:        container.TemplatePath,
				Resource:    resourceIdentity,
			})
		case containerAllowsPrivilegeEscalation(container):
			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s in %s explicitly sets allowPrivilegeEscalation: true.", containerLabel, resourceIdentity),
				Remediation: fmt.Sprintf("Set %s securityContext.allowPrivilegeEscalation to false.", containerLabel),
				Path:        container.TemplatePath,
				Resource:    resourceIdentity,
			})
		}
		return true
	})

	return findings
}

func containerView(container rules.WorkloadContainer) models.K8sResource {
	return models.K8sResource{Raw: container.Container}
}

func containerAllowsPrivilegeEscalation(container rules.WorkloadContainer) bool {
	allowPrivilegeEscalation, ok := containerView(container).GetNestedBool("securityContext", "allowPrivilegeEscalation")
	return ok && allowPrivilegeEscalation
}

func containerDisplayName(container rules.WorkloadContainer) string {
	kind := "container"
	if container.IsInit {
		kind = "init container"
	}
	if container.Name == "" {
		return kind
	}
	return fmt.Sprintf("%s %q", kind, container.Name)
}

func containerResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
