package fix

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/belyaev-dev/helmdoc/pkg/models"
	"sigs.k8s.io/yaml"
)

// MergeValuesOverrides deep-merges all applied values fixes into one deterministic values tree.
func MergeValuesOverrides(fixes []AppliedValuesFix) (map[string]any, error) {
	merged := map[string]any{}
	for _, fix := range fixes {
		patch := nestValue(fix.ValuesPath, fix.Value)
		if err := deepMergeInto(merged, patch); err != nil {
			return nil, fmt.Errorf("merge %s for %s: %w", fix.ValuesPath, fix.Finding.RuleID, err)
		}
	}
	return merged, nil
}

// RenderValuesOverridesYAML serializes the merged values overrides document.
func RenderValuesOverridesYAML(plan BundlePlan) ([]byte, error) {
	plan = normalizedBundlePlan(plan)
	if err := validateBundlePlan(plan); err != nil {
		return nil, err
	}

	merged, err := MergeValuesOverrides(plan.AppliedValuesFixes)
	if err != nil {
		return nil, err
	}

	var data []byte
	if len(merged) == 0 {
		data = []byte("{}\n")
	} else {
		data, err = yaml.Marshal(merged)
		if err != nil {
			return nil, fmt.Errorf("marshal values overrides: %w", err)
		}
		if len(data) == 0 || data[len(data)-1] != '\n' {
			data = append(data, '\n')
		}
	}

	return prependCommentBlock(valuesProvenanceCommentLines(plan), data), nil
}

// RenderREADME renders the human-readable fix summary.
func RenderREADME(plan BundlePlan) ([]byte, error) {
	plan = normalizedBundlePlan(plan)
	if err := validateBundlePlan(plan); err != nil {
		return nil, err
	}

	lines := []string{
		"# Helmdoc fix bundle",
		"",
		"This bundle keeps Helm values overrides separate from Kustomize fallbacks so you can review what helmdoc can apply automatically, what still needs manual work, and what remains advisory-only.",
		"",
		"## How to apply this bundle",
		"",
		"1. Review the explanations below before copying any generated changes into your deployment workflow.",
		"2. Apply `values-overrides.yaml` with your normal Helm process, for example `helm upgrade --install RELEASE CHART -f values-overrides.yaml`.",
	}
	if plan.HasKustomizePatches() {
		lines = append(lines, "3. If you use the generated `kustomize/` overlay, render the chart with the values overrides first and then apply `kustomize/kustomization.yaml` so the patch files layer on top of the rendered manifests.")
	} else {
		lines = append(lines, "3. No Kustomize overlay was needed for this bundle.")
	}
	if plan.HasAdvisoryFindings() || plan.HasPendingFindings() {
		lines = append(lines, "4. Follow the advisory-only and still-pending sections for findings that helmdoc intentionally left for manual review.")
	}

	lines = append(lines, "")
	lines = append(lines, renderAppliedValuesFixSection(plan.AppliedValuesFixes)...)
	lines = append(lines, "")
	lines = append(lines, renderKustomizePatchSection(plan.KustomizePatches)...)
	lines = append(lines, "")
	lines = append(lines, renderAdvisorySection(plan.AdvisoryFindings)...)
	lines = append(lines, "")
	lines = append(lines, renderPendingSection(plan.PendingFindings)...)

	return []byte(strings.Join(lines, "\n") + "\n"), nil
}

func renderAppliedValuesFixSection(fixes []AppliedValuesFix) []string {
	lines := []string{fmt.Sprintf("## Applied values fixes (%d)", len(fixes))}
	if len(fixes) == 0 {
		return append(lines, "- None.")
	}

	for _, fix := range fixes {
		lines = append(lines,
			fmt.Sprintf("- `%s` for %s", fix.Finding.RuleID, markdownFindingLocation(fix.Finding)),
			fmt.Sprintf("  - Values path: `%s`", fix.ValuesPath),
		)
		if summary := normalizeSentence(fix.Summary); summary != "" {
			lines = append(lines, fmt.Sprintf("  - Helmdoc change: %s", summary))
		}
		if description := normalizeSentence(fix.Finding.Description); description != "" {
			lines = append(lines, fmt.Sprintf("  - Rule detail: %s", description))
		}
		if remediation := normalizeSentence(fix.Finding.Remediation); remediation != "" {
			lines = append(lines, fmt.Sprintf("  - Chart hint: %s", remediation))
		}
	}
	return lines
}

