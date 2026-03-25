#!/usr/bin/env bash
set -euo pipefail

OWNER="belyaev-dev"
REPO="helmdoc"
PROJECT_NAME="helmdoc"
INSTALL_KIND="github-release-archive"

log() {
  printf 'install-helmdoc-release: %s\n' "$*"
}

fail() {
  printf 'install-helmdoc-release: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: scripts/install-helmdoc-release.sh [--version <tag-or-latest>]

Install a published helmdoc GitHub Release archive into PATH.

Options:
  --version <value>  Release tag to install (default: latest)
  --help             Show this help text
EOF
}

require_command() {
  local command_name=$1
  if ! command -v "$command_name" >/dev/null 2>&1; then
    fail "required command missing: ${command_name}"
  fi
}

pick_python() {
  if command -v python3 >/dev/null 2>&1; then
    printf 'python3\n'
    return
  fi
  if command -v python >/dev/null 2>&1; then
    printf 'python\n'
    return
  fi
  fail "python3 or python is required to resolve latest tags and unpack release archives"
}

normalize_requested_version() {
  local raw=$1
  local trimmed=${raw//[[:space:]]/}
  if [[ -z "$trimmed" ]]; then
    printf 'latest\n'
    return
  fi
  printf '%s\n' "$trimmed"
}

resolve_latest_tag_from_api() {
  local python_bin=$1
  local api_url="https://api.github.com/repos/${OWNER}/${REPO}/releases/latest"
  local response
  response=$(curl -fsSL --retry 3 --retry-all-errors -H 'Accept: application/vnd.github+json' "$api_url" 2>/dev/null) || return 1
  printf '%s' "$response" | "$python_bin" -c 'import json, sys; data = json.load(sys.stdin); print(data.get("tag_name", ""))'
}

resolve_latest_tag_from_redirect() {
  local redirect_url
  local redirect_basename
  redirect_url=$(curl -fsSLI --retry 3 --retry-all-errors -o /dev/null -w '%{url_effective}' "https://github.com/${OWNER}/${REPO}/releases/latest" 2>/dev/null) || return 1
  redirect_basename=$(basename "$redirect_url")
  case "$redirect_basename" in
    ""|latest|releases)
      return 1
      ;;
  esac
  printf '%s\n' "$redirect_basename"
}

resolve_tag() {
  local requested=$1
  local python_bin=$2

  if [[ "$requested" != "latest" ]]; then
    printf '%s\n' "$requested"
    return
  fi

  local resolved_tag
  resolved_tag=$(resolve_latest_tag_from_api "$python_bin" || true)
  if [[ -n "$resolved_tag" ]]; then
    printf '%s\n' "$resolved_tag"
    return
  fi

  resolved_tag=$(resolve_latest_tag_from_redirect || true)
  if [[ -n "$resolved_tag" ]]; then
    printf '%s\n' "$resolved_tag"
    return
  fi

  fail "unable to resolve latest release tag from GitHub for ${OWNER}/${REPO}"
}

normalize_os() {
  local raw=$1
  case "$raw" in
    Linux|linux)
      printf 'linux\n'
      ;;
    macOS|MacOS|macos|Darwin|darwin)
      printf 'darwin\n'
      ;;
    Windows|windows|MINGW*|MSYS*|CYGWIN*)
      printf 'windows\n'
      ;;
    *)
      fail "unsupported runner OS ${raw}; expected Linux, macOS, or Windows"
      ;;
  esac
}

normalize_arch() {
  local raw=$1
  case "$raw" in
    X64|x64|AMD64|amd64|x86_64)
      printf 'amd64\n'
      ;;
    ARM64|arm64|AARCH64|aarch64)
      printf 'arm64\n'
      ;;
    *)
      fail "unsupported runner architecture ${raw}; expected X64/AMD64 or ARM64"
      ;;
  esac
}

unpack_zip() {
  local python_bin=$1
  local archive_path=$2
  local extract_dir=$3

  "$python_bin" - "$archive_path" "$extract_dir" <<'PY'
import pathlib
import sys
import zipfile

archive_path = pathlib.Path(sys.argv[1])
extract_dir = pathlib.Path(sys.argv[2])
with zipfile.ZipFile(archive_path) as zf:
    zf.extractall(extract_dir)
PY
}

