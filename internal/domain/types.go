package domain

import (
	"strings"
	"time"
)

// ReviewVerdict represents the overall merge recommendation for a pull request.
type ReviewVerdict string

const (
	VerdictNoConcernsFound ReviewVerdict = "no_concerns_found"
	VerdictNeedsReview     ReviewVerdict = "needs_review"
	VerdictChangesRequired ReviewVerdict = "changes_required"
)

// Severity represents finding severity level.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// PullRequest holds basic information about a pull request.
type PullRequest struct {
	ID          int64             `json:"id"`
	Number      int               `json:"number"`
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Author      string            `json:"author"`
	HeadBranch  string            `json:"head_branch"`
	HeadSHA     string            `json:"head_sha"`
	BaseBranch  string            `json:"base_branch"`
	BaseSHA     string            `json:"base_sha"`
	RepoOwner   string            `json:"repo_owner"`
	RepoName    string            `json:"repo_name"`
	URL         string            `json:"url"`
	Labels      []string          `json:"labels"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// DiffFile represents a single file modified within a pull request diff.
type DiffFile struct {
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	Patch     string `json:"patch"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	IsNew     bool   `json:"is_new"`
	IsDeleted bool   `json:"is_deleted"`
	IsBinary  bool   `json:"is_binary"`
}

// DisplayPath returns the active file path (NewPath or OldPath if deleted).
func (f DiffFile) DisplayPath() string {
	if f.NewPath != "" && f.NewPath != "/dev/null" {
		return f.NewPath
	}
	return f.OldPath
}

// ReviewFinding represents a discrete, actionable issue detected in the code diff.
type ReviewFinding struct {
	File          string   `json:"file"`
	StartLine     int      `json:"start_line"`
	EndLine       int      `json:"end_line"`
	IssueTitle    string   `json:"issue_title"`
	Description   string   `json:"description"`
	Severity      Severity `json:"severity"`
	ExistingCode  string   `json:"existing_code,omitempty"`
	SuggestedCode string   `json:"suggested_code,omitempty"`
	Confidence    int      `json:"confidence,omitempty"` // 1-10
}

// HasCodeSuggestion checks if this finding has a concrete replacement suggestion.
func (f ReviewFinding) HasCodeSuggestion() bool {
	return strings.TrimSpace(f.SuggestedCode) != "" && strings.TrimSpace(f.ExistingCode) != ""
}

// ReviewOutput is the structured output produced by a code review.
type ReviewOutput struct {
	Verdict        ReviewVerdict   `json:"verdict"`
	Summary        string          `json:"summary"`
	RiskAssessment string          `json:"risk_assessment"` // "low", "medium", "high"
	EffortToReview int             `json:"effort_to_review"` // 1-5 scale
	Findings       []ReviewFinding `json:"findings"`
	PriorityFiles  []string        `json:"priority_files,omitempty"`
}

// DescribeOutput is the structured output produced by the PR description generator.
type DescribeOutput struct {
	Title         string   `json:"title"`
	Type          string   `json:"type"` // e.g. "feat", "fix", "refactor", "docs"
	Summary       string   `json:"summary"`
	Walkthrough   string   `json:"walkthrough"`
	KeyChanges    []string `json:"key_changes"`
	SuggestedTags []string `json:"suggested_tags,omitempty"`
}

// AskResponse is the response to a user query regarding a pull request.
type AskResponse struct {
	Answer     string   `json:"answer"`
	References []string `json:"references,omitempty"`
}
