package report

import (
	"fmt"
	"image/color"
	"io"
	"os"
	"strconv"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

// RenderText renders the canonical report as a human-facing terminal report.
func RenderText(w io.Writer, report *models.Report) error {
	if report == nil {
		return fmt.Errorf("nil report")
	}

	categories, err := orderedCategories(report)
	if err != nil {
		return err
	}

	styles := newTextStyles(shouldUseColor(w))

	var out strings.Builder
	out.WriteString(styles.heading("HelmDoc scan report"))
	out.WriteString("\n")
	out.WriteString(fmt.Sprintf("Chart: %s\n\n", chartDisplayName(report)))
	out.WriteString("Overall: ")
	out.WriteString(styles.grade(string(report.OverallGrade), report.OverallGrade))
	out.WriteString("\n")
	out.WriteString(fmt.Sprintf("Score: %s/100\n", formatScore(report.OverallScore)))
	out.WriteString(fmt.Sprintf("Total findings: %d\n\n", report.TotalFindings))
	out.WriteString("Categories:\n")

	for _, category := range categories {
		out.WriteString(fmt.Sprintf(
			"- %s: %s (%s/100, weight %.1f, findings %d)\n",
			displayCategory(category.Category),
			styles.grade(string(category.Grade), category.Grade),
			formatScore(category.Score),
			category.Weight,
			len(category.Findings),
		))
	}

	if report.TotalFindings == 0 {
		out.WriteString("\nNo findings.\n")
		_, err := io.WriteString(w, out.String())
		return err
	}

	out.WriteString("\nFindings:\n")
	for _, category := range categories {
		if len(category.Findings) == 0 {
			continue
		}

		out.WriteString(fmt.Sprintf("%s findings (%d):\n", displayCategory(category.Category), len(category.Findings)))
		for _, finding := range category.Findings {
			out.WriteString(fmt.Sprintf(
				"- [%s][%s] %s @ %s\n",
				finding.RuleID,
				styles.severity(finding.Severity.String(), finding.Severity),
				orDefault(finding.Resource, "<unknown-resource>"),
				orDefault(finding.Path, "<unknown-template>"),
			))
			if finding.Title != "" {
				out.WriteString(fmt.Sprintf("  Title: %s\n", finding.Title))
			}
			if finding.Description != "" {
				out.WriteString(fmt.Sprintf("  Description: %s\n", finding.Description))
			}
			if finding.Remediation != "" {
				out.WriteString(fmt.Sprintf("  Remediation: %s\n", finding.Remediation))
			}
		}
		out.WriteString("\n")
	}

	_, err = io.WriteString(w, strings.TrimRight(out.String(), "\n")+"\n")
	return err
}

type textStyles struct {
	color bool
}

func newTextStyles(color bool) textStyles {
	return textStyles{color: color}
}

func (s textStyles) heading(text string) string {
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("69"))
	return s.render(style, text)
}

func (s textStyles) grade(text string, grade models.Grade) string {
	style := lipgloss.NewStyle().Bold(true).Foreground(gradeColor(grade))
	return s.render(style, text)
}

func (s textStyles) severity(text string, severity models.Severity) string {
	style := lipgloss.NewStyle().Foreground(severityColor(severity))
	if severity >= models.SeverityError {
		style = style.Bold(true)
	}
	return s.render(style, text)
}

func (s textStyles) render(style lipgloss.Style, text string) string {
	if !s.color {
		return text
	}
	return style.Render(text)
}

func shouldUseColor(w io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}

	file, ok := w.(*os.File)
	if !ok {
		return false
	}

	info, err := file.Stat()
	if err != nil {
		return false
	}

	return info.Mode()&os.ModeCharDevice != 0
}

func gradeColor(grade models.Grade) color.Color {
	switch grade {
	case models.GradeA:
		return lipgloss.Color("42")
	case models.GradeB:
		return lipgloss.Color("39")
	case models.GradeC:
		return lipgloss.Color("220")
	case models.GradeD:
		return lipgloss.Color("208")
	default:
		return lipgloss.Color("196")
	}
}

func severityColor(severity models.Severity) color.Color {
	switch severity {
	case models.SeverityInfo:
		return lipgloss.Color("244")
	case models.SeverityWarning:
		return lipgloss.Color("220")
	case models.SeverityError:
		return lipgloss.Color("208")
	default:
		return lipgloss.Color("196")
	}
}

func orderedCategories(report *models.Report) ([]models.CategoryScore, error) {
	categoryMap := make(map[models.Category]models.CategoryScore, len(report.Categories))
	for _, category := range report.Categories {
		if _, exists := categoryMap[category.Category]; exists {
			return nil, fmt.Errorf("duplicate category %q in report", category.Category)
		}
		categoryMap[category.Category] = category
	}

	ordered := make([]models.CategoryScore, 0, len(models.AllCategories()))
	for _, category := range models.AllCategories() {
		categoryScore, ok := categoryMap[category]
		if !ok {
			return nil, fmt.Errorf("missing category %q in report", category)
		}
		ordered = append(ordered, categoryScore)
	}

	return ordered, nil
}

func chartDisplayName(report *models.Report) string {
	name := strings.TrimSpace(report.ChartName)
	if name == "" {
		name = "<unnamed-chart>"
	}

	version := strings.TrimSpace(report.ChartVersion)
	if version == "" {
		return name
	}

	return fmt.Sprintf("%s@%s", name, version)
}

func displayCategory(category models.Category) string {
	categoryName := strings.TrimSpace(string(category))
	if categoryName == "" {
		return "Other"
	}

	return strings.ToUpper(categoryName[:1]) + categoryName[1:]
}

func orDefault(value, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

func formatScore(score float64) string {
	return strconv.FormatFloat(score, 'f', 1, 64)
}
