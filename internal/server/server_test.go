package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"pr-review-go/internal/config"
)

type dummyLLM struct{}

func (d *dummyLLM) Complete(ctx context.Context, sys, user string) (string, error) {
	return `{"verdict": "no_concerns_found", "summary": "Looks good"}`, nil
}

func TestHealthzEndpoint(t *testing.T) {
	cfg := config.DefaultConfig()
	srv := NewServer(cfg, &dummyLLM{})

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if w.Body.String() != "ok\n" {
		t.Errorf("got body %q, want ok\n", w.Body.String())
	}
}

func TestGitHubWebhookHandler(t *testing.T) {
	cfg := config.DefaultConfig()
	srv := NewServer(cfg, &dummyLLM{})

	// 1. Ignored event
	ignoredPayload := []byte(`{"action": "labeled"}`)
	req := httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(ignoredPayload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want 200", w.Code)
	}

	// 2. Opened event -> Accepted (202)
	openedPayload := []byte(`{
		"action": "opened",
		"number": 12,
		"repository": {
			"owner": {"login": "octocat"},
			"name": "hello-world"
		}
	}`)
	req = httptest.NewRequest(http.MethodPost, "/webhooks/github", bytes.NewReader(openedPayload))
	req.Header.Set("X-GitHub-Event", "pull_request")
	w = httptest.NewRecorder()
	srv.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Errorf("got status %d, want 202 Accepted", w.Code)
	}
}

func TestDebounceRegistry(t *testing.T) {
	reg := NewDebounceRegistry()

	ctx1, cancel1 := reg.Start(context.Background(), "github", "org/repo", "1")
	defer cancel1()

	if ctx1.Err() != nil {
		t.Errorf("ctx1 should be active")
	}

	// New start on same PR cancels ctx1
	ctx2, cancel2 := reg.Start(context.Background(), "github", "org/repo", "1")
	defer cancel2()

	if ctx1.Err() == nil {
		t.Errorf("ctx1 should have been cancelled by ctx2")
	}
	if ctx2.Err() != nil {
		t.Errorf("ctx2 should be active")
	}
}

func TestWorkerPool(t *testing.T) {
	pool := NewWorkerPool(2, 5)
	defer pool.Shutdown()

	done := make(chan bool)
	ok := pool.Enqueue(func() {
		time.Sleep(10 * time.Millisecond)
		done <- true
	})

	if !ok {
		t.Fatalf("failed to enqueue job")
	}

	select {
	case <-done:
		// success
	case <-time.After(1 * time.Second):
		t.Fatalf("timed out waiting for worker job")
	}
}
