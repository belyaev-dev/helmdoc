package release

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type goReleaserConfig struct {
	Version         int                    `yaml:"version"`
	ProjectName     string                 `yaml:"project_name"`
	Release         releaseConfig          `yaml:"release"`
	Snapshot        snapshotConfig         `yaml:"snapshot"`
	Changelog       changelogConfig        `yaml:"changelog"`
	Builds          []buildConfig          `yaml:"builds"`
	Archives        []archiveConfig        `yaml:"archives"`
	Checksum        checksumConfig         `yaml:"checksum"`
	Brews           []brewConfig           `yaml:"brews"`
	Dockers         []dockerConfig         `yaml:"dockers"`
	DockerManifests []dockerManifestConfig `yaml:"docker_manifests"`
}

type releaseConfig struct {
	GitHub githubReleaseConfig `yaml:"github"`
}

type githubReleaseConfig struct {
	Owner string `yaml:"owner"`
	Name  string `yaml:"name"`
}

type snapshotConfig struct {
	VersionTemplate string `yaml:"version_template"`
}

type changelogConfig struct {
	Disable bool `yaml:"disable"`
}

type buildConfig struct {
	ID      string   `yaml:"id"`
	Main    string   `yaml:"main"`
	Binary  string   `yaml:"binary"`
	Env     []string `yaml:"env"`
	Goos    []string `yaml:"goos"`
	Goarch  []string `yaml:"goarch"`
	Ldflags []string `yaml:"ldflags"`
}

type archiveConfig struct {
	ID              string                  `yaml:"id"`
	IDs             []string                `yaml:"ids"`
	Formats         []string                `yaml:"formats"`
	NameTemplate    string                  `yaml:"name_template"`
	FormatOverrides []archiveFormatOverride `yaml:"format_overrides"`
	Files           []string                `yaml:"files"`
}

type archiveFormatOverride struct {
	Goos    string   `yaml:"goos"`
	Formats []string `yaml:"formats"`
}

type checksumConfig struct {
	NameTemplate string `yaml:"name_template"`
}

type brewConfig struct {
	IDs               []string       `yaml:"ids"`
	Repository        brewRepository `yaml:"repository"`
	Directory         string         `yaml:"directory"`
	Homepage          string         `yaml:"homepage"`
	Description       string         `yaml:"description"`
	License           string         `yaml:"license"`
	CommitMsgTemplate string         `yaml:"commit_msg_template"`
	Install           string         `yaml:"install"`
	Test              string         `yaml:"test"`
}

type brewRepository struct {
	Owner string `yaml:"owner"`
	Name  string `yaml:"name"`
	Token string `yaml:"token"`
}

type dockerConfig struct {
	IDs                []string `yaml:"ids"`
	Goos               string   `yaml:"goos"`
	Goarch             string   `yaml:"goarch"`
	Dockerfile         string   `yaml:"dockerfile"`
	Use                string   `yaml:"use"`
	ImageTemplates     []string `yaml:"image_templates"`
	BuildFlagTemplates []string `yaml:"build_flag_templates"`
}

type dockerManifestConfig struct {
	NameTemplate   string   `yaml:"name_template"`
	ImageTemplates []string `yaml:"image_templates"`
}

