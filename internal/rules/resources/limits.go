package resources

import (
	"fmt"
	"strings"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&LimitsRule{})
}

// LimitsRule flags workload containers that do not define resource limits.
type LimitsRule struct{}

func (r *LimitsRule) ID() string                { return "RES001" }
func (r *LimitsRule) Category() models.Category { return models.CategoryResources }
func (r *LimitsRule) Severity() models.Severity { return models.SeverityWarning }
func (r *LimitsRule) Title() string             { return "Container resource limits are missing" }

func (r *LimitsRule) Check(ctx rules.AnalysisContext) []models.Finding {
	return checkMissingResourceRequirement(ctx, "limits")
}

func checkMissingResourceRequirement(ctx rules.AnalysisContext, requirement string) []models.Finding {
	findings := make([]models.Finding, 0)

	rules.IterateWorkloadContainers(ctx.RenderedResources, func(container rules.WorkloadContainer) bool {
		if containerHasResourceRequirement(container, requirement) {
			return true
		}

		resourceIdentity := resourcesContainerResourceIdentity(container.Resource)
		containerLabel := resourcesContainerDisplayName(container)
		findings = append(findings, models.Finding{
			Description: fmt.Sprintf("%s in %s does not define resource %s.", containerLabel, resourceIdentity, requirement),
			Remediation: resourceRequirementRemediation(ctx.ValuesSurface, container, requirement),
			Path:        container.TemplatePath,
			Resource:    resourceIdentity,
		})
		return true
	})

	return findings
}

func containerHasResourceRequirement(container rules.WorkloadContainer, requirement string) bool {
	resources, ok := resourcesContainerView(container).GetNestedMap("resources", requirement)
	return ok && len(resources) > 0
}

func resourcesContainerView(container rules.WorkloadContainer) models.K8sResource {
	return models.K8sResource{Raw: container.Container}
}

func resourceRequirementRemediation(surface *models.ValuesSurface, container rules.WorkloadContainer, requirement string) string {
	basePath := resourceValuesBasePath(container)
	if basePath == "" {
		return fmt.Sprintf("Add resources.%s directly to %s in the template, or expose a values.yaml path for it.", requirement, resourcesContainerDisplayName(container))
	}

	resourcesPath := basePath + ".resources"
	requirementPath := resourcesPath + "." + requirement
	if valuesSurfaceExposes(surface, resourcesPath) || valuesSurfaceExposes(surface, requirementPath) {
		return fmt.Sprintf("Set %s in values.yaml. The chart already exposes %s, but its default leaves %s empty.", requirementPath, resourcesPath, requirement)
	}

	return fmt.Sprintf("Expose %s in values.yaml or add resources.%s directly to the template, because this chart does not currently expose that knob.", resourcesPath, requirement)
}

func resourceValuesBasePath(container rules.WorkloadContainer) string {
	switch container.TemplatePath {
	case "templates/controller-deployment.yaml", "templates/controller-daemonset.yaml":
		return "controller"
	case "templates/default-backend-deployment.yaml":
		return "defaultBackend"
	case "templates/admission-webhooks/job-patch/job-createSecret.yaml":
		return "controller.admissionWebhooks.createSecretJob"
	case "templates/admission-webhooks/job-patch/job-patchWebhook.yaml":
		return "controller.admissionWebhooks.patchWebhookJob"
	default:
		return ""
	}
}

func valuesSurfaceExposes(surface *models.ValuesSurface, path string) bool {
	if surface == nil || path == "" {
		return false
	}
	if surface.HasPath(path) {
		return true
	}

	prefix := path + "."
	for _, candidate := range surface.AllPaths() {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}
	return false
}

func resourcesContainerDisplayName(container rules.WorkloadContainer) string {
	kind := "container"
	if container.IsInit {
		kind = "init container"
	}
	if container.Name == "" {
		return kind
	}
	return fmt.Sprintf("%s %q", kind, container.Name)
}

func resourcesContainerResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
