# Helmdoc fix bundle

This bundle keeps Helm values overrides separate from Kustomize fallbacks so you can review what helmdoc can apply automatically, what still needs manual work, and what remains advisory-only.

## How to apply this bundle

1. Review the explanations below before copying any generated changes into your deployment workflow.
2. Apply `values-overrides.yaml` with your normal Helm process, for example `helm upgrade --install RELEASE CHART -f values-overrides.yaml`.
3. If you use the generated `kustomize/` overlay, render the chart with the values overrides first and then apply `kustomize/kustomization.yaml` so the patch files layer on top of the rendered manifests.

## Applied values fixes (9)
- `AVL001` for `Deployment/helmdoc-ingress-nginx-controller` @ `templates/controller-deployment.yaml`
  - Values path: `controller.autoscaling.minReplicas`
  - Helmdoc change: Raise controller autoscaling.minReplicas so the chart can render a controller PodDisruptionBudget.
  - Rule detail: Deployment/helmdoc-ingress-nginx-controller has no matching PodDisruptionBudget in namespace "default".
  - Chart hint: Render a PodDisruptionBudget whose selector matches Deployment/helmdoc-ingress-nginx-controller in namespace "default".
- `NET001` for `Deployment/helmdoc-ingress-nginx-controller` @ `templates/controller-deployment.yaml`
  - Values path: `controller.networkPolicy.enabled`
  - Helmdoc change: Enable the controller networkPolicy switch.
  - Rule detail: Deployment/helmdoc-ingress-nginx-controller is rendered in namespace "default", but no NetworkPolicy is rendered for that namespace.
  - Chart hint: Enable a chart networkPolicy setting in values.yaml so namespace "default" renders at least one NetworkPolicy for Deployment/helmdoc-ingress-nginx-controller.
- `RES001` for `Job/helmdoc-ingress-nginx-admission-create` @ `templates/admission-webhooks/job-patch/job-createSecret.yaml`
  - Values path: `controller.admissionWebhooks.createSecretJob.resources.limits`
  - Helmdoc change: Populate resource limits through the chart values surface.
  - Rule detail: container "create" in Job/helmdoc-ingress-nginx-admission-create does not define resource limits.
  - Chart hint: Set controller.admissionWebhooks.createSecretJob.resources.limits in values.yaml. The chart already exposes controller.admissionWebhooks.createSecretJob.resources, but its default leaves limits empty.
- `RES001` for `Job/helmdoc-ingress-nginx-admission-patch` @ `templates/admission-webhooks/job-patch/job-patchWebhook.yaml`
  - Values path: `controller.admissionWebhooks.patchWebhookJob.resources.limits`
  - Helmdoc change: Populate resource limits through the chart values surface.
  - Rule detail: container "patch" in Job/helmdoc-ingress-nginx-admission-patch does not define resource limits.
  - Chart hint: Set controller.admissionWebhooks.patchWebhookJob.resources.limits in values.yaml. The chart already exposes controller.admissionWebhooks.patchWebhookJob.resources, but its default leaves limits empty.
- `RES001` for `Deployment/helmdoc-ingress-nginx-controller` @ `templates/controller-deployment.yaml`
  - Values path: `controller.resources.limits`
  - Helmdoc change: Populate resource limits through the chart values surface.
  - Rule detail: container "controller" in Deployment/helmdoc-ingress-nginx-controller does not define resource limits.
  - Chart hint: Set controller.resources.limits in values.yaml. The chart already exposes controller.resources, but its default leaves limits empty.
- `RES002` for `Job/helmdoc-ingress-nginx-admission-create` @ `templates/admission-webhooks/job-patch/job-createSecret.yaml`
  - Values path: `controller.admissionWebhooks.createSecretJob.resources.requests`
  - Helmdoc change: Populate resource requests through the chart values surface.
  - Rule detail: container "create" in Job/helmdoc-ingress-nginx-admission-create does not define resource requests.
  - Chart hint: Set controller.admissionWebhooks.createSecretJob.resources.requests in values.yaml. The chart already exposes controller.admissionWebhooks.createSecretJob.resources, but its default leaves requests empty.
