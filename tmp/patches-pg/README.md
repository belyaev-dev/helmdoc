# Helmdoc fix bundle

This bundle proposes values overrides first, then deterministic Kustomize patches for findings that do not expose a credible values knob, without mutating the source chart.

## Applied values fixes (0)
- None.

## Kustomize patches (0)
- None.

## Findings pending another fix path (2)
- `IMG002` for `StatefulSet/helmdoc-postgresql` @ `templates/primary/statefulset.yaml` — rule is not supported by the S01 values-first planner; rule IMG002 has no supported Kustomize default for StatefulSet/helmdoc-postgresql @ templates/primary/statefulset.yaml
- `SCL001` for `StatefulSet/helmdoc-postgresql` @ `templates/primary/statefulset.yaml` — no credible autoscaling values knob was exposed by the chart; rule SCL001 has no supported Kustomize default for StatefulSet/helmdoc-postgresql @ templates/primary/statefulset.yaml
