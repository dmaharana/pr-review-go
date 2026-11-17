package bitbucketserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"pr-review-go/internal/config"
	"pr-review-go/internal/domain"
)

func TestBitbucketServerProvider(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/rest/api/1.0/projects/PROJ/repos/repo1/pull-requests/5":
			w.Write([]byte(`{
				"id": 5,
				"title": "Fix memory safety",
				"description": "Fixes buffer overrun",
				"author": {"user": {"name": "bob", "displayName": "Bob Builder"}},
				"fromRef": {"id": "refs/heads/fix", "displayId": "fix", "latestCommit": "aaaa1111"},
				"toRef": {"id": "refs/heads/master", "displayId": "master", "latestCommit": "bbbb2222"},
				"links": {"self": [{"href": "http://bitbucket.internal/projects/PROJ/repos/repo1/pull-requests/5"}]}
			}`))

		case "/rest/api/1.0/projects/PROJ/repos/repo1/pull-requests/5/comments":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 999}`))

		case "/rest/build-status/1.0/commits/aaaa1111":
			w.WriteHeader(http.StatusNoContent)

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	cfg := config.BitbucketServerConfig{
		BaseURL: mockServer.URL,
		Token:   "pat-token-secret",
	}

	p := NewProvider(cfg)

	pr, err := p.GetPullRequest(context.Background(), "PROJ", "repo1", 5)
	if err != nil {
		t.Fatalf("GetPullRequest failed: %v", err)
	}

	if pr.Number != 5 || pr.Author != "bob" || pr.HeadSHA != "aaaa1111" {
		t.Errorf("unexpected pr: %+v", pr)
	}

	err = p.PublishComment(context.Background(), pr, "Review summary here")
	if err != nil {
		t.Fatalf("PublishComment failed: %v", err)
	}

	findings := []domain.ReviewFinding{
		{
			File:          "buf.c",
			StartLine:     20,
			EndLine:       22,
			IssueTitle:    "Buffer overflow",
			Description:   "No bounds checking on write",
			Severity:      domain.SeverityCritical,
			ExistingCode:  "memcpy(dst, src, len);",
			SuggestedCode: "if (len <= max) memcpy(dst, src, len);",
		},
	}
	err = p.PublishInlineComments(context.Background(), pr, findings)
	if err != nil {
		t.Fatalf("PublishInlineComments failed: %v", err)
	}

	err = p.SetCommitStatus(context.Background(), pr, domain.VerdictChangesRequired, "Critical issue found")
	if err != nil {
		t.Fatalf("SetCommitStatus failed: %v", err)
	}
}
