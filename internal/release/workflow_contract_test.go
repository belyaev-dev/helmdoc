package release

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type workflowConfig struct {
	Name        string                   `yaml:"name"`
	On          map[string]workflowEvent `yaml:"on"`
	Permissions map[string]string        `yaml:"permissions"`
	Jobs        map[string]workflowJob   `yaml:"jobs"`
}

type workflowEvent struct {
	Branches   []string                         `yaml:"branches"`
	Tags       []string                         `yaml:"tags"`
	TagsIgnore []string                         `yaml:"tags-ignore"`
	Types      []string                         `yaml:"types"`
	Inputs     map[string]workflowDispatchInput `yaml:"inputs"`
}

type workflowDispatchInput struct {
	Description string `yaml:"description"`
	Required    bool   `yaml:"required"`
	Default     string `yaml:"default"`
	Type        string `yaml:"type"`
}

type workflowJob struct {
	Name        string            `yaml:"name"`
	RunsOn      string            `yaml:"runs-on"`
	Needs       interface{}       `yaml:"needs"`
	If          string            `yaml:"if"`
	Permissions map[string]string `yaml:"permissions"`
	Strategy    workflowStrategy  `yaml:"strategy"`
	Steps       []workflowStep    `yaml:"steps"`
}

type workflowStrategy struct {
	FailFast bool                `yaml:"fail-fast"`
	Matrix   map[string][]string `yaml:"matrix"`
}

type workflowStep struct {
	Name string            `yaml:"name"`
	Uses string            `yaml:"uses"`
	Run  string            `yaml:"run"`
	With map[string]any    `yaml:"with"`
	Env  map[string]string `yaml:"env"`
	If   string            `yaml:"if"`
}

func TestCIWorkflowContract(t *testing.T) {
	workflow, raw := loadWorkflowConfig(t, filepath.Join("..", "..", ".github", "workflows", "ci.yml"))

	if workflow.Name != "CI" {
		t.Fatalf("ci workflow name = %q, want CI", workflow.Name)
	}
	if _, ok := workflow.On["pull_request"]; !ok {
		t.Fatal("ci workflow missing on.pull_request trigger")
	}
	push, ok := workflow.On["push"]
	if !ok {
		t.Fatal("ci workflow missing on.push trigger")
	}
	assertSameStrings(t, "ci push branches", push.Branches, []string{"**"})
	assertSameStrings(t, "ci push tags-ignore", push.TagsIgnore, []string{"v*"})
	if workflow.Permissions["contents"] != "read" {
		t.Fatalf("ci workflow permissions.contents = %q, want read", workflow.Permissions["contents"])
	}
	if strings.Contains(raw, "GH_PAT") {
		t.Fatalf("ci workflow should not reference GH_PAT: %q", raw)
	}

	testBuild := requireJob(t, workflow, "test-build")
	if testBuild.Name != "test-and-build" {
		t.Fatalf("jobs.test-build.name = %q, want test-and-build", testBuild.Name)
	}
	if testBuild.RunsOn != "ubuntu-latest" {
		t.Fatalf("jobs.test-build.runs-on = %q, want ubuntu-latest", testBuild.RunsOn)
	}
	assertUsesStep(t, testBuild, "Check out repository", "actions/checkout@v4")
	setupGo := assertUsesStep(t, testBuild, "Set up Go", "actions/setup-go@v5")
	assertWithValue(t, setupGo.With, "go-version-file", "go.mod")
	assertWithValue(t, setupGo.With, "cache", "true")
	assertRunStepContains(t, testBuild, "Run unit and integration tests", "go test ./...")
	assertRunStepContains(t, testBuild, "Build all packages", "go build ./...")

	releaseSmoke := requireJob(t, workflow, "release-smoke")
	if releaseSmoke.Name != "release-snapshot-smoke" {
		t.Fatalf("jobs.release-smoke.name = %q, want release-snapshot-smoke", releaseSmoke.Name)
	}
	if releaseSmoke.If != "github.event_name == 'push'" {
		t.Fatalf("jobs.release-smoke.if = %q, want github.event_name == 'push'", releaseSmoke.If)
	}
	assertNeeds(t, releaseSmoke.Needs, "test-build")
	if releaseSmoke.RunsOn != "ubuntu-latest" {
		t.Fatalf("jobs.release-smoke.runs-on = %q, want ubuntu-latest", releaseSmoke.RunsOn)
	}
	assertUsesStep(t, releaseSmoke, "Check out repository", "actions/checkout@v4")
	setupGo = assertUsesStep(t, releaseSmoke, "Set up Go", "actions/setup-go@v5")
	assertWithValue(t, setupGo.With, "go-version-file", "go.mod")
	assertWithValue(t, setupGo.With, "cache", "true")
	assertUsesStep(t, releaseSmoke, "Set up QEMU", "docker/setup-qemu-action@v3")
	assertUsesStep(t, releaseSmoke, "Set up Docker Buildx", "docker/setup-buildx-action@v3")
	assertRunStepContains(t, releaseSmoke, "Run release snapshot smoke", "bash scripts/verify-release-snapshot.sh")
}

