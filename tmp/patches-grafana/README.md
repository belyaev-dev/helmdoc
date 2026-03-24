# Helmdoc fix bundle

This bundle keeps Helm values overrides separate from Kustomize fallbacks so you can review what helmdoc can apply automatically, what still needs manual work, and what remains advisory-only.

## How to apply this bundle

1. Review the explanations below before copying any generated changes into your deployment workflow.
2. Apply `values-overrides.yaml` with your normal Helm process, for example `helm upgrade --install RELEASE CHART -f values-overrides.yaml`.
3. No Kustomize overlay was needed for this bundle.
4. Follow the advisory-only and still-pending sections for findings that helmdoc intentionally left for manual review.

## Applied values fixes (1)
- `NET001` for `Deployment/helmdoc-grafana` @ `templates/deployment.yaml`
  - Values path: `imageRenderer.networkPolicy.enabled`
  - Helmdoc change: Enable the only exposed chart networkPolicy knob.
  - Rule detail: Deployment/helmdoc-grafana is rendered in namespace "default", but no NetworkPolicy is rendered for that namespace.
  - Chart hint: Enable a chart networkPolicy setting in values.yaml so namespace "default" renders at least one NetworkPolicy for Deployment/helmdoc-grafana.

## Kustomize patches (0)
- None.

## Advisory-only findings (1)
- `IMG002` for `Deployment/helmdoc-grafana` @ `templates/deployment.yaml`
  - Why helmdoc left this advisory-only: Pinning by digest requires looking up the correct digest in your container registry, so helmdoc cannot determine it automatically.
  - Rule detail: container "grafana" in Deployment/helmdoc-grafana uses image "docker.io/grafana/grafana:12.3.1" without a pinned digest.
  - Manual follow-up: Pin container "grafana" to an immutable digest (for example @sha256:...) in the template or values.yaml.

## Findings pending another fix path (5)
- `AVL001` for `Deployment/helmdoc-grafana` @ `templates/deployment.yaml`
  - Still pending because: availability payload does not know how to populate imageRenderer.autoscaling.minReplicas; rule AVL001 has no supported Kustomize default for Deployment/helmdoc-grafana @ templates/deployment.yaml
  - Rule detail: Deployment/helmdoc-grafana has no matching PodDisruptionBudget in namespace "default".
  - Manual follow-up: Render a PodDisruptionBudget whose selector matches Deployment/helmdoc-grafana in namespace "default".
- `RES001` for `Deployment/helmdoc-grafana` @ `templates/deployment.yaml`
  - Still pending because: no credible resources values subtree was exposed by the chart; rule RES001 has no supported Kustomize default for Deployment/helmdoc-grafana @ templates/deployment.yaml
  - Rule detail: container "grafana" in Deployment/helmdoc-grafana does not define resource limits.
  - Manual follow-up: Add resources.limits directly to container "grafana" in the template, or expose a values.yaml path for it.
- `RES002` for `Deployment/helmdoc-grafana` @ `templates/deployment.yaml`
  - Still pending because: no credible resources values subtree was exposed by the chart; rule RES002 has no supported Kustomize default for Deployment/helmdoc-grafana @ templates/deployment.yaml
  - Rule detail: container "grafana" in Deployment/helmdoc-grafana does not define resource requests.
  - Manual follow-up: Add resources.requests directly to container "grafana" in the template, or expose a values.yaml path for it.
- `SCL001` for `Deployment/helmdoc-grafana` @ `templates/deployment.yaml`
  - Still pending because: no credible autoscaling values knob was exposed by the chart; rule SCL001 has no supported Kustomize default for Deployment/helmdoc-grafana @ templates/deployment.yaml
  - Rule detail: Deployment/helmdoc-grafana has no matching HorizontalPodAutoscaler in namespace "default".
  - Manual follow-up: Render a HorizontalPodAutoscaler with scaleTargetRef matching Deployment/helmdoc-grafana in namespace "default".
- `SEC003` for `Deployment/helmdoc-grafana` @ `templates/deployment.yaml`
  - Still pending because: no credible readOnlyRootFilesystem values knob was exposed by the chart; rule SEC003 has no supported Kustomize default for Deployment/helmdoc-grafana @ templates/deployment.yaml
  - Rule detail: container "grafana" in Deployment/helmdoc-grafana does not set readOnlyRootFilesystem: true.
  - Manual follow-up: Set container "grafana" securityContext.readOnlyRootFilesystem to true.
