package llm

import (
	"fmt"
	"os"
	"strings"
)

// ProviderConfig overrides defaults for the factory.
type ProviderConfig struct {
	OllamaURL        string
	AnthropicAPIKey  string // defaults to ANTHROPIC_API_KEY
	AnthropicBaseURL string // defaults to https://api.anthropic.com
	OpenAIAPIKey     string // defaults to OPENAI_API_KEY
	OpenAIBaseURL    string // defaults to https://api.openai.com
}

// Pick returns an Adapter based on the model name prefix:
//
//	mock-*    → built-in scripted adapter (no network, no API key)
//	claude-*  → Anthropic    (requires ANTHROPIC_API_KEY)
//	gpt-*,o*  → OpenAI       (requires OPENAI_API_KEY)
//	anything else → Ollama   (local)
//
// Returns an error if the matched provider's API key is missing.
func Pick(model string, cfg ProviderConfig) (Adapter, error) {
	switch {
	case isMockModel(model):
		return NewMock(model)
	case strings.HasPrefix(model, "claude-"):
		key := cfg.AnthropicAPIKey
		if key == "" {
			key = os.Getenv("ANTHROPIC_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("model %q needs ANTHROPIC_API_KEY", model)
		}
		a := NewAnthropic(key)
		base := cfg.AnthropicBaseURL
		if base == "" {
			base = os.Getenv("ANTHROPIC_BASE_URL")
		}
		if base != "" {
			a.BaseURL = base
		}
		return a, nil
	case strings.HasPrefix(model, "gpt-") || strings.HasPrefix(model, "o1-") || strings.HasPrefix(model, "o3") || strings.HasPrefix(model, "o4"):
		key := cfg.OpenAIAPIKey
		if key == "" {
			key = os.Getenv("OPENAI_API_KEY")
		}
		if key == "" {
			return nil, fmt.Errorf("model %q needs OPENAI_API_KEY", model)
		}
		o := NewOpenAI(key)
		base := cfg.OpenAIBaseURL
		if base == "" {
			base = os.Getenv("OPENAI_BASE_URL")
		}
		if base != "" {
			o.BaseURL = base
		}
		return o, nil
	default:
		return NewOllama(cfg.OllamaURL), nil
	}
}