func TestReleaseWorkflowContract(t *testing.T) {
	workflow, raw := loadWorkflowConfig(t, filepath.Join("..", "..", ".github", "workflows", "release.yml"))

	if workflow.Name != "Release" {
		t.Fatalf("release workflow name = %q, want Release", workflow.Name)
	}
	push, ok := workflow.On["push"]
	if !ok {
		t.Fatal("release workflow missing on.push trigger")
	}
	if _, ok := workflow.On["pull_request"]; ok {
		t.Fatal("release workflow should not declare pull_request trigger")
	}
	assertSameStrings(t, "release push tags", push.Tags, []string{"v*"})
	if workflow.Permissions["contents"] != "write" {
		t.Fatalf("release workflow permissions.contents = %q, want write", workflow.Permissions["contents"])
	}
	if workflow.Permissions["packages"] != "write" {
		t.Fatalf("release workflow permissions.packages = %q, want write", workflow.Permissions["packages"])
	}
	if len(workflow.Permissions) != 2 {
		t.Fatalf("release workflow permissions = %#v, want exactly contents/packages write", workflow.Permissions)
	}
	if count := strings.Count(raw, "secrets.GH_PAT"); count != 1 {
		t.Fatalf("release workflow should reference secrets.GH_PAT exactly once in the GoReleaser step env; found %d occurrences", count)
	}

	publish := requireJob(t, workflow, "publish-release")
	if publish.Name != "publish-release" {
		t.Fatalf("jobs.publish-release.name = %q, want publish-release", publish.Name)
	}
	if publish.RunsOn != "ubuntu-latest" {
		t.Fatalf("jobs.publish-release.runs-on = %q, want ubuntu-latest", publish.RunsOn)
	}
	checkout := assertUsesStep(t, publish, "Check out repository", "actions/checkout@v4")
	assertWithValue(t, checkout.With, "fetch-depth", "0")
	setupGo := assertUsesStep(t, publish, "Set up Go", "actions/setup-go@v5")
	assertWithValue(t, setupGo.With, "go-version-file", "go.mod")
	assertWithValue(t, setupGo.With, "cache", "true")
	assertUsesStep(t, publish, "Set up QEMU", "docker/setup-qemu-action@v3")
	assertUsesStep(t, publish, "Set up Docker Buildx", "docker/setup-buildx-action@v3")

	ghcrLogin := assertUsesStep(t, publish, "Login to GHCR", "docker/login-action@v4")
	assertWithValue(t, ghcrLogin.With, "registry", "ghcr.io")
	assertWithValue(t, ghcrLogin.With, "username", "${{ github.actor }}")
	assertWithValue(t, ghcrLogin.With, "password", "${{ secrets.GITHUB_TOKEN }}")

	releaseStep := requireStep(t, publish, "Run GoReleaser release")
	if strings.TrimSpace(releaseStep.Run) != "go run github.com/goreleaser/goreleaser/v2@v2.14.3 release --clean --verbose" {
		t.Fatalf("Run GoReleaser release command = %q, want pinned go run goreleaser release --clean --verbose", releaseStep.Run)
	}
	if releaseStep.Env["GITHUB_TOKEN"] != "${{ secrets.GITHUB_TOKEN }}" {
		t.Fatalf("Run GoReleaser release env GITHUB_TOKEN = %q, want ${{ secrets.GITHUB_TOKEN }}", releaseStep.Env["GITHUB_TOKEN"])
	}
	if releaseStep.Env["GH_PAT"] != "${{ secrets.GH_PAT }}" {
		t.Fatalf("Run GoReleaser release env GH_PAT = %q, want ${{ secrets.GH_PAT }}", releaseStep.Env["GH_PAT"])
	}
	if len(releaseStep.Env) != 2 {
		t.Fatalf("Run GoReleaser release env = %#v, want exactly GITHUB_TOKEN and GH_PAT", releaseStep.Env)
	}
	if strings.Contains(fmt.Sprint(ghcrLogin.With["password"]), "GH_PAT") {
		t.Fatalf("Login to GHCR should use GITHUB_TOKEN, not GH_PAT: %#v", ghcrLogin.With)
	}

	assertTrackedCLIReleaseSources(t)
}

