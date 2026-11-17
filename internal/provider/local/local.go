package local

import (
	"context"
	"fmt"
	"io"
	"os"

	"pr-review-go/internal/diff"
	"pr-review-go/internal/domain"
)

// LocalProvider implements GitProvider for local diffs and CLI stdin pipe.
type LocalProvider struct {
	rawDiff string
	out     io.Writer
}

// NewLocalProvider creates a new local provider.
func NewLocalProvider(rawDiff string, out io.Writer) *LocalProvider {
	if out == nil {
		out = os.Stdout
	}
	return &LocalProvider{
		rawDiff: rawDiff,
		out:     out,
	}
}

func (p *LocalProvider) GetPullRequest(ctx context.Context, owner, repo string, number int) (*domain.PullRequest, error) {
	return &domain.PullRequest{
		Number:      number,
		Title:       "Local Pull Request Diff",
		Description: "Review of local working tree or stdin diff",
		RepoOwner:   owner,
		RepoName:    repo,
		HeadBranch:  "HEAD",
		BaseBranch:  "main",
	}, nil
}

func (p *LocalProvider) GetDiff(ctx context.Context, pr *domain.PullRequest) ([]domain.DiffFile, error) {
	return diff.ParseUnifiedDiff(p.rawDiff)
}

func (p *LocalProvider) GetRepoConfigFile(ctx context.Context, pr *domain.PullRequest) ([]byte, error) {
	// Look for .pr-agent.yaml or .pr_agent.toml in current working directory
	for _, name := range []string{".pr-agent.yaml", ".pr-agent.yml", ".pr_agent.toml"} {
		if data, err := os.ReadFile(name); err == nil {
			return data, nil
		}
	}
	return nil, nil
}

func (p *LocalProvider) PublishComment(ctx context.Context, pr *domain.PullRequest, body string) error {
	_, err := fmt.Fprintf(p.out, "\n--- PR Review Summary ---\n%s\n-------------------------\n", body)
	return err
}

func (p *LocalProvider) PublishInlineComments(ctx context.Context, pr *domain.PullRequest, findings []domain.ReviewFinding) error {
	if len(findings) == 0 {
		return nil
	}
	fmt.Fprintf(p.out, "\n--- Inline Findings (%d) ---\n", len(findings))
	for _, f := range findings {
		fmt.Fprintf(p.out, "▶ [%s] %s (lines %d-%d): %s\n  %s\n",
			f.Severity, f.File, f.StartLine, f.EndLine, f.IssueTitle, f.Description)
		if f.HasCodeSuggestion() {
			fmt.Fprintf(p.out, "  Suggested Replacement:\n```\n%s\n```\n", f.SuggestedCode)
		}
	}
	fmt.Fprintf(p.out, "----------------------------\n")
	return nil
}

func (p *LocalProvider) SetCommitStatus(ctx context.Context, pr *domain.PullRequest, verdict domain.ReviewVerdict, summary string) error {
	fmt.Fprintf(p.out, "Commit Status: %s - %s\n", verdict, summary)
	return nil
}

func (p *LocalProvider) SetLabels(ctx context.Context, pr *domain.PullRequest, labels []string) error {
	fmt.Fprintf(p.out, "Applied Labels: %v\n", labels)
	return nil
}
