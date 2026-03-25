#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)
DIST_DIR="$REPO_ROOT/dist"
FORMULA_PATH="$DIST_DIR/homebrew/Formula/helmdoc.rb"
TMP_ROOT="$REPO_ROOT/tmp/verify-release-snapshot"
RELEASE_GOCACHE="$TMP_ROOT/gocache"
RELEASE_GOTMPDIR="$TMP_ROOT/gotmp"
RELEASE_TMPDIR="$TMP_ROOT/tmpdir"
GORELEASER_VERSION="v2.14.3"
GORELEASER_CMD=(go run github.com/goreleaser/goreleaser/v2@${GORELEASER_VERSION})

log() {
  printf 'verify-release-snapshot: %s\n' "$*"
}

fail() {
  printf 'verify-release-snapshot: %s\n' "$*" >&2
  exit 1
}

require_file() {
  local path=$1
  if [[ ! -f "$path" ]]; then
    fail "expected artifact file missing: $path"
  fi
}

run_goreleaser() {
  (
    cd "$REPO_ROOT"
    GOCACHE="$RELEASE_GOCACHE" GOTMPDIR="$RELEASE_GOTMPDIR" TMPDIR="$RELEASE_TMPDIR" "${GORELEASER_CMD[@]}" "$@"
  )
}

mkdir -p "$RELEASE_GOCACHE" "$RELEASE_GOTMPDIR" "$RELEASE_TMPDIR"
host_goos=$(cd "$REPO_ROOT" && go env GOOS)
host_goarch=$(cd "$REPO_ROOT" && go env GOARCH)
short_commit=$(cd "$REPO_ROOT" && git rev-parse --short HEAD)
snapshot_version="0.0.0-snapshot-${short_commit}"
native_binary_pattern="*_${host_goos}_${host_goarch}*/helmdoc"
native_image_tag="ghcr.io/belyaev-dev/helmdoc:${snapshot_version}-${host_goarch}"

log "resetting the shared Go build cache to reclaim space before cross-platform snapshot builds"
(
  cd "$REPO_ROOT"
  go clean -cache
)

log "validating .goreleaser.yaml"
run_goreleaser check .goreleaser.yaml

log "building snapshot artifacts into dist/ via GoReleaser ${GORELEASER_VERSION}"
run_goreleaser release --snapshot --clean --verbose

log "artifact files"
find "$DIST_DIR" -maxdepth 3 -type f | sort | sed 's#^#verify-release-snapshot: artifact #'

require_file "$DIST_DIR/checksums.txt"
require_file "$FORMULA_PATH"

native_binary=$(find "$DIST_DIR" -type f -path "$native_binary_pattern" | head -n 1)
if [[ -z "$native_binary" ]]; then
  fail "native snapshot binary not found with pattern ${native_binary_pattern}"
fi
log "native binary ${native_binary}"

log "smoke-running native binary version command"
if ! native_version_output=$("$native_binary" version 2>&1); then
  fail "native binary version smoke failed: ${native_version_output}"
fi
printf '%s\n' "$native_version_output" | sed 's#^#verify-release-snapshot: native-version #'

log "smoke-running native binary scan on testdata/nginx-ingress"
if ! native_scan_output=$("$native_binary" scan "$REPO_ROOT/testdata/nginx-ingress" --min-score B 2>&1); then
  fail "native binary scan smoke failed: ${native_scan_output}"
fi
printf '%s\n' "$native_scan_output" | sed 's#^#verify-release-snapshot: native-scan #'

log "verifying Docker image ${native_image_tag}"
if ! docker image inspect "$native_image_tag" >/dev/null 2>&1; then
  fail "expected Docker image missing: ${native_image_tag}"
fi

log "smoke-running Docker image version command"
if ! docker_version_output=$(docker run --rm "$native_image_tag" version 2>&1); then
  fail "docker version smoke failed for ${native_image_tag}: ${docker_version_output}"
fi
printf '%s\n' "$docker_version_output" | sed 's#^#verify-release-snapshot: docker-version #'

log "smoke-running Docker image scan on mounted nginx-ingress fixture"
if ! docker_scan_output=$(docker run --rm -v "$REPO_ROOT/testdata:/fixtures:ro" "$native_image_tag" scan /fixtures/nginx-ingress --min-score B 2>&1); then
  fail "docker scan smoke failed for ${native_image_tag}: ${docker_scan_output}"
fi
printf '%s\n' "$docker_scan_output" | sed 's#^#verify-release-snapshot: docker-scan #'

log "snapshot release verification passed"
