package helmdoc

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	internalfix "github.com/belyaev-dev/helmdoc/internal/fix"
	"github.com/spf13/cobra"
)

func newFixCommand() *cobra.Command {
	var outputDir string
	var configPath string

	cmd := &cobra.Command{
		Use:          "fix [CHART_PATH]",
		Short:        "Generate a reviewable remediation bundle for a Helm chart",
		Long:         "Analyzes a Helm chart, plans safe values overrides plus optional Kustomize fallbacks, and writes a reviewable fix bundle to disk.",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFix(cmd.OutOrStdout(), args[0], outputDir, configPath)
		},
	}

	cmd.Flags().StringVar(&outputDir, "output-dir", "", "Directory where the generated fix bundle will be written")
	cmd.Flags().StringVar(&configPath, "config", "", "Optional path to a .helmdoc.yaml policy file")
	_ = cmd.MarkFlagRequired("output-dir")

	return cmd
}

func runFix(w io.Writer, chartPath, outputDir, configPath string) error {
	if strings.TrimSpace(outputDir) == "" {
		return fmt.Errorf("write bundle: --output-dir must not be empty")
	}

	analysis, err := analyzeChart(chartPath, configPath)
	if err != nil {
		return err
	}

	plan := internalfix.PlanBundle(analysis.Context, analysis.Findings)
	if err := writeFixBundle(outputDir, plan); err != nil {
		return err
	}

	writtenFiles := []string{"values-overrides.yaml", "README.md"}
	if plan.HasKustomizePatches() {
		writtenFiles = append(writtenFiles, filepath.ToSlash(filepath.Join(internalfix.KustomizeDirName, "kustomization.yaml")))
		for _, patch := range plan.KustomizePatches {
			writtenFiles = append(writtenFiles, internalfix.KustomizePatchRelativePath(patch))
		}
	}

	_, err = fmt.Fprintf(w, "Wrote helmdoc fix bundle to %s\nFiles: %s\n", outputDir, strings.Join(writtenFiles, ", "))
	return err
}

func writeFixBundle(outputDir string, plan internalfix.BundlePlan) error {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return fmt.Errorf("write bundle: mkdir %s: %w", outputDir, err)
	}

	valuesYAML, err := internalfix.RenderValuesOverridesYAML(plan)
	if err != nil {
		return fmt.Errorf("write bundle values-overrides.yaml: %w", err)
	}
	if err := writeBundleFile(outputDir, "values-overrides.yaml", valuesYAML); err != nil {
		return err
	}

	readme, err := internalfix.RenderREADME(plan)
	if err != nil {
		return fmt.Errorf("write bundle README.md: %w", err)
	}
	if err := writeBundleFile(outputDir, "README.md", readme); err != nil {
		return err
	}

	kustomizeDir := filepath.Join(outputDir, internalfix.KustomizeDirName)
	if err := os.RemoveAll(kustomizeDir); err != nil {
		return fmt.Errorf("write bundle %s/: %w", internalfix.KustomizeDirName, err)
	}
	if !plan.HasKustomizePatches() {
		return nil
	}
	if err := os.MkdirAll(kustomizeDir, 0o755); err != nil {
		return fmt.Errorf("write bundle %s/: %w", internalfix.KustomizeDirName, err)
	}

	kustomizationYAML, err := internalfix.RenderKustomizationYAML(plan)
	if err != nil {
		return fmt.Errorf("write bundle %s/kustomization.yaml: %w", internalfix.KustomizeDirName, err)
	}
	if err := writeBundleFile(outputDir, filepath.ToSlash(filepath.Join(internalfix.KustomizeDirName, "kustomization.yaml")), kustomizationYAML); err != nil {
		return err
	}

	for _, patch := range plan.KustomizePatches {
		relPath := internalfix.KustomizePatchRelativePath(patch)
		patchYAML, err := internalfix.RenderKustomizePatchYAML(patch)
		if err != nil {
			return fmt.Errorf("write bundle %s: %w", relPath, err)
		}
		if err := writeBundleFile(outputDir, relPath, patchYAML); err != nil {
			return err
		}
	}

	return nil
}

func writeBundleFile(outputDir, relativePath string, data []byte) error {
	absolutePath := filepath.Join(outputDir, filepath.FromSlash(relativePath))
	if err := os.WriteFile(absolutePath, data, 0o644); err != nil {
		return fmt.Errorf("write bundle %s: %w", relativePath, err)
	}
	return nil
}
