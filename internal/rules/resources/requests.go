package resources

import (
	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func init() {
	rules.Register(&RequestsRule{})
}

// RequestsRule flags workload containers that do not define resource requests.
type RequestsRule struct{}

func (r *RequestsRule) ID() string                { return "RES002" }
func (r *RequestsRule) Category() models.Category { return models.CategoryResources }
func (r *RequestsRule) Severity() models.Severity { return models.SeverityWarning }
func (r *RequestsRule) Title() string             { return "Container resource requests are missing" }

func (r *RequestsRule) Check(ctx rules.AnalysisContext) []models.Finding {
	return checkMissingResourceRequirement(ctx, "requests")
}
