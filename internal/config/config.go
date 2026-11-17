package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds all configuration options for pr-review-go.
type Config struct {
	LLM             LLMConfig             `yaml:"llm"`
	GitHub          GitHubConfig          `yaml:"github"`
	BitbucketCloud  BitbucketCloudConfig  `yaml:"bitbucket_cloud"`
	BitbucketServer BitbucketServerConfig `yaml:"bitbucket_server"`
	Review          ReviewConfig          `yaml:"review"`
	Server          ServerConfig          `yaml:"server"`
	Ignore          IgnoreConfig          `yaml:"ignore"`
}

// LLMConfig controls the language model connection.
type LLMConfig struct {
	Provider    string  `yaml:"provider"`    // "openai", "azure", "anthropic", "gemini", "ollama"
	Model       string  `yaml:"model"`       // e.g. "gpt-4o", "claude-3-5-sonnet-20241022"
	APIKey      string  `yaml:"api_key"`     // API Key or Token
	BaseURL     string  `yaml:"base_url"`    // Custom endpoint (e.g. for Ollama, vLLM, Azure, Groq)
	MaxTokens   int     `yaml:"max_tokens"`  // Max response tokens
	Temperature float64 `yaml:"temperature"` // Sampling temperature
	TimeoutSec  int     `yaml:"timeout_sec"` // Request timeout
}

// GitHubConfig holds GitHub SaaS and Enterprise credentials.
type GitHubConfig struct {
	BaseURL          string `yaml:"base_url"`           // Blank for SaaS, or e.g. "https://github.enterprise.corp/api/v3"
	Token            string `yaml:"token"`              // Personal Access Token (PAT)
	AppID            int64  `yaml:"app_id"`             // GitHub App ID
	AppPrivateKey    string `yaml:"app_private_key"`    // RSA Private Key PEM text
	AppPrivateKeyPath string `yaml:"app_private_key_path"` // Path to RSA Private Key file
	WebhookSecret    string `yaml:"webhook_secret"`     // Webhook HMAC-SHA256 secret
}

// BitbucketCloudConfig holds credentials for Bitbucket Cloud.
type BitbucketCloudConfig struct {
	Username      string `yaml:"username"`
	AppPassword   string `yaml:"app_password"`
	WebhookSecret string `yaml:"webhook_secret"`
}

// BitbucketServerConfig holds credentials for Bitbucket Data Center / Server.
type BitbucketServerConfig struct {
	BaseURL       string `yaml:"base_url"`       // e.g. "https://bitbucket.myorg.internal"
	Token         string `yaml:"token"`          // Personal Access Token (PAT)
	WebhookSecret string `yaml:"webhook_secret"` // Webhook secret
}

// ReviewConfig controls the review depth and prompt rules.
type ReviewConfig struct {
	MaxFindings          int    `yaml:"max_findings"`
	MinFindingConfidence int    `yaml:"min_finding_confidence"` // 1-10
	RequireEffort        bool   `yaml:"require_effort"`
	RequireRisk          bool   `yaml:"require_risk"`
	ExtraInstructions    string `yaml:"extra_instructions"`
	PublishInline        bool   `yaml:"publish_inline"`
	PublishOverview      bool   `yaml:"publish_overview"`
	PublishCommitStatus  bool   `yaml:"publish_commit_status"`
}

// ServerConfig controls the HTTP webhook daemon.
type ServerConfig struct {
	Host                   string `yaml:"host"`
	Port                   int    `yaml:"port"`
	Workers                int    `yaml:"workers"`
	AutoReviewOnOpen       bool   `yaml:"auto_review_on_open"`
	AutoReviewOnModify     bool   `yaml:"auto_review_on_modify"`
	CommentTriggersEnabled bool   `yaml:"comment_triggers_enabled"`
}

// IgnoreConfig controls which files are skipped from review.
type IgnoreConfig struct {
	GlobPatterns []string `yaml:"glob_patterns"`
}

// DefaultConfig returns safe, production-grade defaults.
func DefaultConfig() *Config {
	return &Config{
		LLM: LLMConfig{
			Provider:    "openai",
			Model:       "gpt-4o",
			MaxTokens:   4096,
			Temperature: 0.2,
			TimeoutSec:  60,
		},
		Review: ReviewConfig{
			MaxFindings:          8,
			MinFindingConfidence: 6,
			RequireEffort:        true,
			RequireRisk:          true,
			PublishInline:        true,
			PublishOverview:      true,
			PublishCommitStatus:  true,
		},
		Server: ServerConfig{
			Host:                   "0.0.0.0",
			Port:                   8080,
			Workers:                4,
			AutoReviewOnOpen:       true,
			AutoReviewOnModify:     true,
			CommentTriggersEnabled: true,
		},
		Ignore: IgnoreConfig{
			GlobPatterns: []string{
				"*.lock",
				"package-lock.json",
				"yarn.lock",
				"pnpm-lock.yaml",
				"go.sum",
				"Cargo.lock",
				"composer.lock",
				"*.min.js",
				"*.min.css",
				"*.map",
				"vendor/**",
				"node_modules/**",
				"*.pb.go",
				"*_generated.go",
				"*.svg",
				"*.png",
				"*.jpg",
			},
		},
	}
}

