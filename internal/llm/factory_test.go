package llm

import (
	"strings"
	"testing"
)

func TestPick_RoutesByPrefix(t *testing.T) {
	// Local: always works, no env needed.
	a, err := Pick("qwen2.5-coder:7b", ProviderConfig{})
	if err != nil {
		t.Fatal(err)
	}
	if a.Name() != "ollama" {
		t.Errorf("got %s want ollama", a.Name())
	}

	// Claude: needs key.
	if _, err := Pick("claude-sonnet-4-6", ProviderConfig{}); err == nil {
		t.Error("expected error without ANTHROPIC_API_KEY")
	} else if !strings.Contains(err.Error(), "ANTHROPIC_API_KEY") {
		t.Errorf("error should name env var: %v", err)
	}
	a, err = Pick("claude-sonnet-4-6", ProviderConfig{AnthropicAPIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Name() != "anthropic" {
		t.Errorf("got %s want anthropic", a.Name())
	}

	// OpenAI: gpt-* / o-*.
	if _, err := Pick("gpt-4.1", ProviderConfig{}); err == nil {
		t.Error("expected error without OPENAI_API_KEY")
	}
	a, err = Pick("gpt-4.1", ProviderConfig{OpenAIAPIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if a.Name() != "openai" {
		t.Errorf("got %s want openai", a.Name())
	}
}