add_bin_dir_to_path() {
  local bin_dir=$1
  if [[ -n "${GITHUB_PATH:-}" ]]; then
    printf '%s\n' "$bin_dir" >> "$GITHUB_PATH"
  fi
  export PATH="$bin_dir:$PATH"
}

derive_release_version() {
  local resolved_tag=$1
  printf '%s\n' "${resolved_tag#v}"
}

export_observability_env() {
  local resolved_tag=$1
  local archive_version=$2
  local target_os=$3
  local target_arch=$4
  local asset_url=$5

  if [[ -n "${GITHUB_ENV:-}" ]]; then
    {
      printf 'HELMDOC_RELEASE_TAG=%s\n' "$resolved_tag"
      printf 'HELMDOC_RELEASE_ARCHIVE_VERSION=%s\n' "$archive_version"
      printf 'HELMDOC_RELEASE_OS=%s\n' "$target_os"
      printf 'HELMDOC_RELEASE_ARCH=%s\n' "$target_arch"
      printf 'HELMDOC_RELEASE_INSTALL_KIND=%s\n' "$INSTALL_KIND"
      printf 'HELMDOC_RELEASE_ASSET_URL=%s\n' "$asset_url"
    } >> "$GITHUB_ENV"
  fi
}

requested_version="latest"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || fail "--version requires a value"
      requested_version=$2
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

require_command curl
python_bin=$(pick_python)
requested_version=$(normalize_requested_version "$requested_version")
resolved_tag=$(resolve_tag "$requested_version" "$python_bin")
archive_version=$(derive_release_version "$resolved_tag")
raw_os=${RUNNER_OS:-$(uname -s)}
raw_arch=${RUNNER_ARCH:-$(uname -m)}
target_os=$(normalize_os "$raw_os")
target_arch=$(normalize_arch "$raw_arch")
archive_name="${PROJECT_NAME}_${archive_version}_${target_os}_${target_arch}.zip"
asset_url="https://github.com/${OWNER}/${REPO}/releases/download/${resolved_tag}/${archive_name}"
install_parent=${RUNNER_TEMP:-${TMPDIR:-/tmp}}
mkdir -p "$install_parent"
install_root=$(mktemp -d "$install_parent/helmdoc-release.XXXXXX")
archive_path="$install_root/$archive_name"
extract_dir="$install_root/extracted"
bin_dir="$install_root/bin"
mkdir -p "$extract_dir" "$bin_dir"

log "install kind ${INSTALL_KIND}"
log "requested version ${requested_version}"
log "resolved release tag ${resolved_tag}"
log "derived archive version ${archive_version}"
log "runner OS ${raw_os} -> ${target_os}"
log "runner architecture ${raw_arch} -> ${target_arch}"
log "download target ${asset_url}"

if ! curl -fsSL --retry 3 --retry-all-errors --output "$archive_path" "$asset_url"; then
  fail "failed to download ${asset_url} for tag ${resolved_tag} archive version ${archive_version} (${raw_os}/${raw_arch} -> ${target_os}/${target_arch})"
fi

if ! unpack_zip "$python_bin" "$archive_path" "$extract_dir"; then
  fail "failed to unpack ${archive_name} from ${asset_url} for tag ${resolved_tag} archive version ${archive_version}"
fi

binary_name="helmdoc"
if [[ "$target_os" == "windows" ]]; then
  binary_name="helmdoc.exe"
fi
binary_source="$extract_dir/$binary_name"
if [[ ! -f "$binary_source" ]]; then
  fail "release archive ${archive_name} for tag ${resolved_tag} archive version ${archive_version} did not contain expected binary ${binary_name}"
fi

cp "$binary_source" "$bin_dir/$binary_name"
chmod +x "$bin_dir/$binary_name" || true

if [[ "$target_os" == "windows" ]]; then
  cat > "$bin_dir/helmdoc" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
exec "$SCRIPT_DIR/helmdoc.exe" "$@"
EOF
  chmod +x "$bin_dir/helmdoc"
fi

add_bin_dir_to_path "$bin_dir"
export_observability_env "$resolved_tag" "$archive_version" "$target_os" "$target_arch" "$asset_url"

if ! version_output=$(helmdoc version 2>&1); then
  fail "helmdoc version failed after installing tag ${resolved_tag}: ${version_output}"
fi
printf '%s\n' "$version_output" | sed 's#^#install-helmdoc-release: helmdoc version #'
log "installed binary path ${bin_dir}/${binary_name}"
