package main

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"pr-review-go/internal/review"
)

var (
	askPRURL string
	askStdin bool
)

var askCmd = &cobra.Command{
	Use:   "ask [question]",
	Short: "Ask a question about the changes in a pull request",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		question := strings.Join(args, " ")
		ctx := context.Background()
		prov, pr, err := resolveProviderAndPR(ctx, askPRURL, askStdin)
		if err != nil {
			return err
		}

		engine := review.NewEngine(appConfig, llmClient, prov)
		fmt.Printf("Analyzing PR #%d to answer question...\n", pr.Number)

		resp, err := engine.Ask(ctx, pr, question)
		if err != nil {
			return fmt.Errorf("ask failed: %w", err)
		}

		fmt.Printf("\nAnswer:\n%s\n", resp.Answer)
		return nil
	},
}

func init() {
	askCmd.Flags().StringVar(&askPRURL, "pr-url", "", "Full URL of the pull request")
	askCmd.Flags().BoolVar(&askStdin, "stdin", false, "Read unified git diff directly from stdin")
	rootCmd.AddCommand(askCmd)
}
