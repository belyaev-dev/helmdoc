package chart

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadChartFromDirectoryAndArchive(t *testing.T) {
	chartPath := fixtureChartPath(t)

	directoryChart, err := LoadChart(chartPath)
	if err != nil {
		t.Fatalf("LoadChart(directory) error = %v", err)
	}

	if directoryChart.Metadata == nil {
		t.Fatal("directory chart metadata is nil")
	}
	if directoryChart.Metadata.Name != "testchart" {
		t.Fatalf("directory chart name = %q, want testchart", directoryChart.Metadata.Name)
	}
	if directoryChart.Metadata.Version != "0.1.0" {
		t.Fatalf("directory chart version = %q, want 0.1.0", directoryChart.Metadata.Version)
	}
	if len(directoryChart.Templates) != 2 {
		t.Fatalf("directory chart templates = %d, want 2", len(directoryChart.Templates))
	}

	archivePath := createChartArchive(t, chartPath)
	archiveChart, err := LoadChart(archivePath)
	if err != nil {
		t.Fatalf("LoadChart(archive) error = %v", err)
	}

	if archiveChart.Metadata == nil {
		t.Fatal("archive chart metadata is nil")
	}
	if archiveChart.Metadata.Name != directoryChart.Metadata.Name {
		t.Fatalf("archive chart name = %q, want %q", archiveChart.Metadata.Name, directoryChart.Metadata.Name)
	}
	if archiveChart.Metadata.Version != directoryChart.Metadata.Version {
		t.Fatalf("archive chart version = %q, want %q", archiveChart.Metadata.Version, directoryChart.Metadata.Version)
	}
	if len(archiveChart.Templates) != len(directoryChart.Templates) {
		t.Fatalf("archive chart templates = %d, want %d", len(archiveChart.Templates), len(directoryChart.Templates))
	}

	templateNames := make(map[string]struct{}, len(archiveChart.Templates))
	for _, template := range archiveChart.Templates {
		templateNames[template.Name] = struct{}{}
	}
	for _, want := range []string{"templates/resources.yaml", "templates/graceful.yaml"} {
		if _, ok := templateNames[want]; !ok {
			t.Fatalf("archive templates missing %q", want)
		}
	}
}

func TestLoadChartMissingPathErrorIncludesRequestedPath(t *testing.T) {
	missingPath := filepath.Join("..", "..", "testdata", "missing-chart")

	_, err := LoadChart(missingPath)
	if err == nil {
		t.Fatal("LoadChart(missing path) error = nil, want non-nil")
	}

	if !strings.Contains(err.Error(), missingPath) {
		t.Fatalf("error %q does not include requested path %q", err.Error(), missingPath)
	}
}

func fixtureChartPath(t *testing.T) string {
	t.Helper()
	return filepath.Join("..", "..", "testdata", "testchart")
}

func createChartArchive(t *testing.T, sourceDir string) string {
	t.Helper()

	archivePath := filepath.Join(t.TempDir(), filepath.Base(sourceDir)+".tgz")
	archiveFile, err := os.Create(archivePath)
	if err != nil {
		t.Fatalf("os.Create(%q) error = %v", archivePath, err)
	}
	defer archiveFile.Close()

	gzipWriter := gzip.NewWriter(archiveFile)
	defer gzipWriter.Close()

	tarWriter := tar.NewWriter(gzipWriter)
	defer tarWriter.Close()

	baseName := filepath.Base(sourceDir)
	err = filepath.Walk(sourceDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		tarPath := baseName
		if relPath != "." {
			tarPath = filepath.ToSlash(filepath.Join(baseName, relPath))
		}

		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = tarPath
		if info.IsDir() {
			header.Name += "/"
		}

		if err := tarWriter.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		_, err = io.Copy(tarWriter, file)
		return err
	})
	if err != nil {
		t.Fatalf("createChartArchive(%q) error = %v", sourceDir, err)
	}

	return archivePath
}
