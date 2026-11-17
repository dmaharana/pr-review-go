package bitbucketcloud

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"pr-review-go/internal/config"
	"pr-review-go/internal/domain"
)

func TestBitbucketCloudProvider(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/repositories/myworkspace/myrepo/pullrequests/10":
			w.Write([]byte(`{
				"id": 10,
				"title": "Add caching",
				"description": "Implements LRU cache",
				"author": {"nickname": "alice"},
				"source": {"branch": {"name": "feature"}, "commit": {"hash": "c0ffee1"}},
				"destination": {"branch": {"name": "main"}, "commit": {"hash": "deadbeef"}},
				"links": {"html": {"href": "https://bitbucket.org/myworkspace/myrepo/pull-requests/10"}}
			}`))

		case "/repositories/myworkspace/myrepo/pullrequests/10/comments":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 123}`))

		case "/repositories/myworkspace/myrepo/commit/c0ffee1/statuses/build":
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{}`))

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	cfg := config.BitbucketCloudConfig{
		Username:    "alice",
		AppPassword: "secret-password",
	}

	p := NewProviderWithBaseURL(cfg, mockServer.URL)

	pr, err := p.GetPullRequest(context.Background(), "myworkspace", "myrepo", 10)
	if err != nil {
		t.Fatalf("GetPullRequest failed: %v", err)
	}

	if pr.Number != 10 || pr.Author != "alice" || pr.HeadSHA != "c0ffee1" {
		t.Errorf("unexpected pr fetched: %+v", pr)
	}

	err = p.PublishComment(context.Background(), pr, "Review summary")
	if err != nil {
		t.Fatalf("PublishComment failed: %v", err)
	}

	findings := []domain.ReviewFinding{
		{
			File:          "cache.go",
			StartLine:     15,
			EndLine:       17,
			IssueTitle:    "Race condition",
			Description:   "Locking omitted on cache write",
			Severity:      domain.SeverityHigh,
			ExistingCode:  "c.m[k] = v",
			SuggestedCode: "c.mu.Lock()\nc.m[k] = v\nc.mu.Unlock()",
		},
	}
	err = p.PublishInlineComments(context.Background(), pr, findings)
	if err != nil {
		t.Fatalf("PublishInlineComments failed: %v", err)
	}

	err = p.SetCommitStatus(context.Background(), pr, domain.VerdictNoConcernsFound, "Clean review")
	if err != nil {
		t.Fatalf("SetCommitStatus failed: %v", err)
	}
}
