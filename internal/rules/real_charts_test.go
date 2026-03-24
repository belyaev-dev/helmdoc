package rules_test

import (
	"reflect"
	"testing"

	"github.com/belyaev-dev/helmdoc/internal/score"
	"github.com/belyaev-dev/helmdoc/internal/testutil/realcharts"
	"github.com/belyaev-dev/helmdoc/pkg/models"
)

func TestRunAllAgainstRealCharts(t *testing.T) {
	manifest := realcharts.LoadManifest(t)
	if len(manifest.Fixtures) != 3 {
		t.Fatalf("len(manifest.Fixtures) = %d, want 3", len(manifest.Fixtures))
	}

	for _, fixture := range manifest.Fixtures {
		fixture := fixture
		t.Run(fixture.ID, func(t *testing.T) {
			ctx := realcharts.AnalysisContext(t, fixture)
			findings := realcharts.RunRules(t, fixture)
			report := score.ComputeReport(findings)
			if ctx.Chart != nil && ctx.Chart.Metadata != nil {
				report.ChartName = ctx.Chart.Metadata.Name
				report.ChartVersion = ctx.Chart.Metadata.Version
			}

			if report.ChartName != fixture.ChartName || report.ChartVersion != fixture.ChartVersion {
				t.Fatalf("report chart metadata = (%q, %q), want (%q, %q)", report.ChartName, report.ChartVersion, fixture.ChartName, fixture.ChartVersion)
			}
			if len(findings) != fixture.Expected.TotalFindings {
				t.Fatalf("len(findings) = %d, want %d (%#v)", len(findings), fixture.Expected.TotalFindings, findings)
			}
			if len(fixture.Expected.FindingTuples) != fixture.Expected.TotalFindings {
				t.Fatalf("len(fixture.Expected.FindingTuples) = %d, want %d", len(fixture.Expected.FindingTuples), fixture.Expected.TotalFindings)
			}
			if len(fixture.Expected.CategorySummaries) != len(models.AllCategories()) {
				t.Fatalf("len(fixture.Expected.CategorySummaries) = %d, want %d", len(fixture.Expected.CategorySummaries), len(models.AllCategories()))
			}

			gotFindings := make([]findingKey, 0, len(findings))
			categoryCounts := map[models.Category]int{}
			for _, finding := range findings {
				gotFindings = append(gotFindings, findingKey{
					RuleID:   finding.RuleID,
					Category: finding.Category,
					Severity: finding.Severity,
					Path:     finding.Path,
					Resource: finding.Resource,
				})
				categoryCounts[finding.Category]++
			}

			wantFindings := make([]findingKey, 0, len(fixture.Expected.FindingTuples))
			wantCategoryCounts := map[models.Category]int{}
			for _, expectedFinding := range fixture.Expected.FindingTuples {
				wantFindings = append(wantFindings, findingKey{
					RuleID:   expectedFinding.RuleID,
					Category: expectedFinding.Category,
					Severity: expectedFinding.Severity,
					Path:     expectedFinding.Path,
					Resource: expectedFinding.Resource,
				})
			}
			for _, summary := range fixture.Expected.CategorySummaries {
				wantCategoryCounts[summary.Category] = summary.Findings
			}

			if !reflect.DeepEqual(gotFindings, wantFindings) {
				t.Fatalf("finding tuples = %#v, want %#v", gotFindings, wantFindings)
			}
			for _, category := range models.AllCategories() {
				if got, want := categoryCounts[category], wantCategoryCounts[category]; got != want {
					t.Fatalf("categoryCounts[%q] = %d, want %d (all=%#v)", category, got, want, categoryCounts)
				}
			}

			switch fixture.ID {
			case "nginx-ingress":
				controllerSecurity := requireFinding(t, findings, findingKey{RuleID: "SEC003", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"})
				if controllerSecurity.Category != models.CategorySecurity {
					t.Fatalf("controllerSecurity.Category = %q, want %q", controllerSecurity.Category, models.CategorySecurity)
				}
				if controllerSecurity.Severity != models.SeverityError {
					t.Fatalf("controllerSecurity.Severity = %v, want %v", controllerSecurity.Severity, models.SeverityError)
				}
				if controllerSecurity.Title != "Container root filesystem is writable" {
					t.Fatalf("controllerSecurity.Title = %q", controllerSecurity.Title)
				}

				controllerLimits := requireFinding(t, findings, findingKey{RuleID: "RES001", Path: "templates/controller-deployment.yaml", Resource: "Deployment/helmdoc-ingress-nginx-controller"})
				if controllerLimits.Remediation != "Set controller.resources.limits in values.yaml. The chart already exposes controller.resources, but its default leaves limits empty." {
					t.Fatalf("controllerLimits.Remediation = %q", controllerLimits.Remediation)
				}

				createSecretRequests := requireFinding(t, findings, findingKey{RuleID: "RES002", Path: "templates/admission-webhooks/job-patch/job-createSecret.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-create"})
				if createSecretRequests.Remediation != "Set controller.admissionWebhooks.createSecretJob.resources.requests in values.yaml. The chart already exposes controller.admissionWebhooks.createSecretJob.resources, but its default leaves requests empty." {
					t.Fatalf("createSecretRequests.Remediation = %q", createSecretRequests.Remediation)
				}

				patchWebhookReadiness := requireFinding(t, findings, findingKey{RuleID: "HLT002", Path: "templates/admission-webhooks/job-patch/job-patchWebhook.yaml", Resource: "Job/helmdoc-ingress-nginx-admission-patch"})
				if patchWebhookReadiness.Remediation != "Add readinessProbe directly to the template for container \"patch\" or expose controller.admissionWebhooks.patchWebhookJob.readinessProbe in values.yaml." {
					t.Fatalf("patchWebhookReadiness.Remediation = %q", patchWebhookReadiness.Remediation)
				}
			case "postgresql":
				unpinnedImage := requireFinding(t, findings, findingKey{RuleID: "IMG002", Path: "templates/primary/statefulset.yaml", Resource: "StatefulSet/helmdoc-postgresql"})
				if unpinnedImage.Title != "Container image is not pinned by digest" {
					t.Fatalf("unpinnedImage.Title = %q", unpinnedImage.Title)
				}
				if unpinnedImage.Description != "container \"postgresql\" in StatefulSet/helmdoc-postgresql uses image \"docker.io/bitnami/postgresql:17.2.0-debian-12-r8\" without a pinned digest." {
					t.Fatalf("unpinnedImage.Description = %q", unpinnedImage.Description)
				}
				if unpinnedImage.Remediation != "Pin container \"postgresql\" to an immutable digest (for example @sha256:...) in the template or values.yaml." {
					t.Fatalf("unpinnedImage.Remediation = %q", unpinnedImage.Remediation)
				}

				missingHPA := requireFinding(t, findings, findingKey{RuleID: "SCL001", Path: "templates/primary/statefulset.yaml", Resource: "StatefulSet/helmdoc-postgresql"})
				if missingHPA.Description != "StatefulSet/helmdoc-postgresql has no matching HorizontalPodAutoscaler in namespace \"default\"." {
					t.Fatalf("missingHPA.Description = %q", missingHPA.Description)
				}
				if missingHPA.Remediation != "Render a HorizontalPodAutoscaler with scaleTargetRef matching StatefulSet/helmdoc-postgresql in namespace \"default\"." {
					t.Fatalf("missingHPA.Remediation = %q", missingHPA.Remediation)
				}
			case "grafana":
				rootFS := requireFinding(t, findings, findingKey{RuleID: "SEC003", Path: "templates/deployment.yaml", Resource: "Deployment/helmdoc-grafana"})
				if rootFS.Title != "Container root filesystem is writable" {
					t.Fatalf("rootFS.Title = %q", rootFS.Title)
				}
				if rootFS.Remediation != "Set container \"grafana\" securityContext.readOnlyRootFilesystem to true." {
					t.Fatalf("rootFS.Remediation = %q", rootFS.Remediation)
				}

				missingRequests := requireFinding(t, findings, findingKey{RuleID: "RES002", Path: "templates/deployment.yaml", Resource: "Deployment/helmdoc-grafana"})
				if missingRequests.Description != "container \"grafana\" in Deployment/helmdoc-grafana does not define resource requests." {
					t.Fatalf("missingRequests.Description = %q", missingRequests.Description)
				}
				if missingRequests.Remediation != "Add resources.requests directly to container \"grafana\" in the template, or expose a values.yaml path for it." {
					t.Fatalf("missingRequests.Remediation = %q", missingRequests.Remediation)
				}

				unpinnedImage := requireFinding(t, findings, findingKey{RuleID: "IMG002", Path: "templates/deployment.yaml", Resource: "Deployment/helmdoc-grafana"})
				if unpinnedImage.Description != "container \"grafana\" in Deployment/helmdoc-grafana uses image \"docker.io/grafana/grafana:12.3.1\" without a pinned digest." {
					t.Fatalf("unpinnedImage.Description = %q", unpinnedImage.Description)
				}

				missingNetworkPolicy := requireFinding(t, findings, findingKey{RuleID: "NET001", Path: "templates/deployment.yaml", Resource: "Deployment/helmdoc-grafana"})
				if missingNetworkPolicy.Remediation != "Enable a chart networkPolicy setting in values.yaml so namespace \"default\" renders at least one NetworkPolicy for Deployment/helmdoc-grafana." {
					t.Fatalf("missingNetworkPolicy.Remediation = %q", missingNetworkPolicy.Remediation)
				}
			default:
				t.Fatalf("unexpected fixture id %q", fixture.ID)
			}

			t.Logf("fixture=%s findings=%d overall_score=%.6f overall_grade=%s", fixture.ID, len(findings), report.OverallScore, report.OverallGrade)
		})
	}
}
