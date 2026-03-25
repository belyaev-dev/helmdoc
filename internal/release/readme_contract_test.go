package release

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type realChartFixtures struct {
	Fixtures []realChartFixture `json:"fixtures"`
}

type realChartFixture struct {
	ID           string               `json:"id"`
	ChartName    string               `json:"chart_name"`
	ChartVersion string               `json:"chart_version"`
	Expected     realChartExpectation `json:"expected"`
}

type realChartExpectation struct {
	OverallGrade  string  `json:"overall_grade"`
	OverallScore  float64 `json:"overall_score"`
	TotalFindings int     `json:"total_findings"`
}

func TestREADMEContract(t *testing.T) {
	readme := loadREADME(t)
	cfg := loadGoReleaserConfig(t)
	action, _ := loadActionConfig(t)
	modulePath := loadModulePath(t)
	fixture := loadFixtureByID(t, "nginx-ingress")

	if strings.TrimSpace(readme) == "" {
		t.Fatal("README.md must not be empty")
	}

	brew := cfg.Brews[0]
	brewInstall := fmt.Sprintf("brew install %s/%s/helmdoc", brew.Repository.Owner, brew.Repository.Name)
	dockerVersionRef := dockerReleaseReference(t, cfg, "${HELMDOC_IMAGE_VERSION}")
	archiveName := archiveReleaseName(t, cfg, "${HELMDOC_VERSION}", "${OS}", "${ARCH}")
	downloadURL := fmt.Sprintf(
		"https://github.com/%s/%s/releases/download/${HELMDOC_TAG}/%s",
		cfg.Release.GitHub.Owner,
		cfg.Release.GitHub.Name,
		archiveName,
	)

	for _, want := range []string{
		"# helmdoc",
		brewInstall,
		dockerRepositoryFromReference(t, dockerVersionRef),
		dockerVersionRef,
		fmt.Sprintf("go install %s@latest", modulePath),
		"HELMDOC_TAG=v0.1.0",
		`HELMDOC_VERSION="${HELMDOC_TAG#v}"`,
		`HELMDOC_IMAGE_VERSION="${HELMDOC_TAG#v}"`,
		downloadURL,
		fmt.Sprintf(`unzip %q`, archiveName),
		"helmdoc completion bash",
		"helmdoc completion zsh",
		"helmdoc fix ./charts/my-chart --output-dir ./tmp/helmdoc-fix",
		"helmdoc scan ./charts/my-chart --config .helmdoc.yaml --min-score B",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README.md missing %q", want)
		}
	}

	for _, forbidden := range []string{
		fmt.Sprintf(
			"https://github.com/%s/%s/releases/download/${HELMDOC_VERSION}/%s",
			cfg.Release.GitHub.Owner,
			cfg.Release.GitHub.Name,
			archiveName,
		),
		archiveReleaseName(t, cfg, "${HELMDOC_TAG}", "${OS}", "${ARCH}"),
		dockerRepositoryFromReference(t, dockerVersionRef) + ":${HELMDOC_TAG}",
	} {
		if strings.Contains(readme, forbidden) {
			t.Fatalf("README.md should use the full tag only for release lookup and the stripped version for archive/image names; found %q", forbidden)
		}
	}

	for _, want := range []string{
		"uses: belyaev-dev/helmdoc@v1",
		"chart-path: ./charts/my-chart",
		"version: latest",
		"output: text",
		"min-score: B",
		"config: .helmdoc.yaml",
		"GitLab CI",
		`HELMDOC_IMAGE_VERSION: "0.1.0"`,
		"entrypoint: [\"\"]",
		dockerVersionRef,
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README.md missing CI snippet %q", want)
		}
	}

	assertSameStrings(t, "action inputs", mapKeys(action.Inputs), []string{"chart-path", "version", "output", "min-score", "config"})
	for _, inputName := range []string{"chart-path", "version", "output", "min-score", "config"} {
		if !strings.Contains(readme, inputName+":") {
			t.Fatalf("README.md must mention action input %q", inputName)
		}
	}

	for _, chartName := range loadCuratedChartNames(t) {
		if !strings.Contains(readme, "`"+chartName+"`") {
			t.Fatalf("README.md missing curated chart mention %q", chartName)
		}
	}

	for _, want := range []string{
		fmt.Sprintf("Chart: %s@%s", fixture.ChartName, fixture.ChartVersion),
		fmt.Sprintf("Overall: %s", fixture.Expected.OverallGrade),
		fmt.Sprintf("Score: %.1f/100", fixture.Expected.OverallScore),
		fmt.Sprintf("Total findings: %d", fixture.Expected.TotalFindings),
		"Security: B (85.0/100, weight 3.0, findings 1)",
		"Resources: D (60.0/100, weight 2.5, findings 5)",
		"[SEC003][error] Deployment/helmdoc-ingress-nginx-controller @ templates/controller-deployment.yaml",
		"Title: Container root filesystem is writable",
	} {
		if !strings.Contains(readme, want) {
			t.Fatalf("README.md missing real fixture output %q", want)
		}
	}
}

