package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type actionConfig struct {
	Name        string                 `yaml:"name"`
	Description string                 `yaml:"description"`
	Inputs      map[string]actionInput `yaml:"inputs"`
	Runs        actionRuns             `yaml:"runs"`
}

type actionInput struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"`
}

type actionRuns struct {
	Using string       `yaml:"using"`
	Steps []actionStep `yaml:"steps"`
}

type actionStep struct {
	Name  string `yaml:"name"`
	Shell string `yaml:"shell"`
	Run   string `yaml:"run"`
}

func TestActionContract(t *testing.T) {
	action, actionRaw := loadActionConfig(t)
	cfg := loadGoReleaserConfig(t)
	scriptPath := filepath.Join("..", "..", "scripts", "install-helmdoc-release.sh")
	scriptBytes, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", scriptPath, err)
	}
	scriptRaw := string(scriptBytes)
	stat, err := os.Stat(scriptPath)
	if err != nil {
		t.Fatalf("os.Stat(%q): %v", scriptPath, err)
	}

	if action.Name != "helmdoc scan" {
		t.Fatalf("action name = %q, want helmdoc scan", action.Name)
	}
	if action.Description == "" {
		t.Fatal("action description must not be empty")
	}
	assertSameStrings(t, "action inputs", mapKeys(action.Inputs), []string{"chart-path", "version", "output", "min-score", "config"})

	chartPath := requireActionInput(t, action, "chart-path")
	if !chartPath.Required {
		t.Fatal("inputs.chart-path.required = false, want true")
	}
	if chartPath.Default != "" {
		t.Fatalf("inputs.chart-path.default = %q, want empty", chartPath.Default)
	}

	version := requireActionInput(t, action, "version")
	if version.Required {
		t.Fatal("inputs.version.required = true, want false")
	}
	if version.Default != "latest" {
		t.Fatalf("inputs.version.default = %q, want latest", version.Default)
	}

	output := requireActionInput(t, action, "output")
	if output.Default != "text" {
		t.Fatalf("inputs.output.default = %q, want text", output.Default)
	}

	minScore := requireActionInput(t, action, "min-score")
	if minScore.Default != "" {
		t.Fatalf("inputs.min-score.default = %q, want empty", minScore.Default)
	}

	config := requireActionInput(t, action, "config")
	if config.Required {
		t.Fatal("inputs.config.required = true, want false")
	}
	if config.Default != "" {
		t.Fatalf("inputs.config.default = %q, want empty", config.Default)
	}

	if action.Runs.Using != "composite" {
		t.Fatalf("runs.using = %q, want composite", action.Runs.Using)
	}
	if len(action.Runs.Steps) != 2 {
		t.Fatalf("len(runs.steps) = %d, want 2", len(action.Runs.Steps))
	}

	installStep := requireActionStep(t, action, "Install helmdoc release")
	if installStep.Shell != "bash" {
		t.Fatalf("install step shell = %q, want bash", installStep.Shell)
	}
	if !strings.Contains(installStep.Run, `scripts/install-helmdoc-release.sh" --version "${{ inputs.version }}`) {
		t.Fatalf("install step run = %q, want shared installer script with version input", installStep.Run)
	}

	scanStep := requireActionStep(t, action, "Run helmdoc scan")
	if scanStep.Shell != "bash" {
		t.Fatalf("scan step shell = %q, want bash", scanStep.Shell)
	}
	for _, want := range []string{
		`args=("scan" "${{ inputs.chart-path }}")`,
		`if [[ -n "${{ inputs.output }}" ]]; then`,
		`if [[ -n "${{ inputs.min-score }}" ]]; then`,
		`if [[ -n "${{ inputs.config }}" ]]; then`,
		`helmdoc "${args[@]}"`,
	} {
		if !strings.Contains(scanStep.Run, want) {
			t.Fatalf("scan step run missing %q in %q", want, scanStep.Run)
		}
	}
	for _, forbidden := range []string{"go build", "go run .", "go install ./..."} {
		if strings.Contains(actionRaw, forbidden) {
			t.Fatalf("action should install a published release, not build from source; found %q in %q", forbidden, actionRaw)
		}
	}

	if stat.Mode()&0o111 == 0 {
		t.Fatalf("installer script mode = %v, want executable bit set", stat.Mode())
	}
	for _, want := range []string{
		`OWNER="belyaev-dev"`,
		`REPO="helmdoc"`,
		`INSTALL_KIND="github-release-archive"`,
		`RUNNER_OS`,
		`RUNNER_ARCH`,
		`archive_version=$(derive_release_version "$resolved_tag")`,
		`https://github.com/${OWNER}/${REPO}/releases/download/${resolved_tag}/${archive_name}`,
		`archive_name="${PROJECT_NAME}_${archive_version}_${target_os}_${target_arch}.zip"`,
		`""|latest|releases)`,
		`printf 'HELMDOC_RELEASE_TAG=%s\n'`,
		`printf 'HELMDOC_RELEASE_ARCHIVE_VERSION=%s\n'`,
		`printf 'HELMDOC_RELEASE_OS=%s\n'`,
		`printf 'HELMDOC_RELEASE_ARCH=%s\n'`,
		`printf 'HELMDOC_RELEASE_INSTALL_KIND=%s\n'`,
		`printf 'HELMDOC_RELEASE_ASSET_URL=%s\n'`,
		`log "resolved release tag ${resolved_tag}"`,
		`log "derived archive version ${archive_version}"`,
		`helmdoc version`,
	} {
		if !strings.Contains(scriptRaw, want) {
			t.Fatalf("installer script missing %q", want)
		}
	}
	for _, forbidden := range []string{
		`archive_name="${PROJECT_NAME}_${resolved_tag}_${target_os}_${target_arch}.zip"`,
		`printf 'HELMDOC_RELEASE_ARCHIVE_VERSION=%s\n' "$resolved_tag"`,
	} {
		if strings.Contains(scriptRaw, forbidden) {
			t.Fatalf("installer script should keep the full git tag only for release lookup and diagnostics, not archive naming: found %q", forbidden)
		}
	}

	build := cfg.Builds[0]
	archive := cfg.Archives[0]
	if archive.NameTemplate != "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}" {
		t.Fatalf("archive name_template = %q, want project/version/os/arch template", archive.NameTemplate)
	}
	for _, goos := range build.Goos {
		if !strings.Contains(scriptRaw, goos) {
			t.Fatalf("installer script should normalize goos %q from .goreleaser.yaml", goos)
		}
	}
	for _, goarch := range build.Goarch {
		if !strings.Contains(scriptRaw, goarch) {
			t.Fatalf("installer script should normalize goarch %q from .goreleaser.yaml", goarch)
		}
	}
	if !strings.Contains(scriptRaw, `binary_name="helmdoc.exe"`) {
		t.Fatal("installer script must handle windows binary naming with helmdoc.exe")
	}
}

func loadActionConfig(t *testing.T) (actionConfig, string) {
	t.Helper()

	path := filepath.Join("..", "..", "action.yml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}

	var action actionConfig
	if err := yaml.Unmarshal(data, &action); err != nil {
		t.Fatalf("yaml.Unmarshal(%q): %v", path, err)
	}
	return action, string(data)
}

func mapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func requireActionInput(t *testing.T, action actionConfig, name string) actionInput {
	t.Helper()

	input, ok := action.Inputs[name]
	if !ok {
		t.Fatalf("action missing input %q in %#v", name, action.Inputs)
	}
	if strings.TrimSpace(input.Description) == "" {
		t.Fatalf("action input %q must have a description", name)
	}
	return input
}

func requireActionStep(t *testing.T, action actionConfig, name string) actionStep {
	t.Helper()

	for _, step := range action.Runs.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("action missing step %q in %#v", name, action.Runs.Steps)
	return actionStep{}
}