func renderKustomizePatchSection(patches []KustomizePatch) []string {
	lines := []string{fmt.Sprintf("## Kustomize patches (%d)", len(patches))}
	if len(patches) == 0 {
		return append(lines, "- None.")
	}

	for _, patch := range patches {
		lines = append(lines,
			fmt.Sprintf("- `%s` for %s", patch.Finding.RuleID, markdownFindingLocation(patch.Finding)),
			fmt.Sprintf("  - Patch file: `%s`", KustomizePatchRelativePath(patch)),
			fmt.Sprintf("  - Target resource: `%s/%s`", patch.Target.Kind, patch.Target.Name),
		)
		if summary := normalizeSentence(patch.Summary); summary != "" {
			lines = append(lines, fmt.Sprintf("  - Helmdoc change: %s", summary))
		}
		if patch.ContainerName != "" {
			lines = append(lines, fmt.Sprintf("  - Target container: `%s`", patch.ContainerName))
		}
		if description := normalizeSentence(patch.Finding.Description); description != "" {
			lines = append(lines, fmt.Sprintf("  - Rule detail: %s", description))
		}
		if remediation := normalizeSentence(patch.Finding.Remediation); remediation != "" {
			lines = append(lines, fmt.Sprintf("  - Manual follow-up: %s", remediation))
		}
	}
	return lines
}

func renderAdvisorySection(findings []AdvisoryFinding) []string {
	lines := []string{fmt.Sprintf("## Advisory-only findings (%d)", len(findings))}
	if len(findings) == 0 {
		return append(lines, "- None.")
	}

	for _, advisory := range findings {
		lines = append(lines, fmt.Sprintf("- `%s` for %s", advisory.Finding.RuleID, markdownFindingLocation(advisory.Finding)))
		if explanation := normalizeSentence(advisory.Explanation); explanation != "" {
			lines = append(lines, fmt.Sprintf("  - Why helmdoc left this advisory-only: %s", explanation))
		}
		if description := normalizeSentence(advisory.Finding.Description); description != "" {
			lines = append(lines, fmt.Sprintf("  - Rule detail: %s", description))
		}
		if remediation := normalizeSentence(advisory.Finding.Remediation); remediation != "" {
			lines = append(lines, fmt.Sprintf("  - Manual follow-up: %s", remediation))
		}
	}
	return lines
}

func renderPendingSection(findings []PendingFinding) []string {
	lines := []string{fmt.Sprintf("## Findings pending another fix path (%d)", len(findings))}
	if len(findings) == 0 {
		return append(lines, "- None.")
	}

	for _, pending := range findings {
		lines = append(lines, fmt.Sprintf("- `%s` for %s", pending.Finding.RuleID, markdownFindingLocation(pending.Finding)))
		if reason := normalizeSentence(pending.Reason); reason != "" {
			lines = append(lines, fmt.Sprintf("  - Still pending because: %s", reason))
		}
		if description := normalizeSentence(pending.Finding.Description); description != "" {
			lines = append(lines, fmt.Sprintf("  - Rule detail: %s", description))
		}
		if remediation := normalizeSentence(pending.Finding.Remediation); remediation != "" {
			lines = append(lines, fmt.Sprintf("  - Manual follow-up: %s", remediation))
		}
	}
	return lines
}

