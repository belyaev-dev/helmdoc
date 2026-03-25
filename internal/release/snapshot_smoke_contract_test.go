package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSnapshotReleaseSmokeContract(t *testing.T) {
	scriptPath := filepath.Join("..", "..", "scripts", "verify-release-snapshot.sh")
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", scriptPath, err)
	}
	scriptRaw := string(scriptBytes)

	for _, want := range []string{
		`assert_output_contains() {`,
		`scan "$REPO_ROOT/testdata/nginx-ingress" --min-score B`,
		`assert_output_contains "native scan output" "$native_scan_output" "Overall:" "Score:" "Total findings:"`,
		`printf '%s\n' "$native_scan_output" | sed 's#^#verify-release-snapshot: native-scan #'`,
		`native_fix_dir="$TMP_ROOT/native-fix"`,
		`fix "$REPO_ROOT/testdata/nginx-ingress" --output-dir "$native_fix_dir"`,
		`printf '%s\n' "$native_fix_output" | sed 's#^#verify-release-snapshot: native-fix #'`,
		`require_file "$native_fix_dir/values-overrides.yaml"`,
		`require_file "$native_fix_dir/README.md"`,
		`docker run --rm -v "$REPO_ROOT/testdata:/fixtures:ro" "$native_image_tag" scan /fixtures/nginx-ingress --min-score B 2>&1`,
		`assert_output_contains "docker scan output" "$docker_scan_output" "Overall:" "Score:" "Total findings:"`,
		`printf '%s\n' "$docker_scan_output" | sed 's#^#verify-release-snapshot: docker-scan #'`,
		`docker_fix_dir="$TMP_ROOT/docker-fix"`,
		`docker run --rm -v "$REPO_ROOT/testdata:/fixtures:ro" -v "$docker_fix_dir:/output" "$native_image_tag" fix /fixtures/nginx-ingress --output-dir /output 2>&1`,
		`printf '%s\n' "$docker_fix_output" | sed 's#^#verify-release-snapshot: docker-fix #'`,
		`require_file "$docker_fix_dir/values-overrides.yaml"`,
		`require_file "$docker_fix_dir/README.md"`,
	} {
		if !strings.Contains(scriptRaw, want) {
			t.Fatalf("verify-release-snapshot.sh missing %q", want)
		}
	}

	assertSubstringOrder(t, scriptRaw,
		`scan "$REPO_ROOT/testdata/nginx-ingress" --min-score B`,
		`printf '%s\n' "$native_scan_output" | sed 's#^#verify-release-snapshot: native-scan #'`,
		`assert_output_contains "native scan output" "$native_scan_output" "Overall:" "Score:" "Total findings:"`,
	)
	assertSubstringOrder(t, scriptRaw,
		`docker run --rm -v "$REPO_ROOT/testdata:/fixtures:ro" "$native_image_tag" scan /fixtures/nginx-ingress --min-score B 2>&1`,
		`printf '%s\n' "$docker_scan_output" | sed 's#^#verify-release-snapshot: docker-scan #'`,
		`assert_output_contains "docker scan output" "$docker_scan_output" "Overall:" "Score:" "Total findings:"`,
	)
}

func assertSubstringOrder(t *testing.T, text string, ordered ...string) {
	t.Helper()

	lastIndex := -1
	for _, want := range ordered {
		index := strings.Index(text, want)
		if index == -1 {
			t.Fatalf("text missing %q while checking order", want)
		}
		if index <= lastIndex {
			t.Fatalf("substring %q appeared out of order after %q", want, ordered)
		}
		lastIndex = index
	}
}
