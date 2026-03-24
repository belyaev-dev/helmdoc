package rules

import (
	"fmt"
	"sort"

	"github.com/belyaev-dev/helmdoc/pkg/models"
)

// WorkloadContainer describes one rendered container inside a supported workload.
type WorkloadContainer struct {
	TemplatePath string
	Resource     models.K8sResource
	Container    map[string]any
	Name         string
	IsInit       bool
}

// IterateWorkloadContainers walks supported workload pod specs in stable template/resource/container order.
func IterateWorkloadContainers(rendered map[string][]models.K8sResource, visit func(container WorkloadContainer) bool) {
	if len(rendered) == 0 || visit == nil {
		return
	}

	templatePaths := make([]string, 0, len(rendered))
	for templatePath := range rendered {
		templatePaths = append(templatePaths, templatePath)
	}
	sort.Strings(templatePaths)

	for _, templatePath := range templatePaths {
		for _, resource := range rendered[templatePath] {
			podSpecPath, ok := workloadPodSpecPath(resource.Kind)
			if !ok {
				continue
			}

			podSpec, ok := resource.GetNestedMap(podSpecPath...)
			if !ok {
				continue
			}

			if !iteratePodSpecContainers(templatePath, resource, podSpec, "initContainers", true, visit) {
				return
			}
			if !iteratePodSpecContainers(templatePath, resource, podSpec, "containers", false, visit) {
				return
			}
		}
	}
}

func workloadPodSpecPath(kind string) ([]string, bool) {
	switch kind {
	case "Deployment", "StatefulSet", "DaemonSet", "Job":
		return []string{"spec", "template", "spec"}, true
	case "CronJob":
		return []string{"spec", "jobTemplate", "spec", "template", "spec"}, true
	default:
		return nil, false
	}
}

func iteratePodSpecContainers(templatePath string, resource models.K8sResource, podSpec map[string]any, field string, isInit bool, visit func(container WorkloadContainer) bool) bool {
	rawContainers, ok := podSpec[field].([]any)
	if !ok {
		return true
	}

	for _, rawContainer := range rawContainers {
		container, ok := normalizeStringMap(rawContainer)
		if !ok {
			continue
		}

		name, _ := container["name"].(string)
		if !visit(WorkloadContainer{
			TemplatePath: templatePath,
			Resource:     resource,
			Container:    container,
			Name:         name,
			IsInit:       isInit,
		}) {
			return false
		}
	}

	return true
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

func workloadResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
