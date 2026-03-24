package models

import (
	"reflect"
	"testing"
)

func TestValuesSurfaceQueries(t *testing.T) {
	seedResourceDefaults := map[string]any{"limits": map[string]any{"cpu": "500m"}}
	surface := NewValuesSurface(map[string]ValuePath{
		"controller.resources":            {Default: seedResourceDefaults, Type: "object"},
		"controller.resources.limits.cpu": {Default: "500m", Type: "string"},
		"controller.tolerations.0.key":    {Default: "workload", Type: "string"},
		"persistence.enabled":             {Default: true, Type: "bool"},
	})

	if !surface.HasPath("controller.resources.limits.cpu") {
		t.Fatal("HasPath(controller.resources.limits.cpu) = false, want true")
	}

	if got := surface.GetDefault("controller.resources.limits.cpu"); got != "500m" {
		t.Fatalf("GetDefault(controller.resources.limits.cpu) = %v, want 500m", got)
	}

	if got := surface.PathType("persistence.enabled"); got != "bool" {
		t.Fatalf("PathType(persistence.enabled) = %q, want bool", got)
	}

	if surface.HasPath("controller.missing") {
		t.Fatal("HasPath(controller.missing) = true, want false")
	}

	if got := surface.GetDefault("controller.missing"); got != nil {
		t.Fatalf("GetDefault(controller.missing) = %v, want nil", got)
	}

	if got := surface.PathType("controller.missing"); got != "" {
		t.Fatalf("PathType(controller.missing) = %q, want empty string", got)
	}

	wantPaths := []string{
		"controller.resources",
		"controller.resources.limits.cpu",
		"controller.tolerations.0.key",
		"persistence.enabled",
	}
	if got := surface.AllPaths(); !reflect.DeepEqual(got, wantPaths) {
		t.Fatalf("AllPaths() = %#v, want %#v", got, wantPaths)
	}

	seedResourceDefaults["limits"].(map[string]any)["cpu"] = "1000m"
	storedDefaults, ok := surface.GetDefault("controller.resources").(map[string]any)
	if !ok {
		t.Fatalf("GetDefault(controller.resources) type = %T, want map[string]any", surface.GetDefault("controller.resources"))
	}
	if got := storedDefaults["limits"].(map[string]any)["cpu"]; got != "500m" {
		t.Fatalf("stored defaults cpu after seed mutation = %#v, want %#v", got, "500m")
	}

	storedDefaults["limits"].(map[string]any)["cpu"] = "750m"
	freshDefaults, ok := surface.GetDefault("controller.resources").(map[string]any)
	if !ok {
		t.Fatalf("GetDefault(controller.resources) fresh type = %T, want map[string]any", surface.GetDefault("controller.resources"))
	}
	if got := freshDefaults["limits"].(map[string]any)["cpu"]; got != "500m" {
		t.Fatalf("stored defaults cpu after returned-value mutation = %#v, want %#v", got, "500m")
	}
}
