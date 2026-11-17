package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pr-review-go/internal/config"
)

func TestOpenAIClientComplete(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if len(req.Messages) != 2 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		resp := chatResponse{
			Choices: []chatChoice{
				{
					Message: struct {
						Content string `json:"content"`
					}{
						Content: "```json\n{\"verdict\": \"no_concerns_found\"}\n```",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	cfg := config.LLMConfig{
		Model:      "mock-model",
		APIKey:     "test-key",
		BaseURL:    mockServer.URL,
		TimeoutSec: 5,
	}

	client := NewOpenAIClient(cfg)
	res, err := client.Complete(context.Background(), "system prompt", "user prompt")
	if err != nil {
		t.Fatalf("Complete failed: %v", err)
	}

	expected := `{"verdict": "no_concerns_found"}`
	if res != expected {
		t.Errorf("got %q, want %q", res, expected)
	}
}

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"{\"key\": \"val\"}", "{\"key\": \"val\"}"},
		{"```json\n{\"key\": \"val\"}\n```", "{\"key\": \"val\"}"},
		{"```\n{\"key\": \"val\"}\n```", "{\"key\": \"val\"}"},
	}

	for _, tc := range tests {
		got := CleanJSONResponse(tc.input)
		if got != tc.expected {
			t.Errorf("input %q: got %q, want %q", tc.input, got, tc.expected)
		}
	}
}
