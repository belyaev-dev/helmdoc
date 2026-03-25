package release

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublishedReleaseSmokeContract(t *testing.T) {
	workflow, raw := loadWorkflowConfig(t, filepath.Join("..", "..", ".github", "workflows", "published-release-smoke.yml"))
	readme := loadREADME(t)

	if workflow.Name != "Published release smoke" {
		t.Fatalf("published release smoke workflow name = %q, want Published release smoke", workflow.Name)
	}

	releaseEvent, ok := workflow.On["release"]
	if !ok {
		t.Fatal("published release smoke workflow missing on.release trigger")
	}
	assertSameStrings(t, "published release smoke release types", releaseEvent.Types, []string{"published"})

	dispatchEvent, ok := workflow.On["workflow_dispatch"]
	if !ok {
		t.Fatal("published release smoke workflow missing on.workflow_dispatch trigger")
	}
	versionInput, ok := dispatchEvent.Inputs["version"]
	if !ok {
		t.Fatalf("workflow_dispatch inputs missing version in %#v", dispatchEvent.Inputs)
	}
	if strings.TrimSpace(versionInput.Description) == "" {
		t.Fatal("workflow_dispatch.inputs.version.description must not be empty")
	}
	if versionInput.Required {
		t.Fatal("workflow_dispatch.inputs.version.required = true, want false")
	}
	if versionInput.Default != "latest" {
		t.Fatalf("workflow_dispatch.inputs.version.default = %q, want latest", versionInput.Default)
	}
	if versionInput.Type != "string" {
		t.Fatalf("workflow_dispatch.inputs.version.type = %q, want string", versionInput.Type)
	}

	if workflow.Permissions["contents"] != "read" {
		t.Fatalf("published release smoke permissions.contents = %q, want read", workflow.Permissions["contents"])
	}
	if len(workflow.Permissions) != 1 {
		t.Fatalf("published release smoke permissions = %#v, want exactly contents: read", workflow.Permissions)
	}
	for _, forbidden := range []string{"secrets.GH_PAT", "secrets.GITHUB_TOKEN", "docker/login-action@", "GH_PAT"} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("published release smoke workflow should use public install paths only; found %q in %q", forbidden, raw)
		}
	}

	direct := requireJob(t, workflow, "published-release-direct")
	if direct.RunsOn != "ubuntu-latest" {
		t.Fatalf("jobs.published-release-direct.runs-on = %q, want ubuntu-latest", direct.RunsOn)
	}
	if !strings.Contains(direct.Name, "${{ matrix.install-kind }}") {
		t.Fatalf("jobs.published-release-direct.name = %q, want matrix install-kind interpolation", direct.Name)
	}
	if direct.Strategy.FailFast {
		t.Fatal("jobs.published-release-direct.strategy.fail-fast = true, want false so all direct install modes report")
	}
	assertSameStrings(t, "published release direct install kinds", direct.Strategy.Matrix["install-kind"], []string{"binary", "docker", "go-install"})
	assertUsesStep(t, direct, "Check out repository", "actions/checkout@v4")
	setupGo := assertUsesStep(t, direct, "Set up Go", "actions/setup-go@v5")
	if setupGo.If != "matrix.install-kind != 'docker'" {
		t.Fatalf("jobs.published-release-direct Set up Go if = %q, want matrix.install-kind != 'docker'", setupGo.If)
	}
	assertWithValue(t, setupGo.With, "go-version-file", "go.mod")
	assertWithValue(t, setupGo.With, "cache", "true")
	runDirect := requireStep(t, direct, "Run published release smoke")
	if runDirect.Env["HELMDOC_RELEASE_VERSION"] == "" {
		t.Fatal("jobs.published-release-direct Run published release smoke must set HELMDOC_RELEASE_VERSION")
	}
	for _, want := range []string{
		"bash scripts/verify-published-release.sh",
		"--version \"${HELMDOC_RELEASE_VERSION}\"",
		"--install-kind \"${{ matrix.install-kind }}\"",
		"--chart-path ./testdata/nginx-ingress",
	} {
		if !strings.Contains(runDirect.Run, want) {
			t.Fatalf("jobs.published-release-direct Run published release smoke missing %q in %q", want, runDirect.Run)
		}
	}

	homebrew := requireJob(t, workflow, "published-release-homebrew")
	if homebrew.RunsOn != "macos-latest" {
		t.Fatalf("jobs.published-release-homebrew.runs-on = %q, want macos-latest", homebrew.RunsOn)
	}
	assertUsesStep(t, homebrew, "Check out repository", "actions/checkout@v4")
	runHomebrew := requireStep(t, homebrew, "Run published release smoke")
	for _, want := range []string{
		"bash scripts/verify-published-release.sh",
		"--version \"${HELMDOC_RELEASE_VERSION}\"",
		"--install-kind brew",
		"--chart-path ./testdata/nginx-ingress",
	} {
		if !strings.Contains(runHomebrew.Run, want) {
			t.Fatalf("jobs.published-release-homebrew Run published release smoke missing %q in %q", want, runHomebrew.Run)
		}
	}

	actionJob := requireJob(t, workflow, "published-release-action")
	if actionJob.RunsOn != "ubuntu-latest" {
		t.Fatalf("jobs.published-release-action.runs-on = %q, want ubuntu-latest", actionJob.RunsOn)
	}
	assertUsesStep(t, actionJob, "Check out repository", "actions/checkout@v4")
	actionStep := requireStep(t, actionJob, "Run local composite action smoke")
	if actionStep.Uses != "./" {
		t.Fatalf("jobs.published-release-action step uses = %q, want ./", actionStep.Uses)
	}
	assertWithValue(t, actionStep.With, "chart-path", "./testdata/nginx-ingress")
	assertWithValue(t, actionStep.With, "output", "text")
	assertWithValue(t, actionStep.With, "min-score", "B")
	versionValue, ok := actionStep.With["version"]
	if !ok {
		t.Fatalf("jobs.published-release-action step missing version input in %#v", actionStep.With)
	}
	if strings.TrimSpace(fmt.Sprint(versionValue)) == "" {
		t.Fatalf("jobs.published-release-action step version input must not be empty: %#v", actionStep.With)
	}
	if !strings.Contains(raw, "uses: ./") {
		t.Fatalf("published release smoke workflow missing local action usage in %q", raw)
	}
	for _, want := range []string{
		"github.event.release.tag_name",
		"inputs.version",
		"scripts/verify-published-release.sh",
		"--install-kind brew",
		"--install-kind \"${{ matrix.install-kind }}\"",
		"uses: ./",
	} {
		if !strings.Contains(raw, want) {
			t.Fatalf("published release smoke workflow missing %q", want)
		}
	}

	scriptPath := filepath.Join("..", "..", "scripts", "verify-published-release.sh")
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", scriptPath, err)
	}
	scriptRaw := string(scriptBytes)
	for _, want := range []string{
		"--version <tag-or-latest>",
		"--install-kind <all|binary|docker|go-install|brew>",
		"--chart-path <path>",
		"BREW_FORMULA=\"belyaev-dev/homebrew-tap/helmdoc\"",
		"brew install \"$BREW_FORMULA\"",
		"docker pull",
		"go install",
		"scripts/install-helmdoc-release.sh",
		"require_file() {",
		"assert_output_contains() {",
		"print_output() {",
		"image_version=$(derive_release_version \"$release_tag\")",
		"image_ref=\"${DOCKER_IMAGE_REPOSITORY}:${image_version}\"",
		"version 2>&1",
		"scan \"$chart_path\" --min-score B",
		"fix \"$chart_path\" --output-dir \"$fix_dir\"",
		"assert_output_contains \"${install_kind} scan output\" \"$scan_output\" \"Overall:\" \"Score:\" \"Total findings:\"",
		"require_file \"$fix_dir/values-overrides.yaml\"",
		"require_file \"$fix_dir/README.md\"",
		"docker run --rm -v \"${chart_parent}:/fixtures:ro\" \"$image_ref\" scan \"/fixtures/${chart_name}\" --min-score B 2>&1",
		"assert_output_contains \"docker scan output\" \"$docker_scan_output\" \"Overall:\" \"Score:\" \"Total findings:\"",
		"docker run --rm -v \"${chart_parent}:/fixtures:ro\" -v \"$docker_fix_dir:/output\" \"$image_ref\" fix \"/fixtures/${chart_name}\" --output-dir /output 2>&1",
		"require_file \"$docker_fix_dir/values-overrides.yaml\"",
		"require_file \"$docker_fix_dir/README.md\"",
		"install kind ${install_kind}",
		"resolved release tag ${release_tag}",
		"derived docker image version ${image_version}",
		"command failed",
	} {
		if !strings.Contains(scriptRaw, want) {
			t.Fatalf("verify-published-release.sh missing %q", want)
		}
	}
	assertSubstringOrder(t, scriptRaw,
		`scan "$chart_path" --min-score B`,
		`print_output "$install_kind" scan "$scan_output"`,
		`assert_output_contains "${install_kind} scan output" "$scan_output" "Overall:" "Score:" "Total findings:"`,
	)
	assertSubstringOrder(t, scriptRaw,
		`fix "$chart_path" --output-dir "$fix_dir"`,
		`print_output "$install_kind" fix "$fix_output"`,
		`require_file "$fix_dir/values-overrides.yaml"`,
		`require_file "$fix_dir/README.md"`,
	)
	assertSubstringOrder(t, scriptRaw,
		`docker run --rm -v "${chart_parent}:/fixtures:ro" "$image_ref" scan "/fixtures/${chart_name}" --min-score B 2>&1`,
		`print_output docker scan "$docker_scan_output"`,
		`assert_output_contains "docker scan output" "$docker_scan_output" "Overall:" "Score:" "Total findings:"`,
	)
	assertSubstringOrder(t, scriptRaw,
		`docker run --rm -v "${chart_parent}:/fixtures:ro" -v "$docker_fix_dir:/output" "$image_ref" fix "/fixtures/${chart_name}" --output-dir /output 2>&1`,
		`print_output docker fix "$docker_fix_output"`,
		`require_file "$docker_fix_dir/values-overrides.yaml"`,
		`require_file "$docker_fix_dir/README.md"`,
	)
	for _, forbidden := range []string{
		"image_ref=\"${DOCKER_IMAGE_REPOSITORY}:${release_tag}\"",
		"derived docker image version ${release_tag}",
	} {
		if strings.Contains(scriptRaw, forbidden) {
			t.Fatalf("verify-published-release.sh should resolve Docker image tags from the stripped release version, not the full git tag: found %q", forbidden)
		}
	}
	if !strings.Contains(readme, "brew install belyaev-dev/homebrew-tap/helmdoc") {
		t.Fatal("README.md must keep the published Homebrew command that verify-published-release.sh smokes")
	}
	if !strings.Contains(readme, "ghcr.io/belyaev-dev/helmdoc") {
		t.Fatal("README.md must keep the published Docker command that verify-published-release.sh smokes")
	}
	if !strings.Contains(readme, "go install github.com/belyaev-dev/helmdoc@latest") {
		t.Fatal("README.md must keep the published go install command that verify-published-release.sh smokes")
	}
}
