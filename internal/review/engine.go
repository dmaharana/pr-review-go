package review

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"
	"pr-review-go/internal/config"
	"pr-review-go/internal/diff"
	"pr-review-go/internal/domain"
	"pr-review-go/internal/llm"
	"pr-review-go/prompts"
	"pr-review-go/internal/provider"
)

// Engine orchestrates the pull request review workflows.
type Engine struct {
	cfg      *config.Config
	llm      llm.Client
	provider provider.GitProvider
}

// NewEngine creates a new review engine.
func NewEngine(cfg *config.Config, llmClient llm.Client, prov provider.GitProvider) *Engine {
	return &Engine{
		cfg:      cfg,
		llm:      llmClient,
		provider: prov,
	}
}

// Review performs a complete review on the given pull request.
func (e *Engine) Review(ctx context.Context, pr *domain.PullRequest) (*domain.ReviewOutput, error) {
	// 1. Check for repo-level configuration overrides (.pr-agent.yaml)
	if repoCfg, err := e.provider.GetRepoConfigFile(ctx, pr); err == nil && len(repoCfg) > 0 {
		_ = e.cfg.ApplyRepoOverrides(repoCfg)
	}

	// 2. Fetch diff files
	diffFiles, err := e.provider.GetDiff(ctx, pr)
	if err != nil {
		return nil, fmt.Errorf("getting pull request diff: %w", err)
	}

	// 3. Filter ignored files
	filteredFiles := diff.FilterFiles(diffFiles, e.cfg.Ignore.GlobPatterns)
	if len(filteredFiles) == 0 {
		out := &domain.ReviewOutput{
			Verdict:        domain.VerdictNoConcernsFound,
			Summary:        "No reviewable files changed (all files filtered or empty).",
			RiskAssessment: "low",
			EffortToReview: 1,
		}
		if e.cfg.Review.PublishOverview {
			_ = e.provider.PublishComment(ctx, pr, FormatReviewMarkdown(out))
		}
		if e.cfg.Review.PublishCommitStatus {
			_ = e.provider.SetCommitStatus(ctx, pr, out.Verdict, out.Summary)
		}
		return out, nil
	}

	// 4. Chunk files according to token budget (e.g., 20,000 tokens per chunk)
	chunks := diff.ChunkFiles(filteredFiles, 20000)

	// 5. Execute review per chunk (concurrently if multiple chunks)
	var mu sync.Mutex
	var collectedOutputs []*domain.ReviewOutput

	g, gCtx := errgroup.WithContext(ctx)
	for _, chunk := range chunks {
		chunkFiles := chunk
		g.Go(func() error {
			chunkDiff := diff.FormatFilesDiff(chunkFiles)

			sysPrompt, err := prompts.RenderReviewSystem(prompts.ReviewSystemData{
				ExtraInstructions: e.cfg.Review.ExtraInstructions,
			})
			if err != nil {
				return fmt.Errorf("rendering system prompt: %w", err)
			}

			userPrompt, err := prompts.RenderReviewUser(prompts.ReviewUserData{
				Title:       pr.Title,
				Description: pr.Description,
				Author:      pr.Author,
				HeadBranch:  pr.HeadBranch,
				BaseBranch:  pr.BaseBranch,
				Diff:        chunkDiff,
			})
			if err != nil {
				return fmt.Errorf("rendering user prompt: %w", err)
			}

			respStr, err := e.llm.Complete(gCtx, sysPrompt, userPrompt)
			if err != nil {
				return fmt.Errorf("calling LLM: %w", err)
			}

			var chunkOut domain.ReviewOutput
			if err := json.Unmarshal([]byte(respStr), &chunkOut); err != nil {
				return fmt.Errorf("decoding review JSON: %w (raw response: %s)", err, respStr)
			}

			mu.Lock()
			collectedOutputs = append(collectedOutputs, &chunkOut)
			mu.Unlock()
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// 6. Aggregate findings and determine final verdict
	finalOutput := e.aggregateReviews(collectedOutputs)

	// 7. Publish outputs according to configuration
	if e.cfg.Review.PublishOverview {
		md := FormatReviewMarkdown(finalOutput)
		if err := e.provider.PublishComment(ctx, pr, md); err != nil {
			return finalOutput, fmt.Errorf("publishing review summary comment: %w", err)
		}
	}

	if e.cfg.Review.PublishInline && len(finalOutput.Findings) > 0 {
		if err := e.provider.PublishInlineComments(ctx, pr, finalOutput.Findings); err != nil {
			// Don't fail the whole review if inline comments partially fail
			fmt.Printf("Warning: failed to publish some inline comments: %v\n", err)
		}
	}

	if e.cfg.Review.PublishCommitStatus {
		if err := e.provider.SetCommitStatus(ctx, pr, finalOutput.Verdict, finalOutput.Summary); err != nil {
			fmt.Printf("Warning: failed to set commit status: %v\n", err)
		}
	}

	return finalOutput, nil
}

// Describe generates an overview description for the pull request.
func (e *Engine) Describe(ctx context.Context, pr *domain.PullRequest) (*domain.DescribeOutput, error) {
	diffFiles, err := e.provider.GetDiff(ctx, pr)
	if err != nil {
		return nil, fmt.Errorf("getting pull request diff: %w", err)
	}

	filteredFiles := diff.FilterFiles(diffFiles, e.cfg.Ignore.GlobPatterns)
	formattedDiff := diff.FormatFilesDiff(filteredFiles)

	sysPrompt, err := prompts.RenderDescribeSystem()
	if err != nil {
		return nil, err
	}

	userPrompt, err := prompts.RenderDescribeUser(prompts.DescribeUserData{
		Title:       pr.Title,
		Description: pr.Description,
		HeadBranch:  pr.HeadBranch,
		BaseBranch:  pr.BaseBranch,
		Diff:        formattedDiff,
	})
	if err != nil {
		return nil, err
	}

	respStr, err := e.llm.Complete(ctx, sysPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("calling LLM: %w", err)
	}

	var descOutput domain.DescribeOutput
	if err := json.Unmarshal([]byte(respStr), &descOutput); err != nil {
		return nil, fmt.Errorf("decoding describe JSON: %w (raw response: %s)", err, respStr)
	}

	if e.cfg.Review.PublishOverview {
		md := FormatDescribeMarkdown(&descOutput)
		_ = e.provider.PublishComment(ctx, pr, md)
	}

	return &descOutput, nil
}

// Ask answers a user question regarding the pull request.
func (e *Engine) Ask(ctx context.Context, pr *domain.PullRequest, question string) (*domain.AskResponse, error) {
	diffFiles, err := e.provider.GetDiff(ctx, pr)
	if err != nil {
		return nil, fmt.Errorf("getting pull request diff: %w", err)
	}

	filteredFiles := diff.FilterFiles(diffFiles, e.cfg.Ignore.GlobPatterns)
	formattedDiff := diff.FormatFilesDiff(filteredFiles)

	sysPrompt, err := prompts.RenderAskSystem()
	if err != nil {
		return nil, err
	}

	userPrompt, err := prompts.RenderAskUser(prompts.AskUserData{
		Question:   question,
		Title:      pr.Title,
		HeadBranch: pr.HeadBranch,
		BaseBranch: pr.BaseBranch,
		Diff:       formattedDiff,
	})
	if err != nil {
		return nil, err
	}

	respStr, err := e.llm.Complete(ctx, sysPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("calling LLM: %w", err)
	}

	var askResp domain.AskResponse
	if err := json.Unmarshal([]byte(respStr), &askResp); err != nil {
		return nil, fmt.Errorf("decoding ask JSON: %w (raw response: %s)", err, respStr)
	}

	if e.cfg.Review.PublishOverview {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("### Answer to: *\"%s\"*\n\n%s\n\n", question, askResp.Answer))
		if len(askResp.References) > 0 {
			sb.WriteString(fmt.Sprintf("**References:** `%s`\n\n", strings.Join(askResp.References, "`, `")))
		}
		sb.WriteString("---\n*Generated by `pr-review-go`*")
		_ = e.provider.PublishComment(ctx, pr, sb.String())
	}

	return &askResp, nil
}

// GenerateLabels suggests and applies labels based on changes in the PR.
func (e *Engine) GenerateLabels(ctx context.Context, pr *domain.PullRequest) ([]string, error) {
	diffFiles, err := e.provider.GetDiff(ctx, pr)
	if err != nil {
		return nil, fmt.Errorf("getting pull request diff: %w", err)
	}

	var labels []string
	labelMap := make(map[string]bool)

	for _, f := range diffFiles {
		path := strings.ToLower(f.DisplayPath())
		if strings.HasSuffix(path, "_test.go") || strings.Contains(path, "test/") {
			labelMap["tests"] = true
		}
		if strings.HasSuffix(path, ".md") || strings.HasPrefix(path, "docs/") {
			labelMap["documentation"] = true
		}
		if strings.Contains(path, "docker") || strings.HasPrefix(path, ".github/") {
			labelMap["ci/cd"] = true
		}
		if strings.HasSuffix(path, ".sql") || strings.Contains(path, "migration") {
			labelMap["database"] = true
		}
		if strings.HasSuffix(path, ".go") {
			labelMap["backend"] = true
		}
		if strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".js") {
			labelMap["frontend"] = true
		}
	}

	for l := range labelMap {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	if len(labels) > 0 {
		_ = e.provider.SetLabels(ctx, pr, labels)
	}
	return labels, nil
}

func (e *Engine) aggregateReviews(outputs []*domain.ReviewOutput) *domain.ReviewOutput {
	if len(outputs) == 0 {
		return &domain.ReviewOutput{
			Verdict:        domain.VerdictNoConcernsFound,
			Summary:        "No findings produced.",
			RiskAssessment: "low",
			EffortToReview: 1,
		}
	}

	final := &domain.ReviewOutput{
		Verdict:        domain.VerdictNoConcernsFound,
		RiskAssessment: "low",
		EffortToReview: 1,
	}

	var summaries []string
	var allFindings []domain.ReviewFinding
	prioritySet := make(map[string]bool)

	for _, o := range outputs {
		if o.Summary != "" {
			summaries = append(summaries, o.Summary)
		}
		allFindings = append(allFindings, o.Findings...)
		for _, pf := range o.PriorityFiles {
			prioritySet[pf] = true
		}

		// Worst verdict wins
		if o.Verdict == domain.VerdictChangesRequired || final.Verdict == domain.VerdictChangesRequired {
			final.Verdict = domain.VerdictChangesRequired
		} else if o.Verdict == domain.VerdictNeedsReview || final.Verdict == domain.VerdictNeedsReview {
			final.Verdict = domain.VerdictNeedsReview
		}

		// Worst risk wins
		if o.RiskAssessment == "high" || final.RiskAssessment == "high" {
			final.RiskAssessment = "high"
		} else if o.RiskAssessment == "medium" || final.RiskAssessment == "medium" {
			final.RiskAssessment = "medium"
		}

		if o.EffortToReview > final.EffortToReview {
			final.EffortToReview = o.EffortToReview
		}
	}

	final.Summary = strings.Join(summaries, " ")
	if len(allFindings) > e.cfg.Review.MaxFindings {
		// Filter by confidence / severity
		allFindings = allFindings[:e.cfg.Review.MaxFindings]
	}
	final.Findings = allFindings

	for p := range prioritySet {
		final.PriorityFiles = append(final.PriorityFiles, p)
	}
	sort.Strings(final.PriorityFiles)

	return final
}
