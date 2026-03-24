package images

import (
	"fmt"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&DigestRule{})
}

// DigestRule flags workload containers whose images are not pinned by digest.
type DigestRule struct{}

func (r *DigestRule) ID() string                { return "IMG002" }
func (r *DigestRule) Category() models.Category { return models.CategoryImages }
func (r *DigestRule) Severity() models.Severity { return models.SeverityWarning }
func (r *DigestRule) Title() string             { return "Container image is not pinned by digest" }

func (r *DigestRule) Check(ctx rules.AnalysisContext) []models.Finding {
	findings := make([]models.Finding, 0)

	rules.IterateWorkloadContainers(ctx.RenderedResources, func(container rules.WorkloadContainer) bool {
		image, ok := imageString(container)
		if !ok {
			return true
		}

		ref := parseImageReference(image)
		if ref.Digest != "" {
			return true
		}

		resourceIdentity := imageContainerResourceIdentity(container.Resource)
		containerLabel := imageContainerDisplayName(container)
		findings = append(findings, models.Finding{
			Description: fmt.Sprintf("%s in %s uses image %q without a pinned digest.", containerLabel, resourceIdentity, image),
			Remediation: fmt.Sprintf("Pin %s to an immutable digest (for example @sha256:...) in the template or values.yaml.", containerLabel),
			Path:        container.TemplatePath,
			Resource:    resourceIdentity,
		})
		return true
	})

	return findings
}
