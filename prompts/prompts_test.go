package prompts

import (
	"strings"
	"testing"
)

func TestRenderReviewPrompts(t *testing.T) {
	sys, err := RenderReviewSystem(ReviewSystemData{
		ExtraInstructions: "Enforce error wrapping with %w.",
	})
	if err != nil {
		t.Fatalf("RenderReviewSystem failed: %v", err)
	}
	if !strings.Contains(sys, "Enforce error wrapping with %w.") {
		t.Errorf("system prompt missing extra instructions")
	}
	if !strings.Contains(sys, "no_concerns_found") {
		t.Errorf("system prompt missing schema keywords")
	}

	user, err := RenderReviewUser(ReviewUserData{
		Title:       "Add webhook listener",
		Description: "Handles incoming events",
		Author:      "octocat",
		HeadBranch:  "feature",
		BaseBranch:  "main",
		Diff:        "+ func Handle() {}",
	})
	if err != nil {
		t.Fatalf("RenderReviewUser failed: %v", err)
	}
	if !strings.Contains(user, "Add webhook listener") {
		t.Errorf("user prompt missing title")
	}
	if !strings.Contains(user, "+ func Handle() {}") {
		t.Errorf("user prompt missing diff")
	}
}

func TestRenderDescribePrompts(t *testing.T) {
	sys, err := RenderDescribeSystem()
	if err != nil {
		t.Fatalf("RenderDescribeSystem failed: %v", err)
	}
	if !strings.Contains(sys, "PR-Describer") {
		t.Errorf("unexpected describe system prompt: %s", sys)
	}

	user, err := RenderDescribeUser(DescribeUserData{
		Title: "Refactor database pool",
		Diff:  "some diff",
	})
	if err != nil {
		t.Fatalf("RenderDescribeUser failed: %v", err)
	}
	if !strings.Contains(user, "Refactor database pool") {
		t.Errorf("describe user prompt missing title")
	}
}

func TestRenderAskPrompts(t *testing.T) {
	sys, err := RenderAskSystem()
	if err != nil {
		t.Fatalf("RenderAskSystem failed: %v", err)
	}
	if !strings.Contains(sys, "PR-Assistant") {
		t.Errorf("unexpected ask system prompt: %s", sys)
	}

	user, err := RenderAskUser(AskUserData{
		Question: "Does this change introduce any memory leaks?",
		Title:    "PR 1",
		Diff:     "diff content",
	})
	if err != nil {
		t.Fatalf("RenderAskUser failed: %v", err)
	}
	if !strings.Contains(user, "Does this change introduce any memory leaks?") {
		t.Errorf("ask user prompt missing question")
	}
}