func archiveReleaseName(t *testing.T, cfg goReleaserConfig, version, goos, goarch string) string {
	t.Helper()

	if len(cfg.Archives) == 0 {
		t.Fatal(".goreleaser.yaml must define at least one archive")
	}
	archive := cfg.Archives[0]
	if len(archive.Formats) == 0 {
		t.Fatal(".goreleaser.yaml archive must define at least one format")
	}

	return renderGoReleaserTemplate(archive.NameTemplate, map[string]string{
		"{{ .ProjectName }}": cfg.ProjectName,
		"{{ .Version }}":     version,
		"{{ .Os }}":          goos,
		"{{ .Arch }}":        goarch,
	}) + "." + archive.Formats[0]
}

func dockerReleaseReference(t *testing.T, cfg goReleaserConfig, version string) string {
	t.Helper()

	for _, manifest := range cfg.DockerManifests {
		if strings.Contains(manifest.NameTemplate, "{{ .Version }}") {
			return renderGoReleaserTemplate(manifest.NameTemplate, map[string]string{
				"{{ .Version }}": version,
			})
		}
	}

	t.Fatal(".goreleaser.yaml must define a versioned docker manifest")
	return ""
}

func dockerRepositoryFromReference(t *testing.T, ref string) string {
	t.Helper()

	separator := strings.LastIndex(ref, ":")
	if separator <= 0 {
		t.Fatalf("docker reference %q does not include a tag separator", ref)
	}
	return ref[:separator]
}

func renderGoReleaserTemplate(template string, values map[string]string) string {
	rendered := template
	for placeholder, value := range values {
		rendered = strings.ReplaceAll(rendered, placeholder, value)
	}
	return rendered
}

func loadREADME(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "README.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}
	return string(data)
}

func loadModulePath(t *testing.T) string {
	t.Helper()

	path := filepath.Join("..", "..", "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	t.Fatalf("module path not found in %q", path)
	return ""
}

func loadFixtureByID(t *testing.T, id string) realChartFixture {
	t.Helper()

	path := filepath.Join("..", "..", "testdata", "real-chart-fixtures.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}

	var fixtures realChartFixtures
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", path, err)
	}

	for _, fixture := range fixtures.Fixtures {
		if fixture.ID == id {
			return fixture
		}
	}

	t.Fatalf("fixture %q not found in %q", id, path)
	return realChartFixture{}
}

func loadCuratedChartNames(t *testing.T) []string {
	t.Helper()

	paths, err := filepath.Glob(filepath.Join("..", "..", "internal", "registry", "charts", "*.yaml"))
	if err != nil {
		t.Fatalf("filepath.Glob(internal/registry/charts/*.yaml): %v", err)
	}
	if len(paths) == 0 {
		t.Fatal("no curated chart registry files found")
	}

	names := make([]string, 0, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(%q): %v", path, err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "chart:") {
				name := strings.TrimSpace(strings.TrimPrefix(line, "chart:"))
				if name == "" {
					t.Fatalf("chart name missing in %q", path)
				}
				names = append(names, name)
				break
			}
		}
	}

	return names
}
