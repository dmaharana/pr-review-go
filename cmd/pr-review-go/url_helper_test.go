package main

import (
	"testing"
)

func TestParsePRURL(t *testing.T) {
	tests := []struct {
		url          string
		wantProvider string
		wantOwner    string
		wantRepo     string
		wantNumber   int
	}{
		{
			url:          "https://github.com/octocat/hello-world/pull/42",
			wantProvider: "github",
			wantOwner:    "octocat",
			wantRepo:     "hello-world",
			wantNumber:   42,
		},
		{
			url:          "https://github.corp.internal/octocat/hello-world/pull/99",
			wantProvider: "github",
			wantOwner:    "octocat",
			wantRepo:     "hello-world",
			wantNumber:   99,
		},
		{
			url:          "https://bitbucket.org/myorg/awesome-repo/pull-requests/15",
			wantProvider: "bitbucket_cloud",
			wantOwner:    "myorg",
			wantRepo:     "awesome-repo",
			wantNumber:   15,
		},
		{
			url:          "https://bitbucket.corp.internal/projects/PROJ/repos/backend/pull-requests/7",
			wantProvider: "bitbucket_server",
			wantOwner:    "PROJ",
			wantRepo:     "backend",
			wantNumber:   7,
		},
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			p, err := parsePRURL(tc.url)
			if err != nil {
				t.Fatalf("parsePRURL failed: %v", err)
			}
			if p.Provider != tc.wantProvider {
				t.Errorf("got provider %q, want %q", p.Provider, tc.wantProvider)
			}
			if p.Owner != tc.wantOwner {
				t.Errorf("got owner %q, want %q", p.Owner, tc.wantOwner)
			}
			if p.Repo != tc.wantRepo {
				t.Errorf("got repo %q, want %q", p.Repo, tc.wantRepo)
			}
			if p.Number != tc.wantNumber {
				t.Errorf("got number %d, want %d", p.Number, tc.wantNumber)
			}
		})
	}
}
