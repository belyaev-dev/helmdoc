package fix

import (
	"fmt"
	"sort"
	"strings"

	"github.com/belyaev-dev/helmdoc/internal/rules"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

// PlanBundle maps findings into applied values fixes, advisory findings, Kustomize fallbacks, and still-pending findings.
func PlanBundle(ctx rules.AnalysisContext, findings []models.Finding) BundlePlan {
	plan := PlanValuesBundle(ctx, findings)
	plan.KustomizePatches, plan.PendingFindings = planPendingKustomizePatches(ctx, plan.PendingFindings)
	sortBundlePlan(&plan)
	return plan
}

// PlanValuesBundle maps findings into applied values fixes, advisory findings, and explicit pending findings.
func PlanValuesBundle(ctx rules.AnalysisContext, findings []models.Finding) BundlePlan {
	plan := BundlePlan{
		AppliedValuesFixes: make([]AppliedValuesFix, 0, len(findings)),
		AdvisoryFindings:   make([]AdvisoryFinding, 0),
		PendingFindings:    make([]PendingFinding, 0),
	}

	for _, finding := range findings {
		if explanation, ok := advisoryExplanationForFinding(finding); ok {
			plan.AdvisoryFindings = append(plan.AdvisoryFindings, AdvisoryFinding{
				Finding:     finding,
				Explanation: explanation,
			})
			continue
		}

		fix, pending, ok := planFinding(ctx.ValuesSurface, findings, finding)
		if ok {
			plan.AppliedValuesFixes = append(plan.AppliedValuesFixes, fix)
			continue
		}
		plan.PendingFindings = append(plan.PendingFindings, pending)
	}

	return plan
}

func planPendingKustomizePatches(ctx rules.AnalysisContext, pending []PendingFinding) ([]KustomizePatch, []PendingFinding) {
	patches := make([]KustomizePatch, 0, len(pending))
	remaining := make([]PendingFinding, 0)

	for _, deferred := range pending {
		patch, unresolved, ok := planKustomizePatch(ctx, deferred)
		if ok {
			patches = append(patches, patch)
			continue
		}
		remaining = append(remaining, unresolved)
	}

	return patches, remaining
}

func planKustomizePatch(ctx rules.AnalysisContext, pending PendingFinding) (KustomizePatch, PendingFinding, bool) {
	resource, err := lookupRenderedResource(ctx.RenderedResources, pending.Finding)
	if err != nil {
		return KustomizePatch{}, pendingFindingWithReason(pending, err.Error()), false
	}

	containerName, err := lookupTargetContainerName(pending.Finding, resource)
	if err != nil {
		return KustomizePatch{}, pendingFindingWithReason(pending, err.Error()), false
	}

	target := ResourceRef{
		APIVersion: resource.APIVersion,
		Kind:       resource.Kind,
		Name:       resource.Name,
		Namespace:  resource.Namespace,
	}
	patch, summary, err := defaultKustomizePatchForFinding(pending.Finding, target, containerName)
	if err != nil {
		return KustomizePatch{}, pendingFindingWithReason(pending, err.Error()), false
	}

	return KustomizePatch{
		Finding:       pending.Finding,
		Target:        target,
		ContainerName: containerName,
		Patch:         patch,
		Summary:       summary,
	}, PendingFinding{}, true
}

func lookupRenderedResource(rendered map[string][]models.K8sResource, finding models.Finding) (models.K8sResource, error) {
	resources := rendered[finding.Path]
	if len(resources) == 0 {
		return models.K8sResource{}, fmt.Errorf("rendered resource lookup missed %s @ %s", finding.Resource, finding.Path)
	}

	kind, name := parseFindingResourceIdentity(finding.Resource)
	for _, resource := range resources {
		if resource.Kind != kind {
			continue
		}
		if name != "" && resource.Name != name {
			continue
		}
		return resource, nil
	}

	return models.K8sResource{}, fmt.Errorf("rendered resource lookup missed %s @ %s", finding.Resource, finding.Path)
}

func lookupTargetContainerName(finding models.Finding, resource models.K8sResource) (string, error) {
	containers := make([]rules.WorkloadContainer, 0, 1)
	rules.IterateWorkloadContainers(map[string][]models.K8sResource{finding.Path: []models.K8sResource{resource}}, func(container rules.WorkloadContainer) bool {
		if container.IsInit {
			return true
		}
		containers = append(containers, container)
		return true
	})
	if len(containers) == 0 {
		return "", fmt.Errorf("rendered resource %s has no supported workload containers to patch", finding.Resource)
	}

	expected := extractFindingContainerName(finding)
	if expected != "" {
		for _, container := range containers {
			if container.Name != expected {
				continue
			}
			if container.Name == "" {
				return "", fmt.Errorf("rendered resource %s has a target container without a name", finding.Resource)
			}
			return container.Name, nil
		}
		return "", fmt.Errorf("rendered resource %s does not include target container %q", finding.Resource, expected)
	}

	if len(containers) != 1 {
		return "", fmt.Errorf("rendered resource %s has %d target containers but the finding does not identify one", finding.Resource, len(containers))
	}
	if containers[0].Name == "" {
		return "", fmt.Errorf("rendered resource %s has a target container without a name", finding.Resource)
	}
	return containers[0].Name, nil
}

func parseFindingResourceIdentity(resource string) (string, string) {
	kind, name, ok := strings.Cut(resource, "/")
	if !ok {
		return strings.TrimSpace(resource), ""
	}
	return strings.TrimSpace(kind), strings.TrimSpace(name)
}

func extractFindingContainerName(finding models.Finding) string {
	for _, candidate := range []string{finding.Description, finding.Remediation} {
		name, ok := extractQuotedContainerName(candidate)
		if ok {
			return name
		}
	}
	return ""
}

func extractQuotedContainerName(text string) (string, bool) {
	const marker = "container \""
	idx := strings.Index(text, marker)
	if idx == -1 {
		return "", false
	}

	start := idx + len(marker)
	end := strings.Index(text[start:], "\"")
	if end <= 0 {
		return "", false
	}
	return text[start : start+end], true
}

func pendingFindingWithReason(pending PendingFinding, reason string) PendingFinding {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return pending
	}
	if pending.Reason == "" {
		pending.Reason = reason
		return pending
	}
	if strings.Contains(pending.Reason, reason) {
		return pending
	}
	pending.Reason = pending.Reason + "; " + reason
	return pending
}