func valuesProvenanceCommentLines(plan BundlePlan) []string {
	lines := []string{
		"Generated by helmdoc fix.",
		"File: values-overrides.yaml",
		"Apply this file with your Helm values workflow, for example: helm upgrade --install RELEASE CHART -f values-overrides.yaml",
		"These overrides contain only findings helmdoc could map to chart values.",
	}
	if len(plan.AppliedValuesFixes) == 0 {
		return append(lines, "Provenance: no values-backed fixes were planned for this bundle.")
	}

	lines = append(lines, "Provenance:")
	for _, fix := range plan.AppliedValuesFixes {
		lines = append(lines,
			fmt.Sprintf("- %s -> %s", plainFindingLocation(fix.Finding), fix.ValuesPath),
		)
		if summary := normalizeSentence(fix.Summary); summary != "" {
			lines = append(lines, "planned fix: "+summary)
		}
		if description := normalizeSentence(fix.Finding.Description); description != "" {
			lines = append(lines, "rule detail: "+description)
		}
	}
	return lines
}

func prependCommentBlock(lines []string, data []byte) []byte {
	if len(lines) == 0 {
		return data
	}

	var builder strings.Builder
	for _, line := range lines {
		text := normalizeSentence(line)
		if text == "" {
			builder.WriteString("#\n")
			continue
		}
		builder.WriteString("# ")
		builder.WriteString(text)
		builder.WriteByte('\n')
	}
	builder.Write(data)
	return []byte(builder.String())
}

func normalizedBundlePlan(plan BundlePlan) BundlePlan {
	normalized := BundlePlan{
		AppliedValuesFixes: append([]AppliedValuesFix(nil), plan.AppliedValuesFixes...),
		KustomizePatches:   append([]KustomizePatch(nil), plan.KustomizePatches...),
		AdvisoryFindings:   append([]AdvisoryFinding(nil), plan.AdvisoryFindings...),
		PendingFindings:    append([]PendingFinding(nil), plan.PendingFindings...),
	}
	sortBundlePlan(&normalized)
	return normalized
}

func markdownFindingLocation(finding models.Finding) string {
	parts := make([]string, 0, 2)
	if resource := strings.TrimSpace(finding.Resource); resource != "" {
		parts = append(parts, fmt.Sprintf("`%s`", resource))
	}
	if path := strings.TrimSpace(finding.Path); path != "" {
		parts = append(parts, fmt.Sprintf("`%s`", path))
	}
	if len(parts) == 0 {
		return "the rendered chart"
	}
	if len(parts) == 1 {
		return parts[0]
	}
	return parts[0] + " @ " + parts[1]
}

func plainFindingLocation(finding models.Finding) string {
	parts := []string{strings.TrimSpace(finding.RuleID)}
	if resource := strings.TrimSpace(finding.Resource); resource != "" {
		parts = append(parts, resource)
	}
	if path := strings.TrimSpace(finding.Path); path != "" {
		parts = append(parts, "@ "+path)
	}
	return strings.TrimSpace(strings.Join(parts, " "))
}

func normalizeSentence(text string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
}

func nestValue(path string, value any) map[string]any {
	segments := strings.Split(path, ".")
	if len(segments) == 0 || path == "" {
		return map[string]any{}
	}

	result := map[string]any{segments[len(segments)-1]: cloneValue(value)}
	for i := len(segments) - 2; i >= 0; i-- {
		result = map[string]any{segments[i]: result}
	}
	return result
}

func deepMergeInto(dst, src map[string]any) error {
	keys := make([]string, 0, len(src))
	for key := range src {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		srcValue := src[key]
		if existing, ok := dst[key]; ok {
			existingMap, existingIsMap := existing.(map[string]any)
			srcMap, srcIsMap := srcValue.(map[string]any)
			switch {
			case existingIsMap && srcIsMap:
				if err := deepMergeInto(existingMap, srcMap); err != nil {
					return err
				}
			case reflect.DeepEqual(existing, srcValue):
				continue
			default:
				return fmt.Errorf("conflicting values for key %q", key)
			}
			continue
		}

		dst[key] = cloneValue(srcValue)
	}
	return nil
}

func cloneValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(typed))
		for key, nested := range typed {
			cloned[key] = cloneValue(nested)
		}
		return cloned
	case []any:
		cloned := make([]any, len(typed))
		for i, nested := range typed {
			cloned[i] = cloneValue(nested)
		}
		return cloned
	default:
		return typed
	}
}
