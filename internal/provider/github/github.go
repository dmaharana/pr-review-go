package github

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
	"pr-review-go/internal/config"
	"pr-review-go/internal/diff"
	"pr-review-go/internal/domain"
)

// Provider implements GitProvider for GitHub SaaS and GitHub Enterprise Server.
type Provider struct {
	cfg    config.GitHubConfig
	client *github.Client
}

// NewProvider creates a new GitHub provider.
func NewProvider(ctx context.Context, cfg config.GitHubConfig) (*Provider, error) {
	var httpClient *http.Client

	if cfg.Token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cfg.Token})
		httpClient = oauth2.NewClient(ctx, ts)
	} else if cfg.AppID > 0 {
		// App authentication
		keyBytes := []byte(cfg.AppPrivateKey)
		if len(keyBytes) == 0 && cfg.AppPrivateKeyPath != "" {
			var err error
			keyBytes, err = os.ReadFile(cfg.AppPrivateKeyPath)
			if err != nil {
				return nil, fmt.Errorf("reading app private key file: %w", err)
			}
		}
		if len(keyBytes) == 0 {
			return nil, fmt.Errorf("github app authentication specified but no private key provided")
		}

		appTransport, err := newAppTransport(cfg.AppID, keyBytes)
		if err != nil {
			return nil, fmt.Errorf("initializing github app transport: %w", err)
		}
		httpClient = &http.Client{Transport: appTransport}
	} else {
		// Unauthenticated client (for public repos)
		httpClient = http.DefaultClient
	}

	var client *github.Client
	var err error
	if cfg.BaseURL != "" {
		baseURL := strings.TrimSuffix(cfg.BaseURL, "/") + "/"
		client, err = github.NewEnterpriseClient(baseURL, baseURL, httpClient)
		if err != nil {
			return nil, fmt.Errorf("creating enterprise github client: %w", err)
		}
	} else {
		client = github.NewClient(httpClient)
	}

	return &Provider{
		cfg:    cfg,
		client: client,
	}, nil
}

func (p *Provider) GetPullRequest(ctx context.Context, owner, repo string, number int) (*domain.PullRequest, error) {
	ghPR, _, err := p.client.PullRequests.Get(ctx, owner, repo, number)
	if err != nil {
		return nil, fmt.Errorf("fetching github pr #%d: %w", number, err)
	}

	var labels []string
	for _, l := range ghPR.Labels {
		if l.Name != nil {
			labels = append(labels, *l.Name)
		}
	}

	author := ""
	if ghPR.User != nil && ghPR.User.Login != nil {
		author = *ghPR.User.Login
	}

	title := ""
	if ghPR.Title != nil {
		title = *ghPR.Title
	}

	desc := ""
	if ghPR.Body != nil {
		desc = *ghPR.Body
	}

	headBranch, headSHA := "", ""
	if ghPR.Head != nil {
		if ghPR.Head.Ref != nil {
			headBranch = *ghPR.Head.Ref
		}
		if ghPR.Head.SHA != nil {
			headSHA = *ghPR.Head.SHA
		}
	}

	baseBranch, baseSHA := "", ""
	if ghPR.Base != nil {
		if ghPR.Base.Ref != nil {
			baseBranch = *ghPR.Base.Ref
		}
		if ghPR.Base.SHA != nil {
			baseSHA = *ghPR.Base.SHA
		}
	}

	return &domain.PullRequest{
		ID:          ghPR.GetID(),
		Number:      number,
		Title:       title,
		Description: desc,
		Author:      author,
		HeadBranch:  headBranch,
		HeadSHA:     headSHA,
		BaseBranch:  baseBranch,
		BaseSHA:     baseSHA,
		RepoOwner:   owner,
		RepoName:    repo,
		URL:         ghPR.GetHTMLURL(),
		Labels:      labels,
		CreatedAt:   ghPR.GetCreatedAt().Time,
		UpdatedAt:   ghPR.GetUpdatedAt().Time,
	}, nil
}

func (p *Provider) GetDiff(ctx context.Context, pr *domain.PullRequest) ([]domain.DiffFile, error) {
	rawDiff, _, err := p.client.PullRequests.GetRaw(ctx, pr.RepoOwner, pr.RepoName, pr.Number, github.RawOptions{
		Type: github.Diff,
	})
	if err != nil {
		return nil, fmt.Errorf("fetching pr diff from github: %w", err)
	}

	return diff.ParseUnifiedDiff(rawDiff)
}

func (p *Provider) GetRepoConfigFile(ctx context.Context, pr *domain.PullRequest) ([]byte, error) {
	for _, name := range []string{".pr-agent.yaml", ".pr-agent.yml", ".pr_agent.toml"} {
		fileContent, _, _, err := p.client.Repositories.GetContents(ctx, pr.RepoOwner, pr.RepoName, name, &github.RepositoryContentGetOptions{
			Ref: pr.HeadSHA,
		})
		if err == nil && fileContent != nil {
			content, err := fileContent.GetContent()
			if err == nil {
				return []byte(content), nil
			}
		}
	}
	return nil, nil
}

