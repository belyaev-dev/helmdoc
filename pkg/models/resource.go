package models

import (
	"strconv"
)

// K8sResource is a rendered Kubernetes manifest plus helper accessors for rules.
type K8sResource struct {
	APIVersion string         `json:"api_version"`
	Kind       string         `json:"kind"`
	Name       string         `json:"name"`
	Namespace  string         `json:"namespace,omitempty"`
	Raw        map[string]any `json:"raw"`
}

// GetNested returns a nested value from Raw using map keys and optional numeric slice indexes.
func (r K8sResource) GetNested(path ...string) (any, bool) {
	if len(path) == 0 {
		if r.Raw == nil {
			return nil, false
		}
		return r.Raw, true
	}

	var current any = r.Raw
	for _, segment := range path {
		switch typed := current.(type) {
		case map[string]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			current = next
		case map[any]any:
			next, ok := typed[segment]
			if !ok {
				return nil, false
			}
			current = next
		case []any:
			index, err := strconv.Atoi(segment)
			if err != nil || index < 0 || index >= len(typed) {
				return nil, false
			}
			current = typed[index]
		default:
			return nil, false
		}
	}

	return current, true
}

// GetNestedString returns a nested string value.
func (r K8sResource) GetNestedString(path ...string) (string, bool) {
	value, ok := r.GetNested(path...)
	if !ok {
		return "", false
	}

	stringValue, ok := value.(string)
	if !ok {
		return "", false
	}

	return stringValue, true
}

// GetNestedBool returns a nested bool value.
func (r K8sResource) GetNestedBool(path ...string) (bool, bool) {
	value, ok := r.GetNested(path...)
	if !ok {
		return false, false
	}

	boolValue, ok := value.(bool)
	if !ok {
		return false, false
	}

	return boolValue, true
}

// GetNestedSlice returns a nested slice value.
func (r K8sResource) GetNestedSlice(path ...string) ([]any, bool) {
	value, ok := r.GetNested(path...)
	if !ok {
		return nil, false
	}

	sliceValue, ok := value.([]any)
	if !ok {
		return nil, false
	}

	return sliceValue, true
}

// GetNestedMap returns a nested map value.
func (r K8sResource) GetNestedMap(path ...string) (map[string]any, bool) {
	value, ok := r.GetNested(path...)
	if !ok {
		return nil, false
	}

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