// LoadConfig loads the default config, layers an optional file, and overlays environment variables.
func LoadConfig(filePath string) (*Config, error) {
	cfg := DefaultConfig()

	if filePath != "" {
		data, err := os.ReadFile(filePath)
		if err != nil {
			return nil, fmt.Errorf("reading config file %q: %w", filePath, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file %q: %w", filePath, err)
		}
	}

	cfg.ApplyEnvOverrides()
	return cfg, nil
}

// ApplyRepoOverrides overlays settings from a repository's `.pr-agent.yaml` content.
func (c *Config) ApplyRepoOverrides(yamlContent []byte) error {
	if len(yamlContent) == 0 {
		return nil
	}
	type repoOverride struct {
		Review *struct {
			ExtraInstructions *string `yaml:"extra_instructions"`
			MaxFindings       *int    `yaml:"max_findings"`
			PublishInline     *bool   `yaml:"publish_inline"`
		} `yaml:"review"`
		Ignore *struct {
			GlobPatterns []string `yaml:"glob_patterns"`
		} `yaml:"ignore"`
	}

	var ro repoOverride
	if err := yaml.Unmarshal(yamlContent, &ro); err != nil {
		return fmt.Errorf("parsing repo config override: %w", err)
	}

	if ro.Review != nil {
		if ro.Review.ExtraInstructions != nil {
			c.Review.ExtraInstructions = *ro.Review.ExtraInstructions
		}
		if ro.Review.MaxFindings != nil {
			c.Review.MaxFindings = *ro.Review.MaxFindings
		}
		if ro.Review.PublishInline != nil {
			c.Review.PublishInline = *ro.Review.PublishInline
		}
	}
	if ro.Ignore != nil && len(ro.Ignore.GlobPatterns) > 0 {
		c.Ignore.GlobPatterns = append(c.Ignore.GlobPatterns, ro.Ignore.GlobPatterns...)
	}
	return nil
}

// ApplyEnvOverrides overrides configuration with standard environment variables.
func (c *Config) ApplyEnvOverrides() {
	if v := os.Getenv("LLM_PROVIDER"); v != "" {
		c.LLM.Provider = strings.ToLower(v)
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		c.LLM.Model = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" {
		c.LLM.APIKey = v
	} else if v := os.Getenv("LLM_API_KEY"); v != "" {
		c.LLM.APIKey = v
	}
	if v := os.Getenv("OPENAI_BASE_URL"); v != "" {
		c.LLM.BaseURL = v
	} else if v := os.Getenv("LLM_BASE_URL"); v != "" {
		c.LLM.BaseURL = v
	}

	// GitHub
	if v := os.Getenv("GITHUB_TOKEN"); v != "" {
		c.GitHub.Token = v
	}
	if v := os.Getenv("GITHUB_BASE_URL"); v != "" {
		c.GitHub.BaseURL = v
	}
	if v := os.Getenv("GITHUB_APP_ID"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.GitHub.AppID = id
		}
	}
	if v := os.Getenv("GITHUB_APP_PRIVATE_KEY"); v != "" {
		c.GitHub.AppPrivateKey = v
	}
	if v := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH"); v != "" {
		c.GitHub.AppPrivateKeyPath = v
	}
	if v := os.Getenv("GITHUB_WEBHOOK_SECRET"); v != "" {
		c.GitHub.WebhookSecret = v
	}

	// Bitbucket Cloud
	if v := os.Getenv("BITBUCKET_USERNAME"); v != "" {
		c.BitbucketCloud.Username = v
	}
	if v := os.Getenv("BITBUCKET_APP_PASSWORD"); v != "" {
		c.BitbucketCloud.AppPassword = v
	}
	if v := os.Getenv("BITBUCKET_CLOUD_WEBHOOK_SECRET"); v != "" {
		c.BitbucketCloud.WebhookSecret = v
	}

	// Bitbucket Server / Data Center
	if v := os.Getenv("BITBUCKET_SERVER_BASE_URL"); v != "" {
		c.BitbucketServer.BaseURL = v
	}
	if v := os.Getenv("BITBUCKET_SERVER_TOKEN"); v != "" {
		c.BitbucketServer.Token = v
	}
	if v := os.Getenv("BITBUCKET_SERVER_WEBHOOK_SECRET"); v != "" {
		c.BitbucketServer.WebhookSecret = v
	}

	// Server
	if v := os.Getenv("PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			c.Server.Port = p
		}
	}
}
