package models

import (
	"fmt"
	"sort"
)

// ValuePath describes one flattened values.yaml path.
type ValuePath struct {
	Default any    `json:"default,omitempty"`
	Type    string `json:"type"`
}

// ValuesSurface is a rule-friendly, read-mostly index of chart values paths.
type ValuesSurface struct {
	entries map[string]ValuePath
}

// NewValuesSurface constructs a values surface from the provided entries.
func NewValuesSurface(entries map[string]ValuePath) *ValuesSurface {
	surface := &ValuesSurface{entries: make(map[string]ValuePath, len(entries))}
	for path, entry := range entries {
		surface.entries[path] = ValuePath{Default: cloneValue(entry.Default), Type: entry.Type}
	}
	return surface
}

// AddPath records or overwrites a flattened values path.
func (s *ValuesSurface) AddPath(path string, defaultValue any, typeName string) {
	if s == nil {
		return
	}
	if s.entries == nil {
		s.entries = make(map[string]ValuePath)
	}
	s.entries[path] = ValuePath{Default: cloneValue(defaultValue), Type: typeName}
}

// HasPath reports whether the surface contains the given dot path.
func (s *ValuesSurface) HasPath(path string) bool {
	if s == nil {
		return false
	}
	_, ok := s.entries[path]
	return ok
}

// Get returns the metadata for a flattened values path.
func (s *ValuesSurface) Get(path string) (ValuePath, bool) {
	if s == nil {
		return ValuePath{}, false
	}
	entry, ok := s.entries[path]
	if !ok {
		return ValuePath{}, false
	}
	return ValuePath{Default: cloneValue(entry.Default), Type: entry.Type}, true
}

// GetDefault returns the default value for a flattened values path, or nil if absent.
func (s *ValuesSurface) GetDefault(path string) any {
	if s == nil {
		return nil
	}
	entry, ok := s.entries[path]
	if !ok {
		return nil
	}
	return cloneValue(entry.Default)
}

// PathType returns the recorded type for a flattened values path, or an empty string if absent.
func (s *ValuesSurface) PathType(path string) string {
	entry, ok := s.Get(path)
	if !ok {
		return ""
	}
	return entry.Type
}

// AllPaths returns all recorded paths in stable sorted order.
func (s *ValuesSurface) AllPaths() []string {
	if s == nil || len(s.entries) == 0 {
		return nil
	}

	paths := make([]string, 0, len(s.entries))
	for path := range s.entries {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, nested := range typed {
			cloned[key] = cloneValue(nested)
		}
		return cloned
	case map[any]any:
		cloned := make(map[string]any, len(typed))
		for key, nested := range typed {
			cloned[fmt.Sprint(key)] = cloneValue(nested)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for index, nested := range typed {
			cloned[index] = cloneValue(nested)
		}
		return cloned
	default:
		return value
	}
}
