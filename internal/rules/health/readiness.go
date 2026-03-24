package health

import (
	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&ReadinessProbeRule{})
}

// ReadinessProbeRule flags workload containers that do not define a readiness probe.
type ReadinessProbeRule struct{}

func (r *ReadinessProbeRule) ID() string                { return "HLT002" }
func (r *ReadinessProbeRule) Category() models.Category { return models.CategoryHealth }
func (r *ReadinessProbeRule) Severity() models.Severity { return models.SeverityWarning }
func (r *ReadinessProbeRule) Title() string             { return "Container readiness probe is missing" }

func (r *ReadinessProbeRule) Check(ctx rules.AnalysisContext) []models.Finding {
	return checkMissingProbe(ctx, "readinessProbe")
}
