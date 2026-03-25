package helmdoc

import (
	"fmt"
	"strings"

	chartanalysis "github.com/belyaev-dev/helmdoc/internal/chart"
	scanconfig "github.com/belyaev-dev/helmdoc/internal/config"
	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/internal/score"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

type chartAnalysis struct {
	Config   *scanconfig.Config
	Context  rules.AnalysisContext
	Findings []models.Finding
	Report   models.Report
}

func analyzeChart(chartPath, configPath string) (*chartAnalysis, error) {
	cfg, err := scanconfig.LoadConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	loadedChart, err := chartanalysis.LoadChart(chartPath)
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}

	rendered, err := chartanalysis.RenderChart(loadedChart)
	if err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}

	ctx := rules.AnalysisContext{
		Chart:             loadedChart,
		RenderedResources: rendered,
		ValuesSurface:     chartanalysis.AnalyzeValues(loadedChart),
	}
	findings := rules.RunAllWithConfig(ctx, cfg)

	report := score.ComputeReport(findings)
	if loadedChart.Metadata != nil {
		report.ChartName = strings.TrimSpace(loadedChart.Metadata.Name)
		report.ChartVersion = strings.TrimSpace(loadedChart.Metadata.Version)
	}

	return &chartAnalysis{
		Config:   cfg,
		Context:  ctx,
		Findings: findings,
		Report:   report,
	}, nil
}