func loadWorkflowConfig(t *testing.T, path string) (workflowConfig, string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}

	var workflow workflowConfig
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatalf("yaml.Unmarshal(%q): %v", path, err)
	}
	return workflow, string(data)
}

func requireJob(t *testing.T, workflow workflowConfig, name string) workflowJob {
	t.Helper()

	job, ok := workflow.Jobs[name]
	if !ok {
		t.Fatalf("workflow missing job %q in %#v", name, workflow.Jobs)
	}
	return job
}

func requireStep(t *testing.T, job workflowJob, name string) workflowStep {
	t.Helper()

	for _, step := range job.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("job %q missing step %q in %#v", job.Name, name, job.Steps)
	return workflowStep{}
}

func assertUsesStep(t *testing.T, job workflowJob, name, uses string) workflowStep {
	t.Helper()

	step := requireStep(t, job, name)
	if step.Uses != uses {
		t.Fatalf("job %q step %q uses = %q, want %q", job.Name, name, step.Uses, uses)
	}
	return step
}

func assertRunStepContains(t *testing.T, job workflowJob, name, want string) {
	t.Helper()

	step := requireStep(t, job, name)
	if !strings.Contains(step.Run, want) {
		t.Fatalf("job %q step %q run = %q, want substring %q", job.Name, name, step.Run, want)
	}
}

func assertWithValue(t *testing.T, values map[string]any, key, want string) {
	t.Helper()

	if values == nil {
		t.Fatalf("step.with is nil, expected %q=%q", key, want)
	}
	got, ok := values[key]
	if !ok {
		t.Fatalf("step.with missing %q in %#v", key, values)
	}
	if fmt.Sprint(got) != want {
		t.Fatalf("step.with[%q] = %q, want %q", key, fmt.Sprint(got), want)
	}
}

func assertNeeds(t *testing.T, needs interface{}, want string) {
	t.Helper()

	switch value := needs.(type) {
	case string:
		if value != want {
			t.Fatalf("job.needs = %q, want %q", value, want)
		}
	case []any:
		if len(value) != 1 || fmt.Sprint(value[0]) != want {
			t.Fatalf("job.needs = %#v, want [%q]", value, want)
		}
	default:
		t.Fatalf("job.needs has unexpected type %T (%#v), want %q", needs, needs, want)
	}
}

func assertTrackedCLIReleaseSources(t *testing.T) {
	t.Helper()

	repoRoot := filepath.Join("..", "..")
	cmd := exec.Command("git", "ls-files", "--", "cmd/helmdoc")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git ls-files -- cmd/helmdoc: %v\n%s", err, strings.TrimSpace(string(output)))
	}

	tracked := strings.Fields(strings.TrimSpace(string(output)))
	for _, want := range []string{
		"cmd/helmdoc/root.go",
		"cmd/helmdoc/scan.go",
		"cmd/helmdoc/analysis.go",
		"cmd/helmdoc/fix.go",
	} {
		if !containsString(tracked, want) {
			t.Fatalf("release workflow builds a clean checkout, so %q must be tracked; git ls-files returned %#v", want, tracked)
		}
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
