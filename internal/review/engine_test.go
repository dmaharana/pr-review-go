package review

import (
	"context"
	"testing"

	"pr-review-go/internal/config"
	"pr-review-go/internal/domain"
)

type mockLLMClient struct {
	response string
}

func (m *mockLLMClient) Complete(ctx context.Context, sys, user string) (string, error) {
	return m.response, nil
}

type mockProvider struct {
	pr                *domain.PullRequest
	diffFiles         []domain.DiffFile
	publishedComment  string
	inlineFindings    []domain.ReviewFinding
	commitStatus      domain.ReviewVerdict
	labels            []string
}

func (m *mockProvider) GetPullRequest(ctx context.Context, owner, repo string, number int) (*domain.PullRequest, error) {
	return m.pr, nil
}

func (m *mockProvider) GetDiff(ctx context.Context, pr *domain.PullRequest) ([]domain.DiffFile, error) {
	return m.diffFiles, nil
}

func (m *mockProvider) GetRepoConfigFile(ctx context.Context, pr *domain.PullRequest) ([]byte, error) {
	return nil, nil
}

func (m *mockProvider) PublishComment(ctx context.Context, pr *domain.PullRequest, body string) error {
	m.publishedComment = body
	return nil
}

func (m *mockProvider) PublishInlineComments(ctx context.Context, pr *domain.PullRequest, findings []domain.ReviewFinding) error {
	m.inlineFindings = findings
	return nil
}

func (m *mockProvider) SetCommitStatus(ctx context.Context, pr *domain.PullRequest, verdict domain.ReviewVerdict, summary string) error {
	m.commitStatus = verdict
	return nil
}

func (m *mockProvider) SetLabels(ctx context.Context, pr *domain.PullRequest, labels []string) error {
	m.labels = labels
	return nil
}

func TestEngineReview(t *testing.T) {
	mockJSON := `{
		"verdict": "changes_required",
		"summary": "Found unhandled error.",
		"risk_assessment": "high",
		"effort_to_review": 2,
		"priority_files": ["server.go"],
		"findings": [
			{
				"file": "server.go",
				"start_line": 20,
				"end_line": 22,
				"issue_title": "Unhandled Error",
				"description": "JSON encoding error ignored",
				"severity": "high",
				"existing_code": "json.NewEncoder(w).Encode(resp)",
				"suggested_code": "if err := json.NewEncoder(w).Encode(resp); err != nil { log.Println(err) }"
			}
		]
	}`

	cfg := config.DefaultConfig()
	llm := &mockLLMClient{response: mockJSON}
	prov := &mockProvider{
		pr: &domain.PullRequest{
			Number:      101,
			Title:       "Add API handler",
			Description: "Adds a new server endpoint",
			Author:      "dev",
			RepoOwner:   "org",
			RepoName:    "app",
			HeadSHA:     "123abc456",
		},
		diffFiles: []domain.DiffFile{
			{
				NewPath: "server.go",
				Patch:   "@@ -20,2 +20,2 @@\n+ json.NewEncoder(w).Encode(resp)",
			},
		},
	}

	engine := NewEngine(cfg, llm, prov)
	out, err := engine.Review(context.Background(), prov.pr)
	if err != nil {
		t.Fatalf("Review failed: %v", err)
	}

	if out.Verdict != domain.VerdictChangesRequired {
		t.Errorf("got verdict %v, want changes_required", out.Verdict)
	}
	if len(out.Findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(out.Findings))
	}
	if prov.commitStatus != domain.VerdictChangesRequired {
		t.Errorf("expected commit status changes_required, got %v", prov.commitStatus)
	}
	if prov.publishedComment == "" {
		t.Errorf("expected published summary comment, got empty")
	}
}

func TestEngineDescribe(t *testing.T) {
	mockJSON := `{
		"title": "feat(server): add API endpoint",
		"type": "feat",
		"summary": "Introduces new REST handler",
		"walkthrough": "- Added server.go handler",
		"key_changes": ["Added endpoint"],
		"suggested_tags": ["backend"]
	}`

	cfg := config.DefaultConfig()
	llm := &mockLLMClient{response: mockJSON}
	prov := &mockProvider{
		pr: &domain.PullRequest{Number: 1, Title: "PR Title"},
		diffFiles: []domain.DiffFile{
			{NewPath: "server.go", Patch: "+ func New() {}"},
		},
	}

	engine := NewEngine(cfg, llm, prov)
	desc, err := engine.Describe(context.Background(), prov.pr)
	if err != nil {
		t.Fatalf("Describe failed: %v", err)
	}

	if desc.Type != "feat" || desc.Title != "feat(server): add API endpoint" {
		t.Errorf("unexpected describe output: %+v", desc)
	}
}

func TestEngineAsk(t *testing.T) {
	mockJSON := `{
		"answer": "This method parses incoming JSON payloads.",
		"references": ["server.go:20-25"]
	}`

	cfg := config.DefaultConfig()
	llm := &mockLLMClient{response: mockJSON}
	prov := &mockProvider{
		pr: &domain.PullRequest{Number: 1, Title: "PR Title"},
		diffFiles: []domain.DiffFile{
			{NewPath: "server.go", Patch: "+ func New() {}"},
		},
	}

	engine := NewEngine(cfg, llm, prov)
	resp, err := engine.Ask(context.Background(), prov.pr, "What does this method do?")
	if err != nil {
		t.Fatalf("Ask failed: %v", err)
	}

	if resp.Answer != "This method parses incoming JSON payloads." {
		t.Errorf("unexpected answer: %q", resp.Answer)
	}
}

func TestEngineGenerateLabels(t *testing.T) {
	cfg := config.DefaultConfig()
	prov := &mockProvider{
		pr: &domain.PullRequest{Number: 1},
		diffFiles: []domain.DiffFile{
			{NewPath: "server.go"},
			{NewPath: "server_test.go"},
			{NewPath: "docs/readme.md"},
		},
	}

	engine := NewEngine(cfg, nil, prov)
	labels, err := engine.GenerateLabels(context.Background(), prov.pr)
	if err != nil {
		t.Fatalf("GenerateLabels failed: %v", err)
	}

	expected := map[string]bool{"backend": true, "tests": true, "documentation": true}
	for _, l := range labels {
		if !expected[l] {
			t.Errorf("unexpected label: %s", l)
		}
	}
}