func (p *Provider) PublishComment(ctx context.Context, pr *domain.PullRequest, body string) error {
	comment := &github.IssueComment{
		Body: &body,
	}
	_, _, err := p.client.Issues.CreateComment(ctx, pr.RepoOwner, pr.RepoName, pr.Number, comment)
	if err != nil {
		return fmt.Errorf("posting comment on github pr #%d: %w", pr.Number, err)
	}
	return nil
}

func (p *Provider) PublishInlineComments(ctx context.Context, pr *domain.PullRequest, findings []domain.ReviewFinding) error {
	if len(findings) == 0 {
		return nil
	}

	var comments []*github.DraftReviewComment
	for _, f := range findings {
		line := f.EndLine
		if line <= 0 {
			line = f.StartLine
		}
		if line <= 0 {
			continue
		}

		var bodyBuilder strings.Builder
		bodyBuilder.WriteString(fmt.Sprintf("**[%s] %s**\n\n%s", strings.ToUpper(string(f.Severity)), f.IssueTitle, f.Description))

		if f.HasCodeSuggestion() {
			bodyBuilder.WriteString(fmt.Sprintf("\n\n```suggestion\n%s\n```", f.SuggestedCode))
		}

		commentBody := bodyBuilder.String()
		filePath := f.File
		side := "RIGHT"

		draft := &github.DraftReviewComment{
			Path: &filePath,
			Line: &line,
			Side: &side,
			Body: &commentBody,
		}
		if f.StartLine > 0 && f.StartLine < f.EndLine {
			startLine := f.StartLine
			draft.StartLine = &startLine
			draft.StartSide = &side
		}
		comments = append(comments, draft)
	}

	if len(comments) == 0 {
		return nil
	}

	event := "COMMENT"
	reviewReq := &github.PullRequestReviewRequest{
		CommitID: &pr.HeadSHA,
		Event:    &event,
		Comments: comments,
	}

	_, _, err := p.client.PullRequests.CreateReview(ctx, pr.RepoOwner, pr.RepoName, pr.Number, reviewReq)
	if err != nil {
		return fmt.Errorf("creating github review with %d inline comments: %w", len(comments), err)
	}
	return nil
}

func (p *Provider) SetCommitStatus(ctx context.Context, pr *domain.PullRequest, verdict domain.ReviewVerdict, summary string) error {
	state := "success"
	switch verdict {
	case domain.VerdictChangesRequired:
		state = "failure"
	case domain.VerdictNeedsReview:
		state = "pending"
	}

	contextName := "pr-review-go"
	status := &github.RepoStatus{
		State:       &state,
		Description: &summary,
		Context:     &contextName,
	}

	_, _, err := p.client.Repositories.CreateStatus(ctx, pr.RepoOwner, pr.RepoName, pr.HeadSHA, status)
	if err != nil {
		return fmt.Errorf("setting commit status on github %s: %w", pr.HeadSHA, err)
	}
	return nil
}

func (p *Provider) SetLabels(ctx context.Context, pr *domain.PullRequest, labels []string) error {
	if len(labels) == 0 {
		return nil
	}
	_, _, err := p.client.Issues.AddLabelsToIssue(ctx, pr.RepoOwner, pr.RepoName, pr.Number, labels)
	if err != nil {
		return fmt.Errorf("adding labels to github pr: %w", err)
	}
	return nil
}

// appTransport signs requests with an RS256 JWT for GitHub App installation.
type appTransport struct {
	appID      int64
	privateKey *rsa.PrivateKey
	base       http.RoundTripper
}

func newAppTransport(appID int64, pemKey []byte) (*appTransport, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, fmt.Errorf("no RSA private key found in PEM data")
	}

	var key *rsa.PrivateKey
	var err error
	if k, e := x509.ParsePKCS1PrivateKey(block.Bytes); e == nil {
		key = k
	} else if pkcs8, e := x509.ParsePKCS8PrivateKey(block.Bytes); e == nil {
		if k, ok := pkcs8.(*rsa.PrivateKey); ok {
			key = k
		} else {
			return nil, fmt.Errorf("PKCS8 key is not RSA")
		}
	} else {
		return nil, fmt.Errorf("parsing RSA key failed: %w", err)
	}

	return &appTransport{
		appID:      appID,
		privateKey: key,
		base:       http.DefaultTransport,
	}, nil
}

func (t *appTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		IssuedAt:  jwt.NewNumericDate(now.Add(-60 * time.Second)),
		ExpiresAt: jwt.NewNumericDate(now.Add(9 * time.Minute)),
		Issuer:    fmt.Sprintf("%d", t.appID),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(t.privateKey)
	if err != nil {
		return nil, fmt.Errorf("signing github app jwt: %w", err)
	}

	reqCopy := req.Clone(req.Context())
	reqCopy.Header.Set("Authorization", "Bearer "+signedToken)
	reqCopy.Header.Set("Accept", "application/vnd.github+json")
	return t.base.RoundTrip(reqCopy)
}
