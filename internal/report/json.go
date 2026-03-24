package report

import (
	"encoding/json"

	"github.com/belyaev-dev/helmdoc/pkg/models"
)

// RenderJSON renders the canonical report as indented JSON.
func RenderJSON(report models.Report) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}
