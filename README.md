# helmdoc

`helmdoc` analyzes Helm charts, scores what it finds, and generates reviewable fix bundles.

It is built for the day-one workflow teams actually need:

- **scan** a chart and get a deterministic score across security, resources, health, storage, availability, network, images, ingress, scaling, and config
- **gate CI** with `--min-score`
- **generate fixes** as Helm values overrides first, then Kustomize fallbacks where the chart does not expose a safe values knob
- **reuse chart-specific knowledge** from the curated launch registry for high-value charts: ingress-nginx, grafana, cert-manager, external-secrets, and redis

## Quick start

```bash
brew install belyaev-dev/homebrew-tap/helmdoc
helmdoc version
helmdoc scan ./charts/my-chart --min-score B
helmdoc fix ./charts/my-chart --output-dir ./tmp/helmdoc-fix
```

## Real scan output from the launch fixture

The excerpt below is the real text report from the pinned `ingress-nginx@4.15.1` fixture used in the release contract tests.

```text
$ helmdoc scan ./testdata/nginx-ingress
HelmDoc scan report
Chart: ingress-nginx@4.15.1

Overall: B
Score: 84.5/100
Total findings: 13

Categories:
- Security: B (85.0/100, weight 3.0, findings 1)
- Resources: D (60.0/100, weight 2.5, findings 5)
- Storage: A (100.0/100, weight 1.5, findings 0)
- Health: D (68.0/100, weight 2.0, findings 4)
- Availability: A (92.0/100, weight 1.0, findings 1)
- Network: A (92.0/100, weight 1.0, findings 1)
- Images: A (100.0/100, weight 1.0, findings 0)
- Ingress: A (100.0/100, weight 1.0, findings 0)
- Scaling: A (92.0/100, weight 1.0, findings 1)
- Config: A (100.0/100, weight 1.0, findings 0)

Findings:
Security findings (1):
- [SEC003][error] Deployment/helmdoc-ingress-nginx-controller @ templates/controller-deployment.yaml
  Title: Container root filesystem is writable
  Description: container "controller" in Deployment/helmdoc-ingress-nginx-controller does not set readOnlyRootFilesystem: true.
  Remediation: Set container "controller" securityContext.readOnlyRootFilesystem to true.
```

## Install

### Homebrew

The Homebrew formula is published from the `belyaev-dev/homebrew-tap` repository.

```bash
brew install belyaev-dev/homebrew-tap/helmdoc
```

### Docker

The published container image lives at `ghcr.io/belyaev-dev/helmdoc` and uses `helmdoc` as its entrypoint. Public Git tags keep the `v` prefix, while published Docker image tags use the stripped GoReleaser version.

```bash
HELMDOC_TAG=v0.1.0
HELMDOC_IMAGE_VERSION="${HELMDOC_TAG#v}"
docker run --rm -v "$PWD:/work" "ghcr.io/belyaev-dev/helmdoc:${HELMDOC_IMAGE_VERSION}" \
  scan ./charts/my-chart --output text --min-score B
```

### go install

```bash
go install github.com/belyaev-dev/helmdoc@latest
```

### Direct binary download

Published archives keep the full git tag in the GitHub Release URL, but archive names use the stripped GoReleaser version:
`helmdoc_${HELMDOC_VERSION}_${OS}_${ARCH}.zip`

```bash
HELMDOC_TAG=v0.1.0
HELMDOC_VERSION="${HELMDOC_TAG#v}"
case "$(uname -s)" in
  Linux) OS=linux ;;
  Darwin) OS=darwin ;;
  *) echo "unsupported OS" >&2; exit 1 ;;
esac
case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  arm64|aarch64) ARCH=arm64 ;;
  *) echo "unsupported architecture" >&2; exit 1 ;;
esac
curl -fsSLO "https://github.com/belyaev-dev/helmdoc/releases/download/${HELMDOC_TAG}/helmdoc_${HELMDOC_VERSION}_${OS}_${ARCH}.zip"
unzip "helmdoc_${HELMDOC_VERSION}_${OS}_${ARCH}.zip"
install -m 0755 helmdoc /usr/local/bin/helmdoc
helmdoc version
```

## Commands

### Scan a chart

Text output is the default:

```bash
helmdoc scan ./charts/my-chart
helmdoc scan ./charts/my-chart --output json
helmdoc scan ./charts/my-chart --config .helmdoc.yaml --min-score B
```

### Generate a fix bundle

`helmdoc fix` writes a reviewable bundle instead of mutating chart sources in place.

```bash
helmdoc fix ./charts/my-chart --output-dir ./tmp/helmdoc-fix
```

A generated bundle contains:

- `values-overrides.yaml` for safe chart-native fixes
- `kustomize/` patches when the chart does not expose the needed knob
- a bundle `README.md` that explains every applied, advisory-only, and pending item

### Print version metadata

```bash
helmdoc version
```

### Generate shell completion

```bash
helmdoc completion bash > /usr/local/etc/bash_completion.d/helmdoc
helmdoc completion zsh > "${fpath[1]}/_helmdoc"
```

## Curated launch registry

The launch registry ships chart-specific remediation knowledge for:

- `ingress-nginx`
- `grafana`
- `cert-manager`
- `external-secrets`
- `redis`

Those curated mappings are what let `helmdoc fix` turn findings into real values overrides such as `controller.networkPolicy.enabled`, `controller.resources`, or chart-native autoscaling and PodDisruptionBudget settings where the chart supports them.

## CI

### GitHub Actions

The repository root publishes a composite action that installs a released binary and then runs `helmdoc scan`.

```yaml
- name: Scan chart with helmdoc
  uses: belyaev-dev/helmdoc@v1
  with:
    chart-path: ./charts/my-chart
    version: latest
    output: text
    min-score: B
    config: .helmdoc.yaml
```

`chart-path` is required. `version`, `output`, `min-score`, and `config` match the public action input contract exactly.

### GitLab CI

Because the published GHCR image already uses `helmdoc` as its entrypoint, override the entrypoint if you want a normal shell-based job script. When pinning a public release tag such as `v0.1.0`, use the stripped image tag (`0.1.0`) for the `image.name` reference.

```yaml
stages:
  - test

variables:
  HELMDOC_IMAGE_VERSION: "0.1.0"

helmdoc_scan:
  stage: test
  image:
    name: ghcr.io/belyaev-dev/helmdoc:${HELMDOC_IMAGE_VERSION}
    entrypoint: [""]
  script:
    - helmdoc version
    - helmdoc scan ./charts/my-chart --output text --min-score B --config .helmdoc.yaml
```

## Why teams use it

- **Deterministic reports** for local runs and CI logs
- **Reviewable fixes** instead of silent chart rewrites
- **Release-backed installs** across Homebrew, Docker, `go install`, direct binaries, and GitHub Actions
- **Registry-backed chart knowledge** for common public charts without requiring networked chart lookups at scan time
