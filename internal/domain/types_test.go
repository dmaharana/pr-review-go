package domain

import (
	"encoding/json"
	"testing"
)

func TestDisplayPath(t *testing.T) {
	tests := []struct {
		name     string
		file     DiffFile
		expected string
	}{
		{
			name:     "new path present",
			file:     DiffFile{OldPath: "a/old.go", NewPath: "b/new.go"},
			expected: "b/new.go",
		},
		{
			name:     "deleted file",
			file:     DiffFile{OldPath: "a/deleted.go", NewPath: "/dev/null"},
			expected: "a/deleted.go",
		},
		{
			name:     "empty new path",
			file:     DiffFile{OldPath: "a/deleted.go", NewPath: ""},
			expected: "a/deleted.go",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.file.DisplayPath()
			if got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestHasCodeSuggestion(t *testing.T) {
	findingWithSuggestion := ReviewFinding{
		ExistingCode:  "if err != nil { return }",
		SuggestedCode: "if err != nil { return fmt.Errorf(\"failed: %w\", err) }",
	}
	if !findingWithSuggestion.HasCodeSuggestion() {
		t.Errorf("expected finding to have code suggestion")
	}

	findingWithoutSuggestion := ReviewFinding{
		ExistingCode:  "",
		SuggestedCode: "",
	}
	if findingWithoutSuggestion.HasCodeSuggestion() {
		t.Errorf("expected finding to NOT have code suggestion")
	}
}

func TestReviewOutputJSON(t *testing.T) {
	out := ReviewOutput{
		Verdict:        VerdictChangesRequired,
		Summary:        "Potential SQL injection found",
		RiskAssessment: "high",
		EffortToReview: 3,
		Findings: []ReviewFinding{
			{
				File:        "db.go",
				StartLine:   10,
				EndLine:     12,
				IssueTitle:  "SQL Injection",
				Description: "Unsanitized user input formatted directly into query string",
				Severity:    SeverityCritical,
			},
		},
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ReviewOutput
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Verdict != VerdictChangesRequired {
		t.Errorf("got verdict %v, want %v", decoded.Verdict, VerdictChangesRequired)
	}
	if len(decoded.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(decoded.Findings))
	}
	if decoded.Findings[0].Severity != SeverityCritical {
		t.Errorf("got severity %v, want %v", decoded.Findings[0].Severity, SeverityCritical)
	}
}
