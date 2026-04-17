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

// OpenAI implements Adapter against the Chat Completions API.
// Uses response_format=json_object to constrain output the same way the
// Ollama adapter uses format:"json".
type OpenAI struct {
	BaseURL string
	APIKey  string
	Client  *http.Client
}

func NewOpenAI(apiKey string) *OpenAI {
	return &OpenAI{
		BaseURL: "https://api.openai.com",
		APIKey:  apiKey,
		Client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

func (o *OpenAI) Name() string { return "openai" }

type openaiMsg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRequest struct {
	Model          string      `json:"model"`
	Messages       []openaiMsg `json:"messages"`
	Temperature    float64     `json:"temperature"`
	TopP           float64     `json:"top_p,omitempty"`
	Seed           *int64      `json:"seed,omitempty"`
	ResponseFormat *openaiRF   `json:"response_format,omitempty"`
}

type openaiRF struct {
	Type string `json:"type"`
}

type openaiResponse struct {
	Choices []struct {
		Message openaiMsg `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
}

func (o *OpenAI) Chat(ctx context.Context, req Request) (Response, error) {
	msgs := make([]openaiMsg, 0, len(req.Messages))
	for _, m := range req.Messages {
		role := m.Role
		content := m.Content
		if m.Role == "tool" {
			role = "user"
			content = fmt.Sprintf("TOOL_RESULT(%s):\n%s", m.Name, m.Content)
		}
		msgs = append(msgs, openaiMsg{Role: role, Content: content})
	}
	body := openaiRequest{
		Model:       req.Model,
		Messages:    msgs,
		Temperature: req.Temperature,
		TopP:        req.TopP,
	}
	if req.Seed != 0 {
		s := req.Seed
		body.Seed = &s
	}
	if req.JSON {
		body.ResponseFormat = &openaiRF{Type: "json_object"}
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/v1/chat/completions", bytes.NewReader(buf))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.APIKey)
	resp, err := o.Client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("openai request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode != 200 {
		return Response{}, fmt.Errorf("openai status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed openaiResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, fmt.Errorf("decode openai response: %w; raw=%s", err, string(raw))
	}
	content := ""
	if len(parsed.Choices) > 0 {
		content = parsed.Choices[0].Message.Content
	}
	return Response{
		Content:    content,
		RawBody:    raw,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