func TestGoReleaserConfigContract(t *testing.T) {
	cfg := loadGoReleaserConfig(t)

	if cfg.Version != 2 {
		t.Fatalf(".goreleaser.yaml version = %d, want 2", cfg.Version)
	}
	if cfg.ProjectName != "helmdoc" {
		t.Fatalf("project_name = %q, want helmdoc", cfg.ProjectName)
	}
	if cfg.Release.GitHub.Owner != "belyaev-dev" || cfg.Release.GitHub.Name != "helmdoc" {
		t.Fatalf("release.github = %#v, want owner=belyaev-dev name=helmdoc", cfg.Release.GitHub)
	}
	if cfg.Snapshot.VersionTemplate != "0.0.0-snapshot-{{ .ShortCommit }}" {
		t.Fatalf("snapshot.version_template = %q, want 0.0.0-snapshot-{{ .ShortCommit }}", cfg.Snapshot.VersionTemplate)
	}
	if !cfg.Changelog.Disable {
		t.Fatal("changelog.disable = false, want true to keep snapshot smoke independent of release-note state")
	}

	if len(cfg.Builds) != 1 {
		t.Fatalf("len(builds) = %d, want 1", len(cfg.Builds))
	}
	build := cfg.Builds[0]
	if build.ID != "helmdoc" || build.Main != "." || build.Binary != "helmdoc" {
		t.Fatalf("build identity = %#v, want id=helmdoc main=. binary=helmdoc", build)
	}
	assertSameStrings(t, "build env", build.Env, []string{"CGO_ENABLED=0"})
	assertSameStrings(t, "build goos", build.Goos, []string{"linux", "darwin", "windows"})
	assertSameStrings(t, "build goarch", build.Goarch, []string{"amd64", "arm64"})
	if len(build.Ldflags) != 1 {
		t.Fatalf("len(build.ldflags) = %d, want 1", len(build.Ldflags))
	}
	for _, want := range []string{
		"github.com/belyaev-dev/helmdoc/cmd/helmdoc.version={{ .Tag }}",
		"github.com/belyaev-dev/helmdoc/cmd/helmdoc.commit={{ .Commit }}",
		"github.com/belyaev-dev/helmdoc/cmd/helmdoc.date={{ .Date }}",
	} {
		if !strings.Contains(build.Ldflags[0], want) {
			t.Fatalf("build ldflags missing %q in %q", want, build.Ldflags[0])
		}
	}
	if strings.Contains(build.Ldflags[0], "github.com/belyaev-dev/helmdoc/cmd/helmdoc.version={{ .Version }}") {
		t.Fatalf("build ldflags must inject the full git tag into helmdoc version output, not the archive/image version: %q", build.Ldflags[0])
	}
	for _, forbidden := range []string{"main.version", "main.commit", "main.date"} {
		if strings.Contains(build.Ldflags[0], forbidden) {
			t.Fatalf("build ldflags should not reference %q: %q", forbidden, build.Ldflags[0])
		}
	}

	if len(cfg.Archives) != 1 {
		t.Fatalf("len(archives) = %d, want 1", len(cfg.Archives))
	}
	archive := cfg.Archives[0]
	if archive.ID != "binaries" {
		t.Fatalf("archive id = %q, want binaries", archive.ID)
	}
	assertSameStrings(t, "archive ids", archive.IDs, []string{"helmdoc"})
	assertSameStrings(t, "archive formats", archive.Formats, []string{"zip"})
	if archive.NameTemplate != "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}" {
		t.Fatalf("archive name_template = %q, want standard project/version/os/arch template", archive.NameTemplate)
	}
	if len(archive.FormatOverrides) != 0 {
		t.Fatalf("len(archive.format_overrides) = %d, want 0 because Homebrew formulas require a single non-gzip archive per os/arch", len(archive.FormatOverrides))
	}
	assertSameStrings(t, "archive files", archive.Files, []string{"LICENSE"})
	if cfg.Checksum.NameTemplate != "checksums.txt" {
		t.Fatalf("checksum.name_template = %q, want checksums.txt", cfg.Checksum.NameTemplate)
	}

	if len(cfg.Brews) != 1 {
		t.Fatalf("len(brews) = %d, want 1", len(cfg.Brews))
	}
	brew := cfg.Brews[0]
	assertSameStrings(t, "brew ids", brew.IDs, []string{"binaries"})
	if brew.Repository.Owner != "belyaev-dev" || brew.Repository.Name != "homebrew-tap" {
		t.Fatalf("brew.repository = %#v, want owner=belyaev-dev name=homebrew-tap", brew.Repository)
	}
	if brew.Repository.Token != "{{ .Env.GH_PAT }}" {
		t.Fatalf("brew.repository.token = %q, want {{ .Env.GH_PAT }}", brew.Repository.Token)
	}
	if brew.Directory != "Formula" {
		t.Fatalf("brew.directory = %q, want Formula", brew.Directory)
	}
	if brew.Homepage != "https://github.com/belyaev-dev/helmdoc" {
		t.Fatalf("brew.homepage = %q, want project GitHub URL", brew.Homepage)
	}
	if brew.Description != "Analyze and fix Helm charts" {
		t.Fatalf("brew.description = %q, want CLI summary", brew.Description)
	}
	if brew.License != "MIT" {
		t.Fatalf("brew.license = %q, want MIT", brew.License)
	}
	if !strings.Contains(brew.Install, `bin.install "helmdoc"`) {
		t.Fatalf("brew.install = %q, want bin.install \"helmdoc\"", brew.Install)
	}
	if !strings.Contains(brew.Test, `system "#{bin}/helmdoc", "version"`) {
		t.Fatalf("brew.test = %q, want helmdoc version smoke", brew.Test)
	}

	if len(cfg.Dockers) != 2 {
		t.Fatalf("len(dockers) = %d, want 2", len(cfg.Dockers))
	}
	assertDockerContract(t, cfg.Dockers, "amd64", "linux/amd64")
	assertDockerContract(t, cfg.Dockers, "arm64", "linux/arm64")

	if len(cfg.DockerManifests) != 2 {
		t.Fatalf("len(docker_manifests) = %d, want 2", len(cfg.DockerManifests))
	}
	assertDockerManifest(t, cfg.DockerManifests, "ghcr.io/belyaev-dev/helmdoc:{{ .Version }}")
	assertDockerManifest(t, cfg.DockerManifests, "ghcr.io/belyaev-dev/helmdoc:latest")
}

