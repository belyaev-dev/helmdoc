package score

import "github.com/belyaev-dev/helmdoc/pkg/models"

const maxCategoryScore = 100.0

var severityDeductions = map[models.Severity]float64{
	models.SeverityCritical: 25,
	models.SeverityError:    15,
	models.SeverityWarning:  8,
	models.SeverityInfo:     3,
}

var categoryWeights = map[models.Category]float64{
	models.CategorySecurity:     3.0,
	models.CategoryResources:    2.5,
	models.CategoryHealth:       2.0,
	models.CategoryStorage:      1.5,
	models.CategoryAvailability: 1.0,
	models.CategoryNetwork:      1.0,
	models.CategoryImages:       1.0,
	models.CategoryIngress:      1.0,
	models.CategoryScaling:      1.0,
	models.CategoryConfig:       1.0,
}

// ComputeReport converts findings into the canonical weighted report contract.
func ComputeReport(findings []models.Finding) models.Report {
	grouped := make(map[models.Category][]models.Finding, len(models.AllCategories()))
	for _, finding := range findings {
		grouped[finding.Category] = append(grouped[finding.Category], finding)
	}

	categories := make([]models.CategoryScore, 0, len(models.AllCategories()))
	var weightedTotal float64
	var totalWeight float64

	for _, category := range models.AllCategories() {
		weight := categoryWeights[category]
		categoryFindings := append([]models.Finding(nil), grouped[category]...)
		score := maxCategoryScore
		for _, finding := range categoryFindings {
			score -= severityDeductions[finding.Severity]
		}
		if score < 0 {
			score = 0
		}

		categories = append(categories, models.CategoryScore{
			Category: category,
			Score:    score,
			Grade:    models.GradeFromScore(score),
			Weight:   weight,
			Findings: categoryFindings,
		})
		weightedTotal += score * weight
		totalWeight += weight
	}

	overallScore := maxCategoryScore
	if totalWeight > 0 {
		overallScore = weightedTotal / totalWeight
	}

	return models.Report{
		OverallScore:  overallScore,
		OverallGrade:  models.GradeFromScore(overallScore),
		Categories:    categories,
		TotalFindings: len(findings),
	}
}
