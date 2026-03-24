// Package chart handles Helm chart loading and parsing.
package chart

import (
	"fmt"
	"os"
	"path/filepath"

	"helm.sh/helm/v3/pkg/chart"
	"helm.sh/helm/v3/pkg/chart/loader"
)

// LoadChart loads a Helm chart from a local path (directory or .tgz).
func LoadChart(path string) (*chart.Chart, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolving chart path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("chart path not found: %w", err)
	}

	var c *chart.Chart
	if info.IsDir() {
		c, err = loader.LoadDir(abs)
	} else {
		c, err = loader.LoadFile(abs)
	}
	if err != nil {
		return nil, fmt.Errorf("loading chart: %w", err)
	}

	return c, nil
}
