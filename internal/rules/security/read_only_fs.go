package security

import (
	"fmt"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&ReadOnlyRootFilesystemRule{})
}

// ReadOnlyRootFilesystemRule flags workload containers whose root filesystem is writable.
type ReadOnlyRootFilesystemRule struct{}

func (r *ReadOnlyRootFilesystemRule) ID() string                { return "SEC003" }
func (r *ReadOnlyRootFilesystemRule) Category() models.Category { return models.CategorySecurity }
func (r *ReadOnlyRootFilesystemRule) Severity() models.Severity { return models.SeverityError }
func (r *ReadOnlyRootFilesystemRule) Title() string             { return "Container root filesystem is writable" }

func (r *ReadOnlyRootFilesystemRule) Check(ctx rules.AnalysisContext) []models.Finding {
	findings := make([]models.Finding, 0)

	rules.IterateWorkloadContainers(ctx.RenderedResources, func(container rules.WorkloadContainer) bool {
		readOnlyRootFilesystem, ok := containerView(container).GetNestedBool("securityContext", "readOnlyRootFilesystem")
		if ok && readOnlyRootFilesystem {
			return true
		}

		resourceIdentity := containerResourceIdentity(container.Resource)
		containerLabel := containerDisplayName(container)
		findings = append(findings, models.Finding{
			Description: fmt.Sprintf("%s in %s does not set readOnlyRootFilesystem: true.", containerLabel, resourceIdentity),
			Remediation: fmt.Sprintf("Set %s securityContext.readOnlyRootFilesystem to true.", containerLabel),
			Path:        container.TemplatePath,
			Resource:    resourceIdentity,
		})
		return true
	})

	return findings
}
