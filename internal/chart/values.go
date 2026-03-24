// Package chart handles Helm chart loading and parsing.
package chart

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"

	"github.com/belyaev-dev/helmdoc/pkg/models"
	helmchart "helm.sh/helm/v3/pkg/chart"
)

// AnalyzeValues flattens chart default values into a rule-friendly dot-path surface.
func AnalyzeValues(c *helmchart.Chart) *models.ValuesSurface {
	surface := models.NewValuesSurface(nil)
	if c == nil || len(c.Values) == 0 {
		return surface
	}

	collectValuePaths(surface, "", c.Values)
	return surface
}

func collectValuePaths(surface *models.ValuesSurface, path string, value any) {
	if path != "" {
		surface.AddPath(path, value, valuesTypeName(value))
	}

	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		for _, key := range keys {
			collectValuePaths(surface, joinValuePath(path, key), typed[key])
		}
	case map[any]any:
		normalized := make(map[string]any, len(typed))
		keys := make([]string, 0, len(typed))
		for key, nested := range typed {
			stringKey := fmt.Sprint(key)
			normalized[stringKey] = nested
			keys = append(keys, stringKey)
		}
		sort.Strings(keys)

		for _, key := range keys {
			collectValuePaths(surface, joinValuePath(path, key), normalized[key])
		}
	case []any:
		for index, nested := range typed {
			collectValuePaths(surface, joinValuePath(path, strconv.Itoa(index)), nested)
		}
	default:
		valueOf := reflect.ValueOf(value)
		if !valueOf.IsValid() {
			return
		}

		switch valueOf.Kind() {
		case reflect.Slice, reflect.Array:
			for index := 0; index < valueOf.Len(); index++ {
				collectValuePaths(surface, joinValuePath(path, strconv.Itoa(index)), valueOf.Index(index).Interface())
			}
		}
	}
}

func joinValuePath(base, segment string) string {
	if base == "" {
		return segment
	}
	return base + "." + segment
}

func valuesTypeName(value any) string {
	switch value.(type) {
	case nil:
		return "null"
	case bool:
		return "bool"
	case string:
		return "string"
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return "number"
	case map[string]any, map[any]any:
		return "object"
	case []any:
		return "array"
	}

	valueOf := reflect.ValueOf(value)
	if !valueOf.IsValid() {
		return "null"
	}

	switch valueOf.Kind() {
	case reflect.Bool:
		return "bool"
	case reflect.String:
		return "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return "number"
	case reflect.Map:
		return "object"
	case reflect.Slice, reflect.Array:
		return "array"
	default:
		return valueOf.Type().String()
	}
}
