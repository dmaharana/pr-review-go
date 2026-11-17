package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"pr-review-go/internal/review"
)

var (
	describePRURL string
	describeStdin bool
)

var describeCmd = &cobra.Command{
	Use:   "describe",
	Short: "Generate title, walkthrough, and summary for a pull request",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()
		prov, pr, err := resolveProviderAndPR(ctx, describePRURL, describeStdin)
		if err != nil {
			return err
		}

		engine := review.NewEngine(appConfig, llmClient, prov)
		fmt.Printf("Generating description for PR #%d...\n", pr.Number)

		desc, err := engine.Describe(ctx, pr)
		if err != nil {
			return fmt.Errorf("describe failed: %w", err)
		}

		fmt.Printf("\nGenerated Title: %s\nType: %s\nSummary:\n%s\n",
			desc.Title, desc.Type, desc.Summary)
		return nil
	},
}

func init() {
	describeCmd.Flags().StringVar(&describePRURL, "pr-url", "", "Full URL of the pull request")
	describeCmd.Flags().BoolVar(&describeStdin, "stdin", false, "Read unified git diff directly from stdin")
	rootCmd.AddCommand(describeCmd)
}