func advisoryExplanationForFinding(finding models.Finding) (string, bool) {
	switch finding.RuleID {
	case "IMG001":
		return "The correct image tag depends on your deployment target, so helmdoc cannot safely choose one automatically.", true
	case "IMG002":
		return "Pinning by digest requires looking up the correct digest in your container registry, so helmdoc cannot determine it automatically.", true
	case "CFG001":
		return "Replacing a hardcoded endpoint requires deployment-specific knowledge about your application's networking topology, so helmdoc leaves this as an advisory-only finding.", true
	case "ING001":
		return "Configuring Ingress TLS requires deployment-specific domains, certificates, and issuer details, so helmdoc cannot auto-fix it safely.", true
	default:
		return "", false
	}
}

func planFinding(surface *models.ValuesSurface, findings []models.Finding, finding models.Finding) (AppliedValuesFix, PendingFinding, bool) {
	valuesPath, summary, reason, ok := selectValuesPath(surface, findings, finding)
	if !ok {
		return AppliedValuesFix{}, PendingFinding{Finding: finding, Reason: reason}, false
	}

	value, err := defaultPayloadForFinding(finding, valuesPath, surface)
	if err != nil {
		return AppliedValuesFix{}, PendingFinding{Finding: finding, Reason: err.Error()}, false
	}

	return AppliedValuesFix{
		Finding:    finding,
		ValuesPath: valuesPath,
		Value:      value,
		Summary:    summary,
	}, PendingFinding{}, true
}

func selectValuesPath(surface *models.ValuesSurface, findings []models.Finding, finding models.Finding) (string, string, string, bool) {
	for _, candidate := range knownValuesCandidates(findings, finding) {
		if surfaceExposesPath(surface, candidate.path) {
			return candidate.path, candidate.summary, "", true
		}
	}

	if candidate, ok := suffixFallbackCandidate(surface, finding); ok {
		return candidate.path, candidate.summary, "", true
	}

	return "", "", pendingReasonForFinding(finding), false
}

