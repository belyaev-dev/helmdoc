package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"unicode"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&HardcodedEndpointRule{})
}

// HardcodedEndpointRule flags literal env values that hardcode service endpoints.
type HardcodedEndpointRule struct{}

func (r *HardcodedEndpointRule) ID() string                { return "CFG001" }
func (r *HardcodedEndpointRule) Category() models.Category { return models.CategoryConfig }
func (r *HardcodedEndpointRule) Severity() models.Severity { return models.SeverityWarning }
func (r *HardcodedEndpointRule) Title() string {
	return "Container environment variable hardcodes an endpoint"
}

func (r *HardcodedEndpointRule) Check(ctx rules.AnalysisContext) []models.Finding {
	findings := make([]models.Finding, 0)

	rules.IterateWorkloadContainers(ctx.RenderedResources, func(container rules.WorkloadContainer) bool {
		envEntries, ok := containerEnvEntries(container)
		if !ok {
			return true
		}

		resourceIdentity := configContainerResourceIdentity(container.Resource)
		containerLabel := configContainerDisplayName(container)
		for _, env := range envEntries {
			name, _ := env["name"].(string)
			value, valueOK := env["value"].(string)
			if !valueOK || strings.TrimSpace(value) == "" {
				continue
			}
			if _, hasValueFrom := env["valueFrom"]; hasValueFrom {
				continue
			}

			endpointKind, endpointValue, matched := detectEndpointLiteral(value)
			if !matched {
				continue
			}

			findings = append(findings, models.Finding{
				Description: fmt.Sprintf("%s in %s sets env %q to hardcoded %s %q.", containerLabel, resourceIdentity, name, endpointKind, endpointValue),
				Remediation: fmt.Sprintf("Move env %q for %s to a configurable values.yaml path or a non-literal reference instead of hardcoding the endpoint in the rendered manifest.", name, containerLabel),
				Path:        container.TemplatePath,
				Resource:    resourceIdentity,
			})
		}

		return true
	})

	return findings
}

func containerEnvEntries(container rules.WorkloadContainer) ([]map[string]any, bool) {
	rawEnv, ok := container.Container["env"].([]any)
	if !ok {
		return nil, false
	}

	entries := make([]map[string]any, 0, len(rawEnv))
	for _, item := range rawEnv {
		entry, ok := normalizeConfigMap(item)
		if !ok {
			continue
		}
		entries = append(entries, entry)
	}
	return entries, len(entries) > 0
}

func normalizeConfigMap(value any) (map[string]any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return typed, true
	case map[any]any:
		converted := make(map[string]any, len(typed))
		for key, nestedValue := range typed {
			keyString, ok := key.(string)
			if !ok {
				return nil, false
			}
			converted[keyString] = nestedValue
		}
		return converted, true
	default:
		return nil, false
	}
}

func detectEndpointLiteral(value string) (kind string, matchedValue string, matched bool) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", false
	}
	if strings.Contains(trimmed, "${") || strings.Contains(trimmed, "$(") {
		return "", "", false
	}

	if parsed, err := url.Parse(trimmed); err == nil && parsed.Scheme != "" && parsed.Host != "" {
		host := parsed.Hostname()
		if isEndpointHost(host, true) || net.ParseIP(host) != nil {
			return "URL", trimmed, true
		}
	}

	if ip := net.ParseIP(trimmed); ip != nil {
		return "IP endpoint", trimmed, true
	}

	if host, ok := splitEndpointHostPort(trimmed); ok {
		if net.ParseIP(host) != nil || isEndpointHost(host, true) {
			return "endpoint", trimmed, true
		}
	}

	if !strings.Contains(trimmed, "/") && isEndpointHost(trimmed, false) {
		return "hostname endpoint", trimmed, true
	}

	return "", "", false
}

func splitEndpointHostPort(value string) (string, bool) {
	if strings.Contains(value, "://") {
		return "", false
	}
	if strings.HasPrefix(value, "[") {
		host, port, err := net.SplitHostPort(value)
		if err != nil || port == "" {
			return "", false
		}
		if _, err := strconv.Atoi(port); err != nil {
			return "", false
		}
		return strings.Trim(host, "[]"), true
	}

	lastColon := strings.LastIndex(value, ":")
	if lastColon <= 0 || lastColon == len(value)-1 {
		return "", false
	}
	if strings.Contains(value[lastColon+1:], ":") {
		return "", false
	}
	if _, err := strconv.Atoi(value[lastColon+1:]); err != nil {
		return "", false
	}
	return value[:lastColon], true
}

func isEndpointHost(host string, allowSingleLabel bool) bool {
	host = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
	if host == "" || strings.ContainsAny(host, "/@") {
		return false
	}
	if host == "localhost" {
		return true
	}

	labels := strings.Split(host, ".")
	if len(labels) == 1 && !allowSingleLabel {
		return false
	}
	if len(labels) > 1 && !labelHasLetter(labels[len(labels)-1]) {
		return false
	}

	for _, label := range labels {
		if !validHostnameLabel(label) {
			return false
		}
	}

	if len(labels) == 1 {
		return labelHasLetter(labels[0])
	}
	return true
}

func validHostnameLabel(label string) bool {
	if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
		return false
	}
	for _, r := range label {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' {
			continue
		}
		return false
	}
	return true
}

func labelHasLetter(label string) bool {
	for _, r := range label {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func configContainerDisplayName(container rules.WorkloadContainer) string {
	kind := "container"
	if container.IsInit {
		kind = "init container"
	}
	if container.Name == "" {
		return kind
	}
	return fmt.Sprintf("%s %q", kind, container.Name)
}

func configContainerResourceIdentity(resource models.K8sResource) string {
	if resource.Name == "" {
		return resource.Kind
	}
	return fmt.Sprintf("%s/%s", resource.Kind, resource.Name)
}
