package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"pr-review-go/internal/review"
)

var (
	reviewPRURL string
	reviewStdin bool
)

var reviewCmd = &cobra.Command{
	Use:   "review",
	Short: "Perform an automated code review on a pull request",
	Long: `review analyzes code diffs, detects bugs, race conditions, security vulnerabilities,
and provides structured recommendations and inline suggestions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		prov, pr, err := resolveProviderAndPR(ctx, reviewPRURL, reviewStdin)
		if err != nil {
			return err
		}

		engine := review.NewEngine(appConfig, llmClient, prov)
		fmt.Printf("Starting code review for PR #%d (%s)...\n", pr.Number, pr.Title)

		out, err := engine.Review(ctx, pr)
		if err != nil {
			return fmt.Errorf("review failed: %w", err)
		}

		fmt.Printf("\nReview complete! Verdict: %s | Risk: %s | Findings: %d\n",
			out.Verdict, out.RiskAssessment, len(out.Findings))
		return nil
	},
}

func init() {
	reviewCmd.Flags().StringVar(&reviewPRURL, "pr-url", "", "Full URL of the pull request to review")
	reviewCmd.Flags().BoolVar(&reviewStdin, "stdin", false, "Read unified git diff directly from stdin")
	rootCmd.AddCommand(reviewCmd)
}