type valuesCandidate struct {
	path    string
	base    string
	summary string
}

func knownValuesCandidates(findings []models.Finding, finding models.Finding) []valuesCandidate {
	switch finding.RuleID {
	case "SEC003":
		switch finding.Path {
		case "templates/controller-deployment.yaml", "templates/controller-daemonset.yaml":
			return []valuesCandidate{{
				path:    "controller.image.readOnlyRootFilesystem",
				base:    "controller.image",
				summary: "Enable the controller image read-only root filesystem knob.",
			}}
		case "templates/default-backend-deployment.yaml":
			return []valuesCandidate{{
				path:    "defaultBackend.image.readOnlyRootFilesystem",
				base:    "defaultBackend.image",
				summary: "Enable the default-backend image read-only root filesystem knob.",
			}}
		}
	case "RES001":
		if base, ok := resourceValuesBasePath(finding.Path); ok {
			return []valuesCandidate{{
				path:    base + ".resources.limits",
				base:    base + ".resources",
				summary: "Populate resource limits through the chart values surface.",
			}}
		}
	case "RES002":
		if base, ok := resourceValuesBasePath(finding.Path); ok {
			return []valuesCandidate{{
				path:    base + ".resources.requests",
				base:    base + ".resources",
				summary: "Populate resource requests through the chart values surface.",
			}}
		}
	case "NET001":
		switch finding.Path {
		case "templates/controller-deployment.yaml", "templates/controller-daemonset.yaml":
			return []valuesCandidate{{
				path:    "controller.networkPolicy.enabled",
				base:    "controller.networkPolicy",
				summary: "Enable the controller networkPolicy switch.",
			}}
		case "templates/default-backend-deployment.yaml":
			return []valuesCandidate{{
				path:    "defaultBackend.networkPolicy.enabled",
				base:    "defaultBackend.networkPolicy",
				summary: "Enable the default-backend networkPolicy switch.",
			}}
		}
	case "SCL001":
		switch finding.Path {
		case "templates/controller-deployment.yaml", "templates/controller-daemonset.yaml":
			return []valuesCandidate{{
				path:    "controller.autoscaling",
				base:    "controller.autoscaling",
				summary: "Enable controller autoscaling with a conservative baseline.",
			}}
		case "templates/default-backend-deployment.yaml":
			return []valuesCandidate{{
				path:    "defaultBackend.autoscaling",
				base:    "defaultBackend.autoscaling",
				summary: "Enable default-backend autoscaling with a conservative baseline.",
			}}
		}
	case "AVL001":
		switch finding.Path {
		case "templates/controller-deployment.yaml", "templates/controller-daemonset.yaml":
			if hasRelatedFinding(findings, finding.Path, finding.Resource, "SCL001") {
				return []valuesCandidate{{
					path:    "controller.autoscaling.minReplicas",
					base:    "controller.autoscaling",
					summary: "Raise controller autoscaling.minReplicas so the chart can render a controller PodDisruptionBudget.",
				}, {
					path:    "controller.replicaCount",
					base:    "controller",
					summary: "Raise controller replicaCount so the chart can render a controller PodDisruptionBudget.",
				}}
			}
			return []valuesCandidate{{
				path:    "controller.replicaCount",
				base:    "controller",
				summary: "Raise controller replicaCount so the chart can render a controller PodDisruptionBudget.",
			}}
		case "templates/default-backend-deployment.yaml":
			if hasRelatedFinding(findings, finding.Path, finding.Resource, "SCL001") {
				return []valuesCandidate{{
					path:    "defaultBackend.autoscaling.minReplicas",
					base:    "defaultBackend.autoscaling",
					summary: "Raise default-backend autoscaling.minReplicas so the chart can render a PodDisruptionBudget.",
				}, {
					path:    "defaultBackend.replicaCount",
					base:    "defaultBackend",
					summary: "Raise default-backend replicaCount so the chart can render a PodDisruptionBudget.",
				}}
			}
			return []valuesCandidate{{
				path:    "defaultBackend.replicaCount",
				base:    "defaultBackend",
				summary: "Raise default-backend replicaCount so the chart can render a PodDisruptionBudget.",
			}}
		}
	}

	return nil
}

