package models

import (
	"encoding/json"
	"testing"
)

func TestSeverityJSON(t *testing.T) {
	type payload struct {
		Severity Severity `json:"severity"`
	}

	tests := []struct {
		name     string
		severity Severity
		wantJSON string
	}{
		{name: "info", severity: SeverityInfo, wantJSON: `{"severity":"info"}`},
		{name: "warning", severity: SeverityWarning, wantJSON: `{"severity":"warning"}`},
		{name: "error", severity: SeverityError, wantJSON: `{"severity":"error"}`},
		{name: "critical", severity: SeverityCritical, wantJSON: `{"severity":"critical"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := json.Marshal(payload{Severity: tt.severity})
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if got := string(encoded); got != tt.wantJSON {
				t.Fatalf("json.Marshal() = %s, want %s", got, tt.wantJSON)
			}

			var decoded payload
			if err := json.Unmarshal(encoded, &decoded); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if decoded.Severity != tt.severity {
				t.Fatalf("decoded severity = %v, want %v", decoded.Severity, tt.severity)
			}
			if decoded.Severity.String() != tt.name {
				t.Fatalf("decoded severity string = %q, want %q", decoded.Severity.String(), tt.name)
			}
		})
	}
}

func TestGradeRank(t *testing.T) {
	tests := []struct {
		grade Grade
		want  int
	}{
		{grade: GradeA, want: 4},
		{grade: GradeB, want: 3},
		{grade: GradeC, want: 2},
		{grade: GradeD, want: 1},
		{grade: GradeF, want: 0},
		{grade: Grade("bad"), want: -1},
	}

	for _, tt := range tests {
		if got := tt.grade.Rank(); got != tt.want {
			t.Fatalf("%q.Rank() = %d, want %d", tt.grade, got, tt.want)
		}
	}
}
