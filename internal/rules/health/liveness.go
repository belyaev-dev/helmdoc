package health

import (
	"fmt"
	"strings"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&LivenessProbeRule{})
}

// LivenessProbeRule flags workload containers that do not define a liveness probe.
type LivenessProbeRule struct{}

func (r *LivenessProbeRule) ID() string                { return "HLT001" }
func (r *LivenessProbeRule) Category() models.Category { return models.CategoryHealth }
func (r *LivenessProbeRule) Severity() models.Severity { return models.SeverityWarning }
func (r *LivenessProbeRule) Title() string             { return "Container liveness probe is missing" }

func (r *LivenessProbeRule) Check(ctx rules.AnalysisContext) []models.Finding {
	return checkMissingProbe(ctx, "livenessProbe")
}

func checkMissingProbe(ctx rules.AnalysisContext, probeField string) []models.Finding {
	findings := make([]models.Finding, 0)

	rules.IterateWorkloadContainers(ctx.RenderedResources, func(container rules.WorkloadContainer) bool {
		if container.IsInit || containerHasProbe(container, probeField) {
			return true
		}

		resourceIdentity := healthContainerResourceIdentity(container.Resource)
		containerLabel := healthContainerDisplayName(container)
		findings = append(findings, models.Finding{
			Description: fmt.Sprintf("%s in %s has no %s.", containerLabel, resourceIdentity, probeField),
			Remediation: probeRemediation(ctx.ValuesSurface, container, probeField),
			Path:        container.TemplatePath,
			Resource:    resourceIdentity,
		})
		return true
	})

	return findings
}

func containerHasProbe(container rules.WorkloadContainer, probeField string) bool {
	probe, ok := healthContainerView(container).GetNestedMap(probeField)
	return ok && len(probe) > 0
}

func healthContainerView(container rules.WorkloadContainer) models.K8sResource {
	return models.K8sResource{Raw: container.Container}
}

func probeRemediation(surface *models.ValuesSurface, container rules.WorkloadContainer, probeField string) string {
	basePath := healthValuesBasePath(container)
	if basePath == "" {
		return fmt.Sprintf("Add %s directly to %s in the template.", probeField, healthContainerDisplayName(container))
	}

	valuesPath := basePath + "." + probeField
	if healthValuesSurfaceExposes(surface, valuesPath) {
		return fmt.Sprintf("Set %s in values.yaml so %s gets a %s.", valuesPath, healthContainerDisplayName(container), probeField)
	}

	return fmt.Sprintf("Add %s directly to the template for %s or expose %s in values.yaml.", probeField, healthContainerDisplayName(container), valuesPath)
}

func healthValuesBasePath(container rules.WorkloadContainer) string {
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

func healthValuesSurfaceExposes(surface *models.ValuesSurface, path string) bool {
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

func healthContainerDisplayName(container rules.WorkloadContainer) string {
	if container.Name == "" {
		return "container"
	}
	return fmt.Sprintf("container %q", container.Name)
}

func healthContainerResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
