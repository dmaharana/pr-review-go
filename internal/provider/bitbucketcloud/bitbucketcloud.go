package bitbucketcloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"pr-review-go/internal/config"
	"pr-review-go/internal/diff"
	"pr-review-go/internal/domain"
)

const defaultBaseURL = "https://api.bitbucket.org/2.0"

// Provider implements GitProvider for Bitbucket Cloud REST 2.0.
type Provider struct {
	cfg        config.BitbucketCloudConfig
	httpClient *http.Client
	baseURL    string
}

// NewProvider creates a new Bitbucket Cloud provider.
func NewProvider(cfg config.BitbucketCloudConfig) *Provider {
	return &Provider{
		cfg: cfg,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		baseURL: defaultBaseURL,
	}
}

// NewProviderWithBaseURL allows pointing to a mock server for testing.
func NewProviderWithBaseURL(cfg config.BitbucketCloudConfig, baseURL string) *Provider {
	p := NewProvider(cfg)
	p.baseURL = strings.TrimSuffix(baseURL, "/")
	return p
}

func (p *Provider) doRequest(ctx context.Context, method, endpoint string, body any) (*http.Response, error) {
	url := p.baseURL + endpoint
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("creating http request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if p.cfg.Username != "" && p.cfg.AppPassword != "" {
		req.SetBasicAuth(p.cfg.Username, p.cfg.AppPassword)
	}

	return p.httpClient.Do(req)
}

func (p *Provider) GetPullRequest(ctx context.Context, workspace, repo string, number int) (*domain.PullRequest, error) {
	endpoint := fmt.Sprintf("/repositories/%s/%s/pullrequests/%d", workspace, repo, number)
	resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching bitbucket cloud pr: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bitbucket cloud pr returned status %d", resp.StatusCode)
	}

	var raw struct {
		ID          int64  `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		Author      struct {
			Nickname    string `json:"nickname"`
			DisplayName string `json:"display_name"`
		} `json:"author"`
		Source struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
			Commit struct {
				Hash string `json:"hash"`
			} `json:"commit"`
		} `json:"source"`
		Destination struct {
			Branch struct {
				Name string `json:"name"`
			} `json:"branch"`
			Commit struct {
				Hash string `json:"hash"`
			} `json:"commit"`
		} `json:"destination"`
		Links struct {
			HTML struct {
				Href string `json:"href"`
			} `json:"html"`
		} `json:"links"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decoding bitbucket cloud pr: %w", err)
	}

	author := raw.Author.Nickname
	if author == "" {
		author = raw.Author.DisplayName
	}

	return &domain.PullRequest{
		ID:          raw.ID,
		Number:      number,
		Title:       raw.Title,
		Description: raw.Description,
		Author:      author,
		HeadBranch:  raw.Source.Branch.Name,
		HeadSHA:     raw.Source.Commit.Hash,
		BaseBranch:  raw.Destination.Branch.Name,
		BaseSHA:     raw.Destination.Commit.Hash,
		RepoOwner:   workspace,
		RepoName:    repo,
		URL:         raw.Links.HTML.Href,
	}, nil
}

func (p *Provider) GetDiff(ctx context.Context, pr *domain.PullRequest) ([]domain.DiffFile, error) {
	endpoint := fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/diff", pr.RepoOwner, pr.RepoName, pr.Number)
	resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("fetching bitbucket cloud diff: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fetching bitbucket diff returned status %d", resp.StatusCode)
	}

	diffBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading bitbucket diff: %w", err)
	}

	return diff.ParseUnifiedDiff(string(diffBytes))
}

func (p *Provider) GetRepoConfigFile(ctx context.Context, pr *domain.PullRequest) ([]byte, error) {
	for _, name := range []string{".pr-agent.yaml", ".pr-agent.yml", ".pr_agent.toml"} {
		endpoint := fmt.Sprintf("/repositories/%s/%s/src/%s/%s", pr.RepoOwner, pr.RepoName, pr.HeadSHA, name)
		resp, err := p.doRequest(ctx, http.MethodGet, endpoint, nil)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			return io.ReadAll(resp.Body)
		}
		if resp != nil {
			resp.Body.Close()
		}
	}
	return nil, nil
}

func (p *Provider) PublishComment(ctx context.Context, pr *domain.PullRequest, body string) error {
	endpoint := fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/comments", pr.RepoOwner, pr.RepoName, pr.Number)
	payload := map[string]any{
		"content": map[string]string{
			"raw": body,
		},
	}
	resp, err := p.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return fmt.Errorf("posting comment to bitbucket cloud: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("posting bitbucket cloud comment returned status %d", resp.StatusCode)
	}
	return nil
}

func (p *Provider) PublishInlineComments(ctx context.Context, pr *domain.PullRequest, findings []domain.ReviewFinding) error {
	for _, f := range findings {
		line := f.EndLine
		if line <= 0 {
			line = f.StartLine
		}
		if line <= 0 {
			continue
		}

		var commentBody strings.Builder
		commentBody.WriteString(fmt.Sprintf("**[%s] %s**\n\n%s", strings.ToUpper(string(f.Severity)), f.IssueTitle, f.Description))
		if f.HasCodeSuggestion() {
			commentBody.WriteString(fmt.Sprintf("\n\n```diff\n@@ -%d,%d +%d,%d @@\n-%s\n+%s\n```",
				f.StartLine, f.EndLine-f.StartLine+1, f.StartLine, f.EndLine-f.StartLine+1,
				f.ExistingCode, f.SuggestedCode))
		}

		endpoint := fmt.Sprintf("/repositories/%s/%s/pullrequests/%d/comments", pr.RepoOwner, pr.RepoName, pr.Number)
		payload := map[string]any{
			"content": map[string]string{
				"raw": commentBody.String(),
			},
			"inline": map[string]any{
				"to":   line,
				"path": f.File,
			},
		}

		resp, err := p.doRequest(ctx, http.MethodPost, endpoint, payload)
		if err != nil {
			return fmt.Errorf("posting bitbucket cloud inline comment: %w", err)
		}
		resp.Body.Close()
	}
	return nil
}

func (p *Provider) SetCommitStatus(ctx context.Context, pr *domain.PullRequest, verdict domain.ReviewVerdict, summary string) error {
	state := "SUCCESSFUL"
	switch verdict {
	case domain.VerdictChangesRequired:
		state = "FAILED"
	case domain.VerdictNeedsReview:
		state = "INPROGRESS"
	}

	endpoint := fmt.Sprintf("/repositories/%s/%s/commit/%s/statuses/build", pr.RepoOwner, pr.RepoName, pr.HeadSHA)
	payload := map[string]any{
		"key":         "pr-review-go",
		"state":       state,
		"name":        "AI Pull Request Review",
		"description": summary,
		"url":         pr.URL,
	}

	resp, err := p.doRequest(ctx, http.MethodPost, endpoint, payload)
	if err != nil {
		return fmt.Errorf("setting bitbucket cloud commit status: %w", err)
	}
	defer resp.Body.Close()
	return nil
}

func (p *Provider) SetLabels(ctx context.Context, pr *domain.PullRequest, labels []string) error {
	// Bitbucket Cloud pull requests do not support labels natively via REST API; safe no-op
	return nil
}
