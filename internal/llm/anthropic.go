package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Anthropic implements Adapter against the Claude Messages API.
// It uses plain JSON output (not native tool_use) so the agent loop's JSON
// parser works uniformly across providers. Tool results are relayed to the
// model as user messages with a "TOOL_RESULT(<name>):" prefix.
type Anthropic struct {
	BaseURL   string
	APIKey    string
	Version   string // "2023-06-01" by default
	MaxTokens int
	Client    *http.Client
}

func NewAnthropic(apiKey string) *Anthropic {
	return &Anthropic{
		BaseURL:   "https://api.anthropic.com",
		APIKey:    apiKey,
		Version:   "2023-06-01",
		MaxTokens: 4096,
		Client:    &http.Client{Timeout: 10 * time.Minute},
	}
}

func (a *Anthropic) Name() string { return "anthropic" }

type anthropicMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model       string         `json:"model"`
	System      string         `json:"system,omitempty"`
	Messages    []anthropicMsg `json:"messages"`
	MaxTokens   int            `json:"max_tokens"`
	Temperature float64        `json:"temperature"`
	TopP        float64        `json:"top_p,omitempty"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func (a *Anthropic) Chat(ctx context.Context, req Request) (Response, error) {
	system, msgs := splitSystem(req.Messages)
	body := anthropicRequest{
		Model:       req.Model,
		System:      system,
		Messages:    msgs,
		MaxTokens:   a.MaxTokens,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	// Anthropic disallows top_p together with temperature=0 in some configs; omit if 1.
	if body.TopP == 1 {
		body.TopP = 0
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", a.BaseURL+"/v1/messages", bytes.NewReader(buf))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", a.APIKey)
	httpReq.Header.Set("anthropic-version", a.Version)
	resp, err := a.Client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("anthropic request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode != 200 {
		return Response{}, fmt.Errorf("anthropic status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed anthropicResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, fmt.Errorf("decode anthropic response: %w; raw=%s", err, string(raw))
	}
	content := ""
	for _, blk := range parsed.Content {
		if blk.Type == "text" {
			content += blk.Text
		}
	}
	return Response{
		Content:    content,
		RawBody:    raw,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}

// splitSystem pulls the system message out of the list and converts tool
// messages into user messages for the Anthropic API.
func splitSystem(messages []Message) (system string, out []anthropicMsg) {
	for _, m := range messages {
		switch m.Role {
		case "system":
			if system != "" {
				system += "\n\n"
			}
			system += m.Content
		case "tool":
			out = append(out, anthropicMsg{
				Role:    "user",
				Content: fmt.Sprintf("TOOL_RESULT(%s):\n%s", m.Name, m.Content),
			})
		default:
			out = append(out, anthropicMsg{Role: m.Role, Content: m.Content})
		}
	}
	return system, mergeConsecutiveRoles(out)
}

// mergeConsecutiveRoles collapses consecutive same-role messages since
// Anthropic requires strictly alternating user/assistant turns.
func mergeConsecutiveRoles(in []anthropicMsg) []anthropicMsg {
	if len(in) == 0 {
		return in
	}
	out := []anthropicMsg{in[0]}
	for i := 1; i < len(in); i++ {
		last := &out[len(out)-1]
		if last.Role == in[i].Role {
			last.Content += "\n\n" + in[i].Content
		} else {
			out = append(out, in[i])
		}
	}
	return out
}
