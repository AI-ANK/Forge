package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/AI-ANK/Forge/internal/llm"
	"github.com/AI-ANK/Forge/internal/rng"
	"github.com/AI-ANK/Forge/internal/session"
	"github.com/AI-ANK/Forge/internal/tools"
)

// Action is the parsed decision from a single LLM turn.
type Action struct {
	Thought string          `json:"thought"`
	Tool    string          `json:"tool,omitempty"`
	Args    json.RawMessage `json:"args,omitempty"`
	Final   string          `json:"final,omitempty"`
}

type Config struct {
	Model     string
	Goal      string
	Cwd       string
	MaxTurns  int
	Recorder  *session.Recorder
	LLM       llm.Adapter
	Tools     *tools.Registry
	Rng       *rng.Source
	OnTurn    func(turn int, a Action)      // UI hook, nil-safe
	OnResult  func(turn int, result string) // UI hook, nil-safe
}

// Run executes the ReAct loop until the agent emits a final or MaxTurns is hit.
func Run(ctx context.Context, cfg Config) (string, error) {
	if cfg.MaxTurns <= 0 {
		cfg.MaxTurns = 25
	}
	system := SystemPrompt(cfg.Tools, cfg.Goal, cfg.Cwd)
	messages := []llm.Message{
		{Role: "system", Content: system},
		{Role: "user", Content: cfg.Goal},
	}
	// Record the initial user message so replay starts from the same state.
	if err := cfg.Recorder.Record(session.KindUserMsg, map[string]string{"goal": cfg.Goal}); err != nil {
		return "", err
	}

	for turn := 1; turn <= cfg.MaxTurns; turn++ {
		req := llm.Request{
			Model:       cfg.Model,
			Messages:    messages,
			Seed:        cfg.Rng.StepSeed(turn),
			Temperature: 0,
			TopP:        1,
			JSON:        true,
		}
		if err := cfg.Recorder.Record(session.KindLLMRequest, req); err != nil {
			return "", err
		}
		resp, err := cfg.LLM.Chat(ctx, req)
		if err != nil {
			return "", fmt.Errorf("turn %d llm: %w", turn, err)
		}
		if err := cfg.Recorder.Record(session.KindLLMResponse, resp); err != nil {
			return "", err
		}

		action, err := parseAction(resp.Content)
		if err != nil {
			// Give the model one chance to self-correct on malformed output.
			messages = append(messages,
				llm.Message{Role: "assistant", Content: resp.Content},
				llm.Message{Role: "user", Content: fmt.Sprintf("Your last response was not valid JSON in the required format: %v. Respond again with a single JSON object only.", err)},
			)
			continue
		}
		if cfg.OnTurn != nil {
			cfg.OnTurn(turn, action)
		}

		if action.Final != "" {
			if err := cfg.Recorder.Record(session.KindAgentFinal, action); err != nil {
				return "", err
			}
			return action.Final, nil
		}
		if action.Tool == "" {
			messages = append(messages,
				llm.Message{Role: "assistant", Content: resp.Content},
				llm.Message{Role: "user", Content: "You must either call a tool or emit a final. Try again."},
			)
			continue
		}

		call := map[string]any{"tool": action.Tool, "args": json.RawMessage(action.Args)}
		if err := cfg.Recorder.Record(session.KindToolCall, call); err != nil {
			return "", err
		}
		result, runErr := cfg.Tools.Dispatch(ctx, action.Tool, action.Args)
		if runErr != nil {
			result = "ERROR: " + runErr.Error()
		}
		if err := cfg.Recorder.Record(session.KindToolResult, map[string]string{
			"tool":   action.Tool,
			"result": result,
		}); err != nil {
			return "", err
		}
		if cfg.OnResult != nil {
			cfg.OnResult(turn, result)
		}

		messages = append(messages,
			llm.Message{Role: "assistant", Content: resp.Content},
			llm.Message{Role: "tool", Name: action.Tool, Content: truncate(result, 6000)},
		)
	}
	return "", fmt.Errorf("max turns (%d) exceeded without final", cfg.MaxTurns)
}

func parseAction(content string) (Action, error) {
	s := strings.TrimSpace(content)
	// Strip markdown code fences if the model ignored the instruction.
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	var a Action
	if err := json.Unmarshal([]byte(s), &a); err != nil {
		return Action{}, err
	}
	if a.Final == "" && a.Tool == "" {
		return Action{}, fmt.Errorf("response has neither 'tool' nor 'final'")
	}
	return a, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "\n...[truncated]"
}
