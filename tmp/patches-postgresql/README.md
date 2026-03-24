# Helmdoc fix bundle

This bundle keeps Helm values overrides separate from Kustomize fallbacks so you can review what helmdoc can apply automatically, what still needs manual work, and what remains advisory-only.

## How to apply this bundle

1. Review the explanations below before copying any generated changes into your deployment workflow.
2. Apply `values-overrides.yaml` with your normal Helm process, for example `helm upgrade --install RELEASE CHART -f values-overrides.yaml`.
3. No Kustomize overlay was needed for this bundle.
4. Follow the advisory-only and still-pending sections for findings that helmdoc intentionally left for manual review.

## Applied values fixes (0)
- None.

## Kustomize patches (0)
- None.

## Advisory-only findings (1)
- `IMG002` for `StatefulSet/helmdoc-postgresql` @ `templates/primary/statefulset.yaml`
  - Why helmdoc left this advisory-only: Pinning by digest requires looking up the correct digest in your container registry, so helmdoc cannot determine it automatically.
  - Rule detail: container "postgresql" in StatefulSet/helmdoc-postgresql uses image "docker.io/bitnami/postgresql:17.2.0-debian-12-r8" without a pinned digest.
  - Manual follow-up: Pin container "postgresql" to an immutable digest (for example @sha256:...) in the template or values.yaml.

## Findings pending another fix path (1)
- `SCL001` for `StatefulSet/helmdoc-postgresql` @ `templates/primary/statefulset.yaml`
  - Still pending because: no credible autoscaling values knob was exposed by the chart; rule SCL001 has no supported Kustomize default for StatefulSet/helmdoc-postgresql @ templates/primary/statefulset.yaml
  - Rule detail: StatefulSet/helmdoc-postgresql has no matching HorizontalPodAutoscaler in namespace "default".
  - Manual follow-up: Render a HorizontalPodAutoscaler with scaleTargetRef matching StatefulSet/helmdoc-postgresql in namespace "default".