func suffixFallbackCandidate(surface *models.ValuesSurface, finding models.Finding) (valuesCandidate, bool) {
	var suffixes []string
	var summary string

	switch finding.RuleID {
	case "SEC003":
		suffixes = []string{".readOnlyRootFilesystem"}
		summary = "Enable a chart-exposed readOnlyRootFilesystem knob."
	case "RES001":
		suffixes = []string{".resources"}
		summary = "Populate resource limits through the only exposed resources subtree."
	case "RES002":
		suffixes = []string{".resources"}
		summary = "Populate resource requests through the only exposed resources subtree."
	case "NET001":
		suffixes = []string{".networkPolicy", ".networkPolicy.enabled"}
		summary = "Enable the only exposed chart networkPolicy knob."
	case "SCL001":
		suffixes = []string{".autoscaling", ".autoscaling.enabled"}
		summary = "Enable the only exposed chart autoscaling knob."
	case "AVL001":
		suffixes = []string{".replicaCount", ".autoscaling.minReplicas"}
		summary = "Adjust the only exposed replica-count knob so the chart can render a PodDisruptionBudget."
	default:
		return valuesCandidate{}, false
	}

	matched := make([]string, 0, 2)
	for _, path := range surface.AllPaths() {
		for _, suffix := range suffixes {
			if strings.HasSuffix(path, suffix) {
				matched = append(matched, path)
				break
			}
		}
	}
	if len(matched) != 1 {
		return valuesCandidate{}, false
	}

	path := matched[0]
	if finding.RuleID == "RES001" {
		return valuesCandidate{path: path + ".limits", base: path, summary: summary}, true
	}
	if finding.RuleID == "RES002" {
		return valuesCandidate{path: path + ".requests", base: path, summary: summary}, true
	}
	if finding.RuleID == "NET001" && strings.HasSuffix(path, ".networkPolicy") {
		path += ".enabled"
	}
	if finding.RuleID == "SCL001" && strings.HasSuffix(path, ".autoscaling.enabled") {
		path = strings.TrimSuffix(path, ".enabled")
	}

	return valuesCandidate{path: path, summary: summary}, true
}

func pendingReasonForFinding(finding models.Finding) string {
	switch finding.RuleID {
	case "HLT001", "HLT002":
		return "no credible values path is exposed for the missing probe; this finding needs another fix path"
	case "AVL001":
		return "no credible replica-count or autoscaling knob was exposed to render a PodDisruptionBudget safely"
	case "NET001":
		return "no credible networkPolicy values knob was exposed by the chart"
	case "RES001", "RES002":
		return "no credible resources values subtree was exposed by the chart"
	case "SEC003":
		return "no credible readOnlyRootFilesystem values knob was exposed by the chart"
	case "SCL001":
		return "no credible autoscaling values knob was exposed by the chart"
	default:
		return "rule is not supported by the S01 values-first planner"
	}
}

func surfaceExposesPath(surface *models.ValuesSurface, path string) bool {
	if surface == nil || path == "" {
		return false
	}
	if surface.HasPath(path) {
		return true
	}

	for parent := parentValuesPath(path); parent != ""; parent = parentValuesPath(parent) {
		if surface.HasPath(parent) && strings.HasPrefix(surface.PathType(parent), "object") {
			return true
		}
	}

	prefix := path + "."
	for _, candidate := range surface.AllPaths() {
		if strings.HasPrefix(candidate, prefix) {
			return true
		}
	}

	return false
}

func parentValuesPath(path string) string {
	idx := strings.LastIndex(path, ".")
	if idx == -1 {
		return ""
	}
	return path[:idx]
}

func resourceValuesBasePath(templatePath string) (string, bool) {
	switch templatePath {
	case "templates/controller-deployment.yaml", "templates/controller-daemonset.yaml":
		return "controller", true
	case "templates/default-backend-deployment.yaml":
		return "defaultBackend", true
	case "templates/admission-webhooks/job-patch/job-createSecret.yaml":
		return "controller.admissionWebhooks.createSecretJob", true
	case "templates/admission-webhooks/job-patch/job-patchWebhook.yaml":
		return "controller.admissionWebhooks.patchWebhookJob", true
	default:
		return "", false
	}
}

