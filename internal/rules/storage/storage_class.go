package storage

import (
	"fmt"
	"sort"
	"strings"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&StorageClassRule{})
}

// StorageClassRule flags rendered PVC surfaces that do not set a storage class and
// do not expose any storage-class values knob.
type StorageClassRule struct{}

func (r *StorageClassRule) ID() string                { return "STR001" }
func (r *StorageClassRule) Category() models.Category { return models.CategoryStorage }
func (r *StorageClassRule) Severity() models.Severity { return models.SeverityWarning }
func (r *StorageClassRule) Title() string             { return "Persistent storage class is not configurable" }

func (r *StorageClassRule) Check(ctx rules.AnalysisContext) []models.Finding {
	if storageValuesSurfaceExposesClass(ctx.ValuesSurface) {
		return nil
	}

	findings := make([]models.Finding, 0)
	for _, templatePath := range sortedTemplatePaths(ctx.RenderedResources) {
		for _, resource := range ctx.RenderedResources[templatePath] {
			switch resource.Kind {
			case "PersistentVolumeClaim":
				if pvcHasExplicitStorageClass(resource) {
					continue
				}
				resourceIdentity := storageResourceIdentity(resource)
				findings = append(findings, models.Finding{
					Description: fmt.Sprintf("%s does not set spec.storageClassName and the chart does not expose a storageClass values path.", resourceIdentity),
					Remediation: fmt.Sprintf("Expose a values.yaml path for storageClassName or set spec.storageClassName directly in the template for %s.", resourceIdentity),
					Path:        templatePath,
					Resource:    resourceIdentity,
				})
			case "StatefulSet":
				claimTemplates, ok := resource.GetNestedSlice("spec", "volumeClaimTemplates")
				if !ok {
					continue
				}
				for _, rawClaimTemplate := range claimTemplates {
					claimTemplate, ok := normalizeStringMap(rawClaimTemplate)
					if !ok {
						continue
					}

					claimResource := models.K8sResource{Raw: claimTemplate}
					if pvcHasExplicitStorageClass(claimResource) {
						continue
					}

					claimName, _ := claimResource.GetNestedString("metadata", "name")
					resourceIdentity := storageResourceIdentity(resource)
					claimLabel := fmt.Sprintf("volumeClaimTemplate in %s", resourceIdentity)
					if claimName != "" {
						claimLabel = fmt.Sprintf("volumeClaimTemplate %q in %s", claimName, resourceIdentity)
					}
					findings = append(findings, models.Finding{
						Description: fmt.Sprintf("%s does not set spec.storageClassName and the chart does not expose a storageClass values path.", claimLabel),
						Remediation: fmt.Sprintf("Expose a values.yaml path for storageClassName or set spec.storageClassName directly in the template for %s.", claimLabel),
						Path:        templatePath,
						Resource:    resourceIdentity,
					})
				}
			}
		}
	}

	return findings
}

func pvcHasExplicitStorageClass(resource models.K8sResource) bool {
	value, ok := resource.GetNested("spec", "storageClassName")
	return ok && value != nil
}

func storageValuesSurfaceExposesClass(surface *models.ValuesSurface) bool {
	if surface == nil {
		return false
	}

	for _, candidate := range surface.AllPaths() {
		if candidate == "storageClass" || candidate == "storageClassName" {
			return true
		}
		if strings.HasSuffix(candidate, ".storageClass") || strings.HasSuffix(candidate, ".storageClassName") {
			return true
		}
	}

	return false
}

func sortedTemplatePaths(rendered map[string][]models.K8sResource) []string {
	paths := make([]string, 0, len(rendered))
	for path := range rendered {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func normalizeStringMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		for key, nestedValue := range typed {
			keyString, ok := key.(string)
			if !ok {
				return nil, false
			}
			normalized[keyString] = nestedValue
		}
		return normalized, true
	default:
		return nil, false
	}
}

func storageResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
