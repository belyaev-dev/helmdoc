package chart

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type realChartFixturesManifest struct {
	Fixtures []realChartFixture `json:"fixtures"`
}

type realChartFixture struct {
	ID           string                   `json:"id"`
	Path         string                   `json:"path"`
	SourceURL    string                   `json:"source_url"`
	ChartName    string                   `json:"chart_name"`
	ChartVersion string                   `json:"chart_version"`
	Expected     realChartFixtureExpected `json:"expected"`
}

type realChartFixtureExpected struct {
	OverallGrade  string  `json:"overall_grade"`
	OverallScore  float64 `json:"overall_score"`
	TotalFindings int     `json:"total_findings"`
}

func TestLoadAndRenderRealChartFixtures(t *testing.T) {
	manifest := loadRealChartFixturesManifest(t)
	if len(manifest.Fixtures) != 3 {
		t.Fatalf("len(manifest.Fixtures) = %d, want 3", len(manifest.Fixtures))
	}

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			if fixture.Path == "" {
				t.Fatal("fixture path is empty")
			}
			if fixture.SourceURL == "" {
				t.Fatal("fixture source_url is empty")
			}
			if fixture.ChartName == "" || fixture.ChartVersion == "" {
				t.Fatalf("fixture chart metadata is incomplete: %#v", fixture)
			}
			if fixture.Expected.OverallGrade == "" || fixture.Expected.OverallScore <= 0 || fixture.Expected.TotalFindings <= 0 {
				t.Fatalf("fixture expected baseline is incomplete: %#v", fixture.Expected)
			}

			chartPath := filepath.Join(realChartFixturesRepoRoot(t), filepath.FromSlash(fixture.Path))
			info, err := os.Stat(chartPath)
			if err != nil {
				t.Fatalf("os.Stat(%q) error = %v", chartPath, err)
			}
			if info.IsDir() != (filepath.Ext(chartPath) == "") {
				t.Fatalf("fixture %q directory/archive shape mismatch: isDir=%t", chartPath, info.IsDir())
			}

			loadedChart, err := LoadChart(chartPath)
			if err != nil {
				t.Fatalf("LoadChart(%q) error = %v", chartPath, err)
			}
			if loadedChart.Metadata == nil {
				t.Fatal("loaded chart metadata is nil")
			}
			if got := loadedChart.Metadata.Name; got != fixture.ChartName {
				t.Fatalf("loaded chart name = %q, want %q", got, fixture.ChartName)
			}
			if got := loadedChart.Metadata.Version; got != fixture.ChartVersion {
				t.Fatalf("loaded chart version = %q, want %q", got, fixture.ChartVersion)
			}

			rendered, err := RenderChart(loadedChart)
			if err != nil {
				t.Fatalf("RenderChart(%q) error = %v", chartPath, err)
			}
			if len(rendered) == 0 {
				t.Fatal("RenderChart() returned no rendered templates")
			}

			totalResources := 0
			for templatePath, resources := range rendered {
				if len(resources) == 0 {
					t.Fatalf("rendered[%q] unexpectedly contains zero resources", templatePath)
				}
				totalResources += len(resources)
			}
			if totalResources == 0 {
				t.Fatal("RenderChart() returned zero rendered resources")
			}

			t.Logf("fixture=%s chart=%s version=%s templates=%d resources=%d expected_grade=%s expected_score=%.1f expected_findings=%d", fixture.ID, loadedChart.Metadata.Name, loadedChart.Metadata.Version, len(rendered), totalResources, fixture.Expected.OverallGrade, fixture.Expected.OverallScore, fixture.Expected.TotalFindings)
		})
	}
}

func loadRealChartFixturesManifest(t *testing.T) realChartFixturesManifest {
	t.Helper()

	manifestPath := filepath.Join(realChartFixturesRepoRoot(t), "testdata", "real-chart-fixtures.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", manifestPath, err)
	}

	var manifest realChartFixturesManifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", manifestPath, err)
	}

	return manifest
}

func realChartFixturesRepoRoot(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..")
}
