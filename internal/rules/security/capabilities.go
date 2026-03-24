package security

import (
	"fmt"
	"strings"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&CapabilitiesRule{})
}

// CapabilitiesRule flags workload containers that do not drop all Linux capabilities.
type CapabilitiesRule struct{}

func (r *CapabilitiesRule) ID() string                { return "SEC002" }
func (r *CapabilitiesRule) Category() models.Category { return models.CategorySecurity }
func (r *CapabilitiesRule) Severity() models.Severity { return models.SeverityError }
func (r *CapabilitiesRule) Title() string             { return "Container does not drop ALL capabilities" }

func (r *CapabilitiesRule) Check(ctx rules.AnalysisContext) []models.Finding {
	findings := make([]models.Finding, 0)

	rules.IterateWorkloadContainers(ctx.RenderedResources, func(container rules.WorkloadContainer) bool {
		dropValues, ok := containerView(container).GetNestedSlice("securityContext", "capabilities", "drop")
		if ok && sliceContainsString(dropValues, "ALL") {
			return true
		}

		resourceIdentity := containerResourceIdentity(container.Resource)
		containerLabel := containerDisplayName(container)
		findings = append(findings, models.Finding{
			Description: fmt.Sprintf("%s in %s does not drop all Linux capabilities.", containerLabel, resourceIdentity),
			Remediation: fmt.Sprintf("Set %s securityContext.capabilities.drop to include \"ALL\".", containerLabel),
			Path:        container.TemplatePath,
			Resource:    resourceIdentity,
		})
		return true
	})

	return findings
}

func sliceContainsString(values []any, want string) bool {
	for _, value := range values {
		if strings.EqualFold(fmt.Sprint(value), want) {
			return true
		}
	}
	return false
}
