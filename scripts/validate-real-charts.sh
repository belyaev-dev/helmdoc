#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
REPO_ROOT=$(CDPATH= cd -- "$SCRIPT_DIR/.." && pwd)

assert_contains() {
  local haystack=$1
  local needle=$2
  local context=$3
  if [[ "$haystack" != *"$needle"* ]]; then
    printf 'validate-real-charts: %s missing %q\n' "$context" "$needle" >&2
    exit 1
  fi
}

run_chart_checks() {
  local fixture_id=$1
  local chart_path=$2
  local chart_name=$3
  local chart_version=$4
  local grade=$5
  local score=$6
  local total_findings=$7
  local anchor_rule=$8
  local anchor_title=$9

  printf '==> %s (text)\n' "$fixture_id"
  local text_output
  if ! text_output=$(cd "$REPO_ROOT" && go run . scan "$chart_path" 2>&1); then
    printf 'validate-real-charts: %s text scan failed\n%s\n' "$fixture_id" "$text_output" >&2
    exit 1
  fi

  assert_contains "$text_output" "Chart: ${chart_name}@${chart_version}" "$fixture_id text"
  assert_contains "$text_output" "Overall: ${grade}" "$fixture_id text"
  assert_contains "$text_output" "Score: ${score}/100" "$fixture_id text"
  assert_contains "$text_output" "Total findings: ${total_findings}" "$fixture_id text"
  assert_contains "$text_output" "[${anchor_rule}]" "$fixture_id text"
  assert_contains "$text_output" "Title: ${anchor_title}" "$fixture_id text"

  printf '==> %s (json)\n' "$fixture_id"
  local json_output
  if ! json_output=$(cd "$REPO_ROOT" && go run . scan "$chart_path" --output json 2>&1); then
    printf 'validate-real-charts: %s json scan failed\n%s\n' "$fixture_id" "$json_output" >&2
    exit 1
  fi

  assert_contains "$json_output" "\"chart_name\": \"${chart_name}\"" "$fixture_id json"
  assert_contains "$json_output" "\"chart_version\": \"${chart_version}\"" "$fixture_id json"
  assert_contains "$json_output" "\"overall_grade\": \"${grade}\"" "$fixture_id json"
  assert_contains "$json_output" "\"total_findings\": ${total_findings}" "$fixture_id json"
  assert_contains "$json_output" "\"rule_id\": \"${anchor_rule}\"" "$fixture_id json"
  assert_contains "$json_output" "\"title\": \"${anchor_title}\"" "$fixture_id json"
}

run_chart_checks "nginx-ingress" "testdata/nginx-ingress" "ingress-nginx" "4.15.1" "B" "84.5" "13" "SEC003" "Container root filesystem is writable"
run_chart_checks "postgresql" "testdata/postgresql.tgz" "postgresql" "16.4.5" "A" "98.9" "2" "IMG002" "Container image is not pinned by digest"
run_chart_checks "grafana" "testdata/grafana.tgz" "grafana" "10.5.15" "A" "92.2" "7" "SEC003" "Container root filesystem is writable"

printf 'validate-real-charts: all real chart CLI checks passed\n'
