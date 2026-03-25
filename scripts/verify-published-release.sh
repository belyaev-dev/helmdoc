#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
INSTALLER_SCRIPT="$REPO_ROOT/scripts/install-helmdoc-release.sh"
DEFAULT_CHART_PATH="$REPO_ROOT/testdata/nginx-ingress"
DEFAULT_INSTALL_KIND="all"
DEFAULT_VERSION="latest"
BREW_FORMULA="belyaev-dev/homebrew-tap/helmdoc"
DOCKER_IMAGE_REPOSITORY="ghcr.io/belyaev-dev/helmdoc"
MODULE_PATH=$(awk '/^module / { print $2; exit }' "$REPO_ROOT/go.mod")
TMP_ROOT=${TMPDIR:-/tmp}
WORK_ROOT=$(mktemp -d "$TMP_ROOT/helmdoc-published-release.XXXXXX")
GO_WORK_ROOT="$WORK_ROOT/go-install"
trap 'rm -rf "$WORK_ROOT"' EXIT

log() {
  printf 'verify-published-release: %s\n' "$*"
}

fail() {
  printf 'verify-published-release: %s\n' "$*" >&2
  exit 1
}

require_file() {
  local path=$1
  if [[ ! -f "$path" ]]; then
    fail "expected artifact file missing: $path"
  fi
}

assert_output_contains() {
  local label=$1
  local output=$2
  shift 2

  local marker
  for marker in "$@"; do
    if [[ "$output" != *"$marker"* ]]; then
      fail "${label} missing report marker ${marker}"
    fi
  done
}

print_output() {
  local install_kind=$1
  local phase=$2
  local output=${3:-}
  if [[ -n "$output" ]]; then
    printf '%s\n' "$output" | sed "s#^#verify-published-release: ${install_kind}: ${phase} #"
  fi
}

usage() {
  cat <<'EOF'
Usage: scripts/verify-published-release.sh [--version <tag-or-latest>] [--install-kind <all|binary|docker|go-install|brew>] [--chart-path <path>]

Smoke-test the published helmdoc install surfaces against a released tag.

Options:
  --version <value>       Release tag to verify (default: latest)
  --install-kind <value>  Install surface to verify: all, binary, docker, go-install, or brew (default: all)
  --chart-path <path>     Helm chart to scan (default: ./testdata/nginx-ingress)
  --help                  Show this help text
EOF
}

require_command() {
  local command_name=$1
  if ! command -v "$command_name" >/dev/null 2>&1; then
    fail "required command missing: ${command_name}"
  fi
}

normalize_requested_version() {
  local raw=$1
  local trimmed=${raw//[[:space:]]/}
  if [[ -z "$trimmed" ]]; then
    printf '%s\n' "$DEFAULT_VERSION"
    return
  fi
  printf '%s\n' "$trimmed"
}

normalize_install_kind() {
  local raw=${1:-$DEFAULT_INSTALL_KIND}
  case "$raw" in
    all|binary|docker|go-install|brew)
      printf '%s\n' "$raw"
      ;;
    *)
      fail "unsupported --install-kind ${raw}; expected all, binary, docker, go-install, or brew"
      ;;
  esac
}

