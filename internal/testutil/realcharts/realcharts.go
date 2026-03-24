package realcharts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	chartanalysis "github.com/belyaev-dev/helmdoc/internal/chart"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	_ "github.com/belyaev-dev/helmdoc/internal/rules/all"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

type Manifest struct {
	Fixtures []Fixture `json:"fixtures"`
}

type Fixture struct {
	ID           string   `json:"id"`
	Path         string   `json:"path"`
	SourceURL    string   `json:"source_url"`
	ChartName    string   `json:"chart_name"`
	ChartVersion string   `json:"chart_version"`
	Expected     Expected `json:"expected"`
}

type Expected struct {
	OverallGrade      models.Grade      `json:"overall_grade"`
	OverallScore      float64           `json:"overall_score"`
	TotalFindings     int               `json:"total_findings"`
	CategorySummaries []CategorySummary `json:"category_summaries"`
	FindingTuples     []ExpectedFinding `json:"finding_tuples"`
	AnchorFindings    []AnchorFinding   `json:"anchor_findings"`
}

type CategorySummary struct {
	Category models.Category `json:"category"`
	Score    float64         `json:"score"`
	Grade    models.Grade    `json:"grade"`
	Findings int             `json:"findings"`
}

type ExpectedFinding struct {
	RuleID   string          `json:"rule_id"`
	Category models.Category `json:"category"`
	Severity models.Severity `json:"severity"`
	Path     string          `json:"path"`
	Resource string          `json:"resource"`
}

type AnchorFinding struct {
	RuleID      string          `json:"rule_id"`
	Category    models.Category `json:"category"`
	Severity    models.Severity `json:"severity"`
	Path        string          `json:"path"`
	Resource    string          `json:"resource"`
	Title       string          `json:"title"`
	Description string          `json:"description"`
	Remediation string          `json:"remediation"`
}

func LoadManifest(t testing.TB) Manifest {
	t.Helper()

	manifestPath := filepath.Join(findRepoRoot(t), "testdata", "real-chart-fixtures.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", manifestPath, err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", manifestPath, err)
	}

	return manifest
}

func FixtureByID(t testing.TB, id string) Fixture {
	t.Helper()

	for _, fixture := range LoadManifest(t).Fixtures {
		if fixture.ID == id {
			return fixture
		}
	}

	t.Fatalf("fixture %q missing from manifest", id)
	return Fixture{}
}

func AnalysisContext(t testing.TB, fixture Fixture) rules.AnalysisContext {
	t.Helper()

	chartPath := FixturePath(t, fixture)
	loadedChart, err := chartanalysis.LoadChart(chartPath)
	if err != nil {
		t.Fatalf("LoadChart(%q) error = %v", chartPath, err)
	}

	rendered, err := chartanalysis.RenderChart(loadedChart)
	if err != nil {
		t.Fatalf("RenderChart(%q) error = %v", chartPath, err)
	}

	return rules.AnalysisContext{
		Chart:             loadedChart,
		RenderedResources: rendered,
		ValuesSurface:     chartanalysis.AnalyzeValues(loadedChart),
	}
}

func RunRules(t testing.TB, fixture Fixture) []models.Finding {
	t.Helper()

	ctx := AnalysisContext(t, fixture)
	return rules.RunAll(ctx)
}

func RepoPath(t testing.TB, rel string) string {
	t.Helper()

	return filepath.Join(findRepoRoot(t), filepath.FromSlash(rel))
}

func FixturePath(t testing.TB, fixture Fixture) string {
	t.Helper()

	return RepoPath(t, fixture.Path)
}

func findRepoRoot(t testing.TB) string {
	t.Helper()

	for _, candidate := range []string{".", "..", filepath.Join("..", ".."), filepath.Join("..", "..", ".."), filepath.Join("..", "..", "..", "..")} {
		manifestPath := filepath.Join(candidate, "testdata", "real-chart-fixtures.json")
		if _, err := os.Stat(manifestPath); err == nil {
			return candidate
		}
	}

	t.Fatal("unable to locate repo root from test working directory")
	return ""
}
