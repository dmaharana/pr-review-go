package config

import (
	"os"
	"testing"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.LLM.Model != "gpt-4o" {
		t.Errorf("got %q, want gpt-4o", cfg.LLM.Model)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("got %d, want 8080", cfg.Server.Port)
	}
	if len(cfg.Ignore.GlobPatterns) == 0 {
		t.Errorf("expected default ignore patterns, got empty")
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	os.Setenv("OPENAI_API_KEY", "sk-test-key-1234")
	os.Setenv("PORT", "9090")
	os.Setenv("GITHUB_TOKEN", "ghp_secret_token")
	os.Setenv("LLM_MODEL", "gpt-4o-mini")
	defer func() {
		os.Unsetenv("OPENAI_API_KEY")
		os.Unsetenv("PORT")
		os.Unsetenv("GITHUB_TOKEN")
		os.Unsetenv("LLM_MODEL")
	}()

	cfg := DefaultConfig()
	cfg.ApplyEnvOverrides()

	if cfg.LLM.APIKey != "sk-test-key-1234" {
		t.Errorf("got APIKey %q, want sk-test-key-1234", cfg.LLM.APIKey)
	}
	if cfg.Server.Port != 9090 {
		t.Errorf("got Port %d, want 9090", cfg.Server.Port)
	}
	if cfg.GitHub.Token != "ghp_secret_token" {
		t.Errorf("got Token %q, want ghp_secret_token", cfg.GitHub.Token)
	}
	if cfg.LLM.Model != "gpt-4o-mini" {
		t.Errorf("got Model %q, want gpt-4o-mini", cfg.LLM.Model)
	}
}

func TestApplyRepoOverrides(t *testing.T) {
	yamlContent := []byte(`
review:
  extra_instructions: "Focus strictly on memory leaks."
  max_findings: 3
  publish_inline: false
ignore:
  glob_patterns:
    - "legacy/**"
`)
	cfg := DefaultConfig()
	if err := cfg.ApplyRepoOverrides(yamlContent); err != nil {
		t.Fatalf("failed to apply repo overrides: %v", err)
	}

	if cfg.Review.ExtraInstructions != "Focus strictly on memory leaks." {
		t.Errorf("got extra instructions %q", cfg.Review.ExtraInstructions)
	}
	if cfg.Review.MaxFindings != 3 {
		t.Errorf("got max findings %d, want 3", cfg.Review.MaxFindings)
	}
	if cfg.Review.PublishInline {
		t.Errorf("expected publish_inline to be false")
	}

	found := false
	for _, p := range cfg.Ignore.GlobPatterns {
		if p == "legacy/**" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected legacy/** in ignore patterns")
	}
}
