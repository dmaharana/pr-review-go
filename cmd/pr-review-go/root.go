package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"pr-review-go/internal/config"
	"pr-review-go/internal/domain"
	"pr-review-go/internal/llm"
	"pr-review-go/internal/provider"
	"pr-review-go/internal/provider/bitbucketcloud"
	"pr-review-go/internal/provider/bitbucketserver"
	"pr-review-go/internal/provider/github"
	"pr-review-go/internal/provider/local"
)

var (
	configFile string
	modelFlag  string
	appConfig  *config.Config
	llmClient  llm.Client
)

var rootCmd = &cobra.Command{
	Use:   "pr-review-go",
	Short: "pr-review-go: Automated AI Pull Request Reviewer",
	Long: `pr-review-go is a high-performance single-binary automated pull request review tool.
It supports GitHub SaaS, GitHub Enterprise Server, Bitbucket Cloud, Bitbucket Data Center, and local git diffs.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.LoadConfig(configFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		if modelFlag != "" {
			cfg.LLM.Model = modelFlag
		}
		appConfig = cfg
		llmClient = llm.NewOpenAIClient(cfg.LLM)
		return nil
	},
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&configFile, "config", "c", "", "Path to configuration file")
	rootCmd.PersistentFlags().StringVarP(&modelFlag, "model", "m", "", "Override LLM model name (e.g. gpt-4o, claude-3-5-sonnet, ollama/deepseek-r1)")
}

func resolveProviderAndPR(ctx context.Context, prURL string, stdinDiff bool) (provider.GitProvider, *domain.PullRequest, error) {
	if stdinDiff {
		diffBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, nil, fmt.Errorf("reading diff from stdin: %w", err)
		}
		prov := local.NewLocalProvider(string(diffBytes), os.Stdout)
		pr, err := prov.GetPullRequest(ctx, "local", "local", 1)
		return prov, pr, err
	}

	if prURL == "" {
		return nil, nil, fmt.Errorf("either --pr-url or --stdin must be provided")
	}

	parsed, err := parsePRURL(prURL)
	if err != nil {
		return nil, nil, err
	}

	switch parsed.Provider {
	case "github":
		ghCfg := appConfig.GitHub
		if parsed.BaseURL != "" {
			ghCfg.BaseURL = parsed.BaseURL
		}
		prov, err := github.NewProvider(ctx, ghCfg)
		if err != nil {
			return nil, nil, err
		}
		pr, err := prov.GetPullRequest(ctx, parsed.Owner, parsed.Repo, parsed.Number)
		return prov, pr, err

	case "bitbucket_cloud":
		prov := bitbucketcloud.NewProvider(appConfig.BitbucketCloud)
		pr, err := prov.GetPullRequest(ctx, parsed.Owner, parsed.Repo, parsed.Number)
		return prov, pr, err

	case "bitbucket_server":
		bbCfg := appConfig.BitbucketServer
		if parsed.BaseURL != "" {
			bbCfg.BaseURL = parsed.BaseURL
		}
		prov := bitbucketserver.NewProvider(bbCfg)
		pr, err := prov.GetPullRequest(ctx, parsed.Owner, parsed.Repo, parsed.Number)
		return prov, pr, err

	default:
		return nil, nil, fmt.Errorf("unsupported provider: %s", parsed.Provider)
	}
}
