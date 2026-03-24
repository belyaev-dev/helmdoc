package fix

import "github.com/belyaev-dev/helmdoc/pkg/models"

// BundlePlan carries the values-first output plus any second-stage Kustomize fallbacks.
type BundlePlan struct {
	AppliedValuesFixes []AppliedValuesFix
	KustomizePatches   []KustomizePatch
	AdvisoryFindings   []AdvisoryFinding
	PendingFindings    []PendingFinding
}

// AppliedValuesFix records one finding that can be addressed through values overrides.
type AppliedValuesFix struct {
	Finding    models.Finding
	ValuesPath string
	Value      any
	Summary    string
}

// ResourceRef identifies one exact rendered resource for Kustomize targeting.
type ResourceRef struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
}

// KustomizePatch records one finding that can be addressed through a strategic-merge patch.
type KustomizePatch struct {
	Finding       models.Finding
	Target        ResourceRef
	ContainerName string
	Patch         map[string]any
	Summary       string
}

// AdvisoryFinding records a finding helmdoc intentionally leaves as guidance-only.
type AdvisoryFinding struct {
	Finding     models.Finding
	Explanation string
}

// PendingFinding records a finding that remains visible but needs a later fix path.
type PendingFinding struct {
	Finding models.Finding
	Reason  string
}

// HasAppliedValuesFixes reports whether the bundle contains at least one values-backed fix.
func (p BundlePlan) HasAppliedValuesFixes() bool {
	return len(p.AppliedValuesFixes) > 0
}

// HasKustomizePatches reports whether the bundle contains at least one Kustomize patch plan.
func (p BundlePlan) HasKustomizePatches() bool {
	return len(p.KustomizePatches) > 0
}

// HasAdvisoryFindings reports whether the bundle includes advisory-only findings.
func (p BundlePlan) HasAdvisoryFindings() bool {
	return len(p.AdvisoryFindings) > 0
}

// HasPendingFindings reports whether the bundle intentionally deferred any findings.
func (p BundlePlan) HasPendingFindings() bool {
	return len(p.PendingFindings) > 0
}
