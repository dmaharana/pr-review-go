package provider

import (
	"context"

	"pr-review-go/internal/domain"
)

// GitProvider abstracts VCS operations across GitHub, Bitbucket, and Local Git.
type GitProvider interface {
	// GetPullRequest fetches metadata about a pull request.
	GetPullRequest(ctx context.Context, owner, repo string, number int) (*domain.PullRequest, error)

	// GetDiff fetches and parses the modified files in the pull request.
	GetDiff(ctx context.Context, pr *domain.PullRequest) ([]domain.DiffFile, error)

	// GetRepoConfigFile fetches an optional .pr-agent.yaml file from the repo.
	GetRepoConfigFile(ctx context.Context, pr *domain.PullRequest) ([]byte, error)

	// PublishComment posts a top-level summary comment on the pull request.
	PublishComment(ctx context.Context, pr *domain.PullRequest, body string) error

	// PublishInlineComments posts review comments directly on affected lines of code.
	PublishInlineComments(ctx context.Context, pr *domain.PullRequest, findings []domain.ReviewFinding) error

	// SetCommitStatus publishes an automated check run / commit build status.
	SetCommitStatus(ctx context.Context, pr *domain.PullRequest, verdict domain.ReviewVerdict, summary string) error

	// SetLabels applies labels to the pull request.
	SetLabels(ctx context.Context, pr *domain.PullRequest, labels []string) error
}
