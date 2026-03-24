package images

import (
	"fmt"
	"strings"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&TagRule{})
}

// TagRule flags workload containers that omit an explicit image tag or use latest.
type TagRule struct{}

func (r *TagRule) ID() string                { return "IMG001" }
func (r *TagRule) Category() models.Category { return models.CategoryImages }
func (r *TagRule) Severity() models.Severity { return models.SeverityWarning }
func (r *TagRule) Title() string             { return "Container image tag is missing or mutable" }

func (r *TagRule) Check(ctx rules.AnalysisContext) []models.Finding {
	findings := make([]models.Finding, 0)

	rules.IterateWorkloadContainers(ctx.RenderedResources, func(container rules.WorkloadContainer) bool {
		image, ok := imageString(container)
		if !ok {
			return true
		}

		ref := parseImageReference(image)
		resourceIdentity := imageContainerResourceIdentity(container.Resource)
		containerLabel := imageContainerDisplayName(container)

		switch {
		case ref.Tag == "":
			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s in %s uses image %q without an explicit tag.", containerLabel, resourceIdentity, image),
				Remediation: fmt.Sprintf("Pin %s to a specific non-latest image tag in the template or values.yaml.", containerLabel),
				Path:        container.TemplatePath,
				Resource:    resourceIdentity,
			})
		case strings.EqualFold(ref.Tag, "latest"):
			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s in %s uses image %q with the mutable \"latest\" tag.", containerLabel, resourceIdentity, image),
				Remediation: fmt.Sprintf("Replace the \"latest\" tag for %s with a specific immutable version tag.", containerLabel),
				Path:        container.TemplatePath,
				Resource:    resourceIdentity,
			})
		}

		return true
	})

	return findings
}

type imageReference struct {
	Tag    string
	Digest string
}

func parseImageReference(value string) imageReference {
	ref := imageReference{}
	image := strings.TrimSpace(value)
	if image == "" {
		return ref
	}

	withoutDigest := image
	if atIndex := strings.LastIndex(withoutDigest, "@"); atIndex >= 0 {
		ref.Digest = withoutDigest[atIndex+1:]
		withoutDigest = withoutDigest[:atIndex]
	}

	lastSlash := strings.LastIndex(withoutDigest, "/")
	lastColon := strings.LastIndex(withoutDigest, ":")
	if lastColon > lastSlash {
		ref.Tag = withoutDigest[lastColon+1:]
	}

	return ref
}

func imageString(container rules.WorkloadContainer) (string, bool) {
	image, ok := container.Container["image"].(string)
	image = strings.TrimSpace(image)
	return image, ok && image != ""
}

func imageContainerDisplayName(container rules.WorkloadContainer) string {
	kind := "container"
	if container.IsInit {
		kind = "init container"
	}
	if container.Name == "" {
		return kind
	}
	return fmt.Sprintf("%s %q", kind, container.Name)
}

func imageContainerResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