normalize_chart_path() {
  local raw=${1:-$DEFAULT_CHART_PATH}
  if [[ -z "$raw" ]]; then
    raw=$DEFAULT_CHART_PATH
  fi
  if [[ "$raw" = /* ]]; then
    printf '%s\n' "$raw"
    return
  fi
  printf '%s\n' "$REPO_ROOT/$raw"
}

resolve_latest_tag() {
  local response
  response=$(curl -fsSL --retry 3 --retry-all-errors -H 'Accept: application/vnd.github+json' https://api.github.com/repos/belyaev-dev/helmdoc/releases/latest 2>/dev/null) || return 1
  python3 -c 'import json, sys; data = json.load(sys.stdin); print(data.get("tag_name", ""))' <<<"$response"
}

resolve_release_tag() {
  local requested=$1
  if [[ "$requested" != "latest" ]]; then
    printf '%s\n' "$requested"
    return
  fi

  local resolved
  resolved=$(resolve_latest_tag || true)
  if [[ -n "$resolved" ]]; then
    printf '%s\n' "$resolved"
    return
  fi

  fail 'unable to resolve latest published release tag from GitHub'
}

derive_release_version() {
  local release_tag=$1
  printf '%s\n' "${release_tag#v}"
}

host_platform() {
  local os arch
  os=$(go env GOOS 2>/dev/null || uname -s | tr '[:upper:]' '[:lower:]')
  arch=$(go env GOARCH 2>/dev/null || uname -m)
  case "$arch" in
    x86_64) arch=amd64 ;;
    aarch64) arch=arm64 ;;
  esac
  printf '%s/%s\n' "$os" "$arch"
}

run_command() {
  local install_kind=$1
  local release_tag=$2
  local description=$3
  shift 3

  log "install kind ${install_kind}"
  log "resolved release tag ${release_tag}"
  log "host platform $(host_platform)"
  log "command ${description}"

  local output
  if ! output=$("$@" 2>&1); then
    fail "install kind ${install_kind} tag ${release_tag} command failed (${description}): ${output}"
  fi
  print_output "$install_kind" command "$output"
}

assert_version_output() {
  local install_kind=$1
  local release_tag=$2
  local output=$3
  # go install builds from source without goreleaser ldflags, so version will be "dev"
  if [[ "$install_kind" == "go-install" ]]; then
    log "skipping version assertion for go-install (no ldflags injection)"
    return 0
  fi
  if [[ "$output" != *"${release_tag}"* ]]; then
    fail "install kind ${install_kind} tag ${release_tag} returned unexpected helmdoc version output: ${output}"
  fi
}

run_version_scan_and_fix() {
  local install_kind=$1
  local release_tag=$2
  local chart_path=$3
  local fix_dir=$4
  shift 4

  local version_output
  log "install kind ${install_kind}"
  log "resolved release tag ${release_tag}"
  log "host platform $(host_platform)"
  log "command $* version"
  if ! version_output=$("$@" version 2>&1); then
    fail "install kind ${install_kind} tag ${release_tag} command failed ($* version): ${version_output}"
  fi
  print_output "$install_kind" version "$version_output"
  assert_version_output "$install_kind" "$release_tag" "$version_output"

  local scan_output
  log "command $* scan ${chart_path} --min-score B"
  if ! scan_output=$("$@" scan "$chart_path" --min-score B 2>&1); then
    fail "install kind ${install_kind} tag ${release_tag} command failed ($* scan ${chart_path} --min-score B): ${scan_output}"
  fi
  print_output "$install_kind" scan "$scan_output"
  assert_output_contains "${install_kind} scan output" "$scan_output" "Overall:" "Score:" "Total findings:"

  rm -rf "$fix_dir"
  mkdir -p "$fix_dir"

  local fix_output
  log "command $* fix ${chart_path} --output-dir ${fix_dir}"
  if ! fix_output=$("$@" fix "$chart_path" --output-dir "$fix_dir" 2>&1); then
    fail "install kind ${install_kind} tag ${release_tag} command failed ($* fix ${chart_path} --output-dir ${fix_dir}): ${fix_output}"
  fi
  print_output "$install_kind" fix "$fix_output"
  require_file "$fix_dir/values-overrides.yaml"
  require_file "$fix_dir/README.md"
}

smoke_binary() {
  local release_tag=$1
  local chart_path=$2
  local github_path_file="$WORK_ROOT/binary-github-path"
  local github_env_file="$WORK_ROOT/binary-github-env"
  local binary_fix_dir="$WORK_ROOT/binary-fix"
  : > "$github_path_file"
  : > "$github_env_file"

  log "install kind binary"
  log "resolved release tag ${release_tag}"
  log "host platform $(host_platform)"
  log "command bash ${INSTALLER_SCRIPT} --version ${release_tag}"
  local install_output
  if ! install_output=$(GITHUB_PATH="$github_path_file" GITHUB_ENV="$github_env_file" bash "$INSTALLER_SCRIPT" --version "$release_tag" 2>&1); then
    fail "install kind binary tag ${release_tag} command failed (bash ${INSTALLER_SCRIPT} --version ${release_tag}): ${install_output}"
  fi
  print_output binary install "$install_output"

  local bin_dir
  bin_dir=$(tail -n 1 "$github_path_file")
  if [[ -z "$bin_dir" ]]; then
    fail "install kind binary tag ${release_tag} did not write a PATH entry via GITHUB_PATH"
  fi
  export PATH="$bin_dir:$PATH"
  run_version_scan_and_fix binary "$release_tag" "$chart_path" "$binary_fix_dir" helmdoc
}

smoke_docker() {
  local release_tag=$1
  local chart_path=$2
  local chart_parent chart_name image_ref image_version docker_fix_dir
  chart_parent=$(dirname "$chart_path")
  chart_name=$(basename "$chart_path")
  image_version=$(derive_release_version "$release_tag")
  image_ref="${DOCKER_IMAGE_REPOSITORY}:${image_version}"
  docker_fix_dir="$WORK_ROOT/docker-fix"

  require_command docker
  log "install kind docker"
  log "resolved release tag ${release_tag}"
  log "derived docker image version ${image_version}"
  log "download target ${image_ref}"
  run_command docker "$release_tag" "docker pull ${image_ref}" docker pull "$image_ref"

  local version_output
  log "command docker run --rm ${image_ref} version"
  if ! version_output=$(docker run --rm "$image_ref" version 2>&1); then
    fail "install kind docker tag ${release_tag} command failed (docker run --rm ${image_ref} version): ${version_output}"
  fi
  print_output docker version "$version_output"
  assert_version_output docker "$release_tag" "$version_output"

  local docker_scan_output
  log "command docker run --rm -v ${chart_parent}:/fixtures:ro ${image_ref} scan /fixtures/${chart_name} --min-score B"
  if ! docker_scan_output=$(docker run --rm -v "${chart_parent}:/fixtures:ro" "$image_ref" scan "/fixtures/${chart_name}" --min-score B 2>&1); then
    fail "install kind docker tag ${release_tag} command failed (docker run --rm -v ${chart_parent}:/fixtures:ro ${image_ref} scan /fixtures/${chart_name} --min-score B): ${docker_scan_output}"
  fi
  print_output docker scan "$docker_scan_output"
  assert_output_contains "docker scan output" "$docker_scan_output" "Overall:" "Score:" "Total findings:"

  rm -rf "$docker_fix_dir"
  mkdir -p "$docker_fix_dir"

  local docker_fix_output
  log "command docker run --rm -v ${chart_parent}:/fixtures:ro -v ${docker_fix_dir}:/output ${image_ref} fix /fixtures/${chart_name} --output-dir /output"
  if ! docker_fix_output=$(docker run --rm -v "${chart_parent}:/fixtures:ro" -v "$docker_fix_dir:/output" "$image_ref" fix "/fixtures/${chart_name}" --output-dir /output 2>&1); then
    fail "install kind docker tag ${release_tag} command failed (docker run --rm -v ${chart_parent}:/fixtures:ro -v ${docker_fix_dir}:/output ${image_ref} fix /fixtures/${chart_name} --output-dir /output): ${docker_fix_output}"
  fi
  print_output docker fix "$docker_fix_output"
  require_file "$docker_fix_dir/values-overrides.yaml"
  require_file "$docker_fix_dir/README.md"
}

smoke_go_install() {
  local release_tag=$1
  local chart_path=$2
  local gobin="$GO_WORK_ROOT/bin"
  local gocache="$GO_WORK_ROOT/cache"
  local gotmp="$GO_WORK_ROOT/tmp"
  local gopath="$GO_WORK_ROOT/path"
  local go_install_fix_dir="$WORK_ROOT/go-install-fix"
  mkdir -p "$gobin" "$gocache" "$gotmp" "$gopath"

  require_command go
  log "install kind go-install"
  log "resolved release tag ${release_tag}"
  log "download target ${MODULE_PATH}@${release_tag}"
  run_command go-install "$release_tag" "go install ${MODULE_PATH}@${release_tag}" env GOBIN="$gobin" GOCACHE="$gocache" GOTMPDIR="$gotmp" GOPATH="$gopath" go install "${MODULE_PATH}@${release_tag}"
  run_version_scan_and_fix go-install "$release_tag" "$chart_path" "$go_install_fix_dir" "$gobin/helmdoc"
}

smoke_brew() {
  local release_tag=$1
  local chart_path=$2
  local brew_fix_dir="$WORK_ROOT/brew-fix"

  require_command brew
  log "install kind brew"
  log "resolved release tag ${release_tag}"
  log "download target ${BREW_FORMULA}"
  run_command brew "$release_tag" "brew tap belyaev-dev/homebrew-tap" brew tap belyaev-dev/homebrew-tap
  if brew list --versions helmdoc >/dev/null 2>&1; then
    run_command brew "$release_tag" "brew upgrade --formula ${BREW_FORMULA} || brew reinstall ${BREW_FORMULA}" bash -lc "brew upgrade --formula '${BREW_FORMULA}' || brew reinstall '${BREW_FORMULA}'"
  else
    run_command brew "$release_tag" "brew install ${BREW_FORMULA}" brew install "$BREW_FORMULA"
  fi
  run_version_scan_and_fix brew "$release_tag" "$chart_path" "$brew_fix_dir" helmdoc
}

requested_version=$DEFAULT_VERSION
install_kind=$DEFAULT_INSTALL_KIND
chart_path=$DEFAULT_CHART_PATH
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || fail '--version requires a value'
      requested_version=$2
      shift 2
      ;;
    --install-kind)
      [[ $# -ge 2 ]] || fail '--install-kind requires a value'
      install_kind=$2
      shift 2
      ;;
    --chart-path)
      [[ $# -ge 2 ]] || fail '--chart-path requires a value'
      chart_path=$2
      shift 2
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      fail "unknown argument: $1"
      ;;
  esac
done

requested_version=$(normalize_requested_version "$requested_version")
install_kind=$(normalize_install_kind "$install_kind")
chart_path=$(normalize_chart_path "$chart_path")
resolved_tag=$(resolve_release_tag "$requested_version")

if [[ ! -d "$chart_path" ]]; then
  fail "chart path does not exist: ${chart_path}"
fi
if [[ ! -f "$chart_path/Chart.yaml" ]]; then
  fail "chart path is missing Chart.yaml: ${chart_path}"
fi

require_command curl
require_command python3

case "$install_kind" in
  all)
    smoke_binary "$resolved_tag" "$chart_path"
    smoke_docker "$resolved_tag" "$chart_path"
    smoke_go_install "$resolved_tag" "$chart_path"
    smoke_brew "$resolved_tag" "$chart_path"
    ;;
  binary)
    smoke_binary "$resolved_tag" "$chart_path"
    ;;
  docker)
    smoke_docker "$resolved_tag" "$chart_path"
    ;;
  go-install)
    smoke_go_install "$resolved_tag" "$chart_path"
    ;;
  brew)
    smoke_brew "$resolved_tag" "$chart_path"
    ;;
esac

log "published release verification passed"