- `RES002` for `Job/helmdoc-ingress-nginx-admission-patch` @ `templates/admission-webhooks/job-patch/job-patchWebhook.yaml`
  - Values path: `controller.admissionWebhooks.patchWebhookJob.resources.requests`
  - Helmdoc change: Populate resource requests through the chart values surface.
  - Rule detail: container "patch" in Job/helmdoc-ingress-nginx-admission-patch does not define resource requests.
  - Chart hint: Set controller.admissionWebhooks.patchWebhookJob.resources.requests in values.yaml. The chart already exposes controller.admissionWebhooks.patchWebhookJob.resources, but its default leaves requests empty.
- `SCL001` for `Deployment/helmdoc-ingress-nginx-controller` @ `templates/controller-deployment.yaml`
  - Values path: `controller.autoscaling`
  - Helmdoc change: Enable controller autoscaling with a conservative baseline.
  - Rule detail: Deployment/helmdoc-ingress-nginx-controller has no matching HorizontalPodAutoscaler in namespace "default".
  - Chart hint: Render a HorizontalPodAutoscaler with scaleTargetRef matching Deployment/helmdoc-ingress-nginx-controller in namespace "default".
- `SEC003` for `Deployment/helmdoc-ingress-nginx-controller` @ `templates/controller-deployment.yaml`
  - Values path: `controller.image.readOnlyRootFilesystem`
  - Helmdoc change: Enable the controller image read-only root filesystem knob.
  - Rule detail: container "controller" in Deployment/helmdoc-ingress-nginx-controller does not set readOnlyRootFilesystem: true.
  - Chart hint: Set container "controller" securityContext.readOnlyRootFilesystem to true.

## Kustomize patches (4)
- `HLT001` for `Job/helmdoc-ingress-nginx-admission-create` @ `templates/admission-webhooks/job-patch/job-createSecret.yaml`
  - Patch file: `kustomize/hlt001-job-helmdoc-ingress-nginx-admission-create.yaml`
  - Target resource: `Job/helmdoc-ingress-nginx-admission-create`
  - Helmdoc change: Add a conservative livenessProbe directly to the rendered workload container.
  - Target container: `create`
  - Rule detail: container "create" in Job/helmdoc-ingress-nginx-admission-create has no livenessProbe.
  - Manual follow-up: Add livenessProbe directly to the template for container "create" or expose controller.admissionWebhooks.createSecretJob.livenessProbe in values.yaml.
- `HLT001` for `Job/helmdoc-ingress-nginx-admission-patch` @ `templates/admission-webhooks/job-patch/job-patchWebhook.yaml`
  - Patch file: `kustomize/hlt001-job-helmdoc-ingress-nginx-admission-patch.yaml`
  - Target resource: `Job/helmdoc-ingress-nginx-admission-patch`
  - Helmdoc change: Add a conservative livenessProbe directly to the rendered workload container.
  - Target container: `patch`
  - Rule detail: container "patch" in Job/helmdoc-ingress-nginx-admission-patch has no livenessProbe.
  - Manual follow-up: Add livenessProbe directly to the template for container "patch" or expose controller.admissionWebhooks.patchWebhookJob.livenessProbe in values.yaml.
- `HLT002` for `Job/helmdoc-ingress-nginx-admission-create` @ `templates/admission-webhooks/job-patch/job-createSecret.yaml`
  - Patch file: `kustomize/hlt002-job-helmdoc-ingress-nginx-admission-create.yaml`
  - Target resource: `Job/helmdoc-ingress-nginx-admission-create`
  - Helmdoc change: Add a conservative readinessProbe directly to the rendered workload container.
  - Target container: `create`
  - Rule detail: container "create" in Job/helmdoc-ingress-nginx-admission-create has no readinessProbe.
  - Manual follow-up: Add readinessProbe directly to the template for container "create" or expose controller.admissionWebhooks.createSecretJob.readinessProbe in values.yaml.
- `HLT002` for `Job/helmdoc-ingress-nginx-admission-patch` @ `templates/admission-webhooks/job-patch/job-patchWebhook.yaml`
  - Patch file: `kustomize/hlt002-job-helmdoc-ingress-nginx-admission-patch.yaml`
  - Target resource: `Job/helmdoc-ingress-nginx-admission-patch`
  - Helmdoc change: Add a conservative readinessProbe directly to the rendered workload container.
  - Target container: `patch`
  - Rule detail: container "patch" in Job/helmdoc-ingress-nginx-admission-patch has no readinessProbe.
  - Manual follow-up: Add readinessProbe directly to the template for container "patch" or expose controller.admissionWebhooks.patchWebhookJob.readinessProbe in values.yaml.

## Advisory-only findings (0)
- None.

## Findings pending another fix path (0)
- None.