func hasRelatedFinding(findings []models.Finding, path, resource, ruleID string) bool {
	for _, finding := range findings {
		if finding.RuleID == ruleID && finding.Path == path && finding.Resource == resource {
			return true
		}
	}
	return false
}

func sortBundlePlan(plan *BundlePlan) {
	if plan == nil {
		return
	}

	sort.SliceStable(plan.AppliedValuesFixes, func(i, j int) bool {
		left := plan.AppliedValuesFixes[i]
		right := plan.AppliedValuesFixes[j]
		if left.Finding.RuleID != right.Finding.RuleID {
			return left.Finding.RuleID < right.Finding.RuleID
		}
		if left.Finding.Path != right.Finding.Path {
			return left.Finding.Path < right.Finding.Path
		}
		if left.Finding.Resource != right.Finding.Resource {
			return left.Finding.Resource < right.Finding.Resource
		}
		return left.ValuesPath < right.ValuesPath
	})

	sort.SliceStable(plan.KustomizePatches, func(i, j int) bool {
		left := plan.KustomizePatches[i]
		right := plan.KustomizePatches[j]
		if left.Finding.RuleID != right.Finding.RuleID {
			return left.Finding.RuleID < right.Finding.RuleID
		}
		if left.Finding.Path != right.Finding.Path {
			return left.Finding.Path < right.Finding.Path
		}
		if left.Finding.Resource != right.Finding.Resource {
			return left.Finding.Resource < right.Finding.Resource
		}
		if left.Target.Kind != right.Target.Kind {
			return left.Target.Kind < right.Target.Kind
		}
		if left.Target.Name != right.Target.Name {
			return left.Target.Name < right.Target.Name
		}
		return left.ContainerName < right.ContainerName
	})

	sort.SliceStable(plan.PendingFindings, func(i, j int) bool {
		left := plan.PendingFindings[i]
		right := plan.PendingFindings[j]
		if left.Finding.RuleID != right.Finding.RuleID {
			return left.Finding.RuleID < right.Finding.RuleID
		}
		if left.Finding.Path != right.Finding.Path {
			return left.Finding.Path < right.Finding.Path
		}
		if left.Finding.Resource != right.Finding.Resource {
			return left.Finding.Resource < right.Finding.Resource
		}
		return left.Reason < right.Reason
	})

	sort.SliceStable(plan.AdvisoryFindings, func(i, j int) bool {
		left := plan.AdvisoryFindings[i]
		right := plan.AdvisoryFindings[j]
		if left.Finding.RuleID != right.Finding.RuleID {
			return left.Finding.RuleID < right.Finding.RuleID
		}
		if left.Finding.Path != right.Finding.Path {
			return left.Finding.Path < right.Finding.Path
		}
		if left.Finding.Resource != right.Finding.Resource {
			return left.Finding.Resource < right.Finding.Resource
		}
		return left.Explanation < right.Explanation
	})
}

func validateBundlePlan(plan BundlePlan) error {
	for _, fix := range plan.AppliedValuesFixes {
		if fix.ValuesPath == "" {
			return fmt.Errorf("applied fix for %s is missing a values path", fix.Finding.RuleID)
		}
	}
	for _, patch := range plan.KustomizePatches {
		if patch.Target.APIVersion == "" || patch.Target.Kind == "" || patch.Target.Name == "" {
			return fmt.Errorf("kustomize patch for %s is missing target resource identity", patch.Finding.RuleID)
		}
		if patch.Finding.RuleID == "HLT001" || patch.Finding.RuleID == "HLT002" {
			if patch.ContainerName == "" {
				return fmt.Errorf("kustomize patch for %s is missing a target container name", patch.Finding.RuleID)
			}
		}
		if len(patch.Patch) == 0 {
			return fmt.Errorf("kustomize patch for %s is missing patch content", patch.Finding.RuleID)
		}
	}
	for _, advisory := range plan.AdvisoryFindings {
		if strings.TrimSpace(advisory.Explanation) == "" {
			return fmt.Errorf("advisory finding for %s is missing an explanation", advisory.Finding.RuleID)
		}
	}
	return nil
}
