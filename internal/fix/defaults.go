package fix

import (
	"fmt"
	"strings"

	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func defaultPayloadForFinding(finding models.Finding, valuesPath string, surface *models.ValuesSurface) (any, error) {
	switch finding.RuleID {
	case "SEC003":
		return true, nil
	case "RES001":
		return defaultResourceRequirements(valuesPath, surface, "limits"), nil
	case "RES002":
		return defaultResourceRequirements(valuesPath, surface, "requests"), nil
	case "NET001":
		return true, nil
	case "AVL001":
		return defaultAvailabilityPayload(valuesPath, surface)
	case "SCL001":
		return defaultAutoscalingPayload(valuesPath, surface)
	default:
		return nil, fmt.Errorf("rule %s has no default payload builder", finding.RuleID)
	}
}

func defaultKustomizePatchForFinding(finding models.Finding, target ResourceRef, containerName string) (map[string]any, string, error) {
	if !supportsAdmissionWebhookProbePatchDefault(finding, target) {
		return nil, "", fmt.Errorf("rule %s has no supported Kustomize default for %s @ %s", finding.RuleID, finding.Resource, finding.Path)
	}
	if strings.TrimSpace(containerName) == "" {
		return nil, "", fmt.Errorf("rule %s requires a named target container for Kustomize patching", finding.RuleID)
	}

	var field string
	var summary string
	var payload map[string]any

	switch finding.RuleID {
	case "HLT001":
		field = "livenessProbe"
		summary = "Add a conservative livenessProbe directly to the rendered workload container."
		payload = defaultLivenessProbePayload()
	case "HLT002":
		field = "readinessProbe"
		summary = "Add a conservative readinessProbe directly to the rendered workload container."
		payload = defaultReadinessProbePayload()
	default:
		return nil, "", fmt.Errorf("rule %s has no Kustomize patch payload builder", finding.RuleID)
	}

	return containerScopedStrategicMergePatch(target, containerName, field, payload), summary, nil
}

func supportsAdmissionWebhookProbePatchDefault(finding models.Finding, target ResourceRef) bool {
	if target.Kind != "Job" {
		return false
	}
	switch finding.Path {
	case "templates/admission-webhooks/job-patch/job-createSecret.yaml", "templates/admission-webhooks/job-patch/job-patchWebhook.yaml":
		return finding.RuleID == "HLT001" || finding.RuleID == "HLT002"
	default:
		return false
	}
}

func defaultLivenessProbePayload() map[string]any {
	return map[string]any{
		"exec": map[string]any{
			"command": []any{"/kube-webhook-certgen", "--help"},
		},
		"initialDelaySeconds": 5,
		"periodSeconds":       10,
		"timeoutSeconds":      1,
		"successThreshold":    1,
		"failureThreshold":    3,
	}
}

func defaultReadinessProbePayload() map[string]any {
	return map[string]any{
		"exec": map[string]any{
			"command": []any{"/kube-webhook-certgen", "--help"},
		},
		"periodSeconds":    5,
		"timeoutSeconds":   1,
		"successThreshold": 1,
		"failureThreshold": 1,
	}
}

func containerScopedStrategicMergePatch(target ResourceRef, containerName, field string, payload map[string]any) map[string]any {
	metadata := map[string]any{"name": target.Name}
	if target.Namespace != "" {
		metadata["namespace"] = target.Namespace
	}

	return map[string]any{
		"apiVersion": target.APIVersion,
		"kind":       target.Kind,
		"metadata":   metadata,
		"spec": map[string]any{
			"template": map[string]any{
				"spec": map[string]any{
					"containers": []any{
						map[string]any{
							"name": containerName,
							field:  cloneValue(payload),
						},
					},
				},
			},
		},
	}
}

func defaultResourceRequirements(valuesPath string, surface *models.ValuesSurface, requirement string) map[string]any {
	basePath := parentValuesPath(valuesPath)
	if sibling, ok := nonEmptyMapDefault(surface, siblingResourceRequirementPath(basePath, requirement)); ok {
		return sibling
	}
	if existing, ok := nonEmptyMapDefault(surface, valuesPath); ok {
		return existing
	}
	if existing, ok := nonEmptyMapDefault(surface, basePath); ok {
		if nested, ok := nestedMap(existing, requirement); ok && len(nested) > 0 {
			return nested
		}
	}

	if strings.Contains(valuesPath, ".admissionWebhooks.") {
		return map[string]any{"cpu": "10m", "memory": "20Mi"}
	}
	return map[string]any{"cpu": "100m", "memory": "90Mi"}
}

func siblingResourceRequirementPath(basePath, requirement string) string {
	if requirement == "limits" {
		return basePath + ".requests"
	}
	return basePath + ".limits"
}

func defaultAvailabilityPayload(valuesPath string, surface *models.ValuesSurface) (any, error) {
	switch valuesPath {
	case "controller.autoscaling.minReplicas", "defaultBackend.autoscaling.minReplicas":
		defaultValue, _ := intDefault(surface, valuesPath)
		if defaultValue < 2 {
			defaultValue = 2
		}
		return defaultValue, nil
	case "controller.replicaCount", "defaultBackend.replicaCount":
		defaultValue, _ := intDefault(surface, valuesPath)
		if defaultValue < 2 {
			defaultValue = 2
		}
		return defaultValue, nil
	default:
		return nil, fmt.Errorf("availability payload does not know how to populate %s", valuesPath)
	}
}

func defaultAutoscalingPayload(valuesPath string, surface *models.ValuesSurface) (any, error) {
	basePath := valuesPath
	if strings.HasSuffix(basePath, ".enabled") {
		basePath = parentValuesPath(basePath)
	}
	if basePath == "" {
		return nil, fmt.Errorf("autoscaling payload requires a base path")
	}

	minReplicas, ok := intDefault(surface, basePath+".minReplicas")
	if !ok || minReplicas < 2 {
		minReplicas = 2
	}
	maxReplicas, ok := intDefault(surface, basePath+".maxReplicas")
	if !ok || maxReplicas <= minReplicas {
		maxReplicas = minReplicas + 1
	}
	cpuUtil, ok := intDefault(surface, basePath+".targetCPUUtilizationPercentage")
	if !ok || cpuUtil <= 0 {
		cpuUtil = 50
	}
	memoryUtil, ok := intDefault(surface, basePath+".targetMemoryUtilizationPercentage")
	if !ok || memoryUtil <= 0 {
		memoryUtil = 50
	}

	return map[string]any{
		"enabled":                           true,
		"minReplicas":                       minReplicas,
		"maxReplicas":                       maxReplicas,
		"targetCPUUtilizationPercentage":    cpuUtil,
		"targetMemoryUtilizationPercentage": memoryUtil,
	}, nil
}

func nonEmptyMapDefault(surface *models.ValuesSurface, path string) (map[string]any, bool) {
	if surface == nil || path == "" {
		return nil, false
	}
	value := surface.GetDefault(path)
	mapped, ok := value.(map[string]any)
	if !ok || len(mapped) == 0 {
		return nil, false
	}
	return cloneMap(mapped), true
}

func nestedMap(value map[string]any, key string) (map[string]any, bool) {
	nested, ok := value[key].(map[string]any)
	if !ok || len(nested) == 0 {
		return nil, false
	}
	return cloneMap(nested), true
}

func intDefault(surface *models.ValuesSurface, path string) (int, bool) {
	if surface == nil || path == "" {
		return 0, false
	}
	value := surface.GetDefault(path)
	switch typed := value.(type) {
	case int:
		return typed, true
	case int8:
		return int(typed), true
	case int16:
		return int(typed), true
	case int32:
		return int(typed), true
	case int64:
		return int(typed), true
	case uint:
		return int(typed), true
	case uint8:
		return int(typed), true
	case uint16:
		return int(typed), true
	case uint32:
		return int(typed), true
	case uint64:
		return int(typed), true
	case float32:
		return int(typed), true
	case float64:
		return int(typed), true
	default:
		return 0, false
	}
}

func cloneMap(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		switch typed := value.(type) {
		case map[string]any:
			out[key] = cloneMap(typed)
		case []any:
			copied := make([]any, len(typed))
			copy(copied, typed)
			out[key] = copied
		default:
			out[key] = typed
		}
	}
	return out
}
