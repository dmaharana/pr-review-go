package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/go-github/v62/github"
	"pr-review-go/internal/config"
	"pr-review-go/internal/domain"
)

func TestGitHubProvider(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/v3/repos/octocat/hello-world/pulls/42":
			title := "Fix memory leak"
			body := "Resolves unclosed file descriptors"
			user := "octocat"
			headRef := "fix-leak"
			headSHA := "abc1234"
			baseRef := "main"
			baseSHA := "def5678"
			htmlURL := "https://github.com/octocat/hello-world/pull/42"
			createdAt := time.Now()
			updatedAt := time.Now()

			pr := github.PullRequest{
				Number:    github.Int(42),
				Title:     &title,
				Body:      &body,
				User:      &github.User{Login: &user},
				Head:      &github.PullRequestBranch{Ref: &headRef, SHA: &headSHA},
				Base:      &github.PullRequestBranch{Ref: &baseRef, SHA: &baseSHA},
				HTMLURL:   &htmlURL,
				CreatedAt: &github.Timestamp{Time: createdAt},
				UpdatedAt: &github.Timestamp{Time: updatedAt},
			}
			json.NewEncoder(w).Encode(pr)

		case "/api/v3/repos/octocat/hello-world/issues/42/comments":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(github.IssueComment{})

		case "/api/v3/repos/octocat/hello-world/statuses/abc1234":
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(github.RepoStatus{})

		case "/api/v3/repos/octocat/hello-world/issues/42/labels":
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]*github.Label{})

		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer mockServer.Close()

	cfg := config.GitHubConfig{
		BaseURL: mockServer.URL + "/api/v3",
		Token:   "dummy-token",
	}

	p, err := NewProvider(context.Background(), cfg)
	if err != nil {
		t.Fatalf("NewProvider failed: %v", err)
	}

	pr, err := p.GetPullRequest(context.Background(), "octocat", "hello-world", 42)
	if err != nil {
		t.Fatalf("GetPullRequest failed: %v", err)
	}

	if pr.Number != 42 || pr.Title != "Fix memory leak" {
		t.Errorf("unexpected pr fetched: %+v", pr)
	}

	err = p.PublishComment(context.Background(), pr, "Summary comment")
	if err != nil {
		t.Fatalf("PublishComment failed: %v", err)
	}

	err = p.SetCommitStatus(context.Background(), pr, domain.VerdictNoConcernsFound, "Review clean")
	if err != nil {
		t.Fatalf("SetCommitStatus failed: %v", err)
	}

	err = p.SetLabels(context.Background(), pr, []string{"reviewed", "ready-to-merge"})
	if err != nil {
		t.Fatalf("SetLabels failed: %v", err)
	}
}