func assertDockerContract(t *testing.T, dockers []dockerConfig, goarch, platform string) {
	t.Helper()

	for _, docker := range dockers {
		if docker.Goarch != goarch {
			continue
		}
		assertSameStrings(t, "docker ids "+goarch, docker.IDs, []string{"helmdoc"})
		if docker.Goos != "linux" {
			t.Fatalf("docker[%s].goos = %q, want linux", goarch, docker.Goos)
		}
		if docker.Dockerfile != "Dockerfile" {
			t.Fatalf("docker[%s].dockerfile = %q, want Dockerfile", goarch, docker.Dockerfile)
		}
		if docker.Use != "buildx" {
			t.Fatalf("docker[%s].use = %q, want buildx", goarch, docker.Use)
		}
		assertSameStrings(t, "docker image templates "+goarch, docker.ImageTemplates, []string{"ghcr.io/belyaev-dev/helmdoc:{{ .Version }}-" + goarch})
		assertContains(t, "docker build flags "+goarch, docker.BuildFlagTemplates, "--platform="+platform)
		assertContains(t, "docker build flags "+goarch, docker.BuildFlagTemplates, "--label=org.opencontainers.image.version={{ .Version }}")
		assertContains(t, "docker build flags "+goarch, docker.BuildFlagTemplates, "--label=org.opencontainers.image.revision={{ .FullCommit }}")
		assertContains(t, "docker build flags "+goarch, docker.BuildFlagTemplates, "--label=org.opencontainers.image.source=https://github.com/belyaev-dev/helmdoc")
		return
	}

	t.Fatalf("no docker entry found for goarch %q in %#v", goarch, dockers)
}

func assertDockerManifest(t *testing.T, manifests []dockerManifestConfig, name string) {
	t.Helper()

	for _, manifest := range manifests {
		if manifest.NameTemplate != name {
			continue
		}
		assertSameStrings(t, "docker manifest "+name, manifest.ImageTemplates, []string{
			"ghcr.io/belyaev-dev/helmdoc:{{ .Version }}-amd64",
			"ghcr.io/belyaev-dev/helmdoc:{{ .Version }}-arm64",
		})
		return
	}

	t.Fatalf("docker manifest %q not found in %#v", name, manifests)
}

func assertContains(t *testing.T, name string, values []string, want string) {
	t.Helper()
	for _, value := range values {
		if value == want {
			return
		}
	}
	t.Fatalf("%s missing %q in %#v", name, want, values)
}

func assertSameStrings(t *testing.T, name string, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s length = %d, want %d (%#v)", name, len(got), len(want), got)
	}
	remaining := make(map[string]int, len(want))
	for _, value := range want {
		remaining[value]++
	}
	for _, value := range got {
		remaining[value]--
	}
	for _, count := range remaining {
		if count != 0 {
			t.Fatalf("%s mismatch: got %#v want %#v", name, got, want)
		}
	}
}

func loadGoReleaserConfig(t *testing.T) goReleaserConfig {
	t.Helper()

	path := filepath.Join("..", "..", ".goreleaser.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q): %v", path, err)
	}

	var cfg goReleaserConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("yaml.Unmarshal(%q): %v", path, err)
	}
	return cfg
}
