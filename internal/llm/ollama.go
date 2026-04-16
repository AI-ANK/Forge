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

// Ollama implements Adapter against a local Ollama server.
type Ollama struct {
	BaseURL string
	Client  *http.Client
}

func NewOllama(baseURL string) *Ollama {
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	return &Ollama{
		BaseURL: baseURL,
		Client:  &http.Client{Timeout: 10 * time.Minute},
	}
}

func (o *Ollama) Name() string { return "ollama" }

type ollamaOptions struct {
	Seed        int64   `json:"seed"`
	Temperature float64 `json:"temperature"`
	TopP        float64 `json:"top_p"`
	NumCtx      int     `json:"num_ctx,omitempty"`
}

type ollamaRequest struct {
	Model    string        `json:"model"`
	Messages []Message     `json:"messages"`
	Stream   bool          `json:"stream"`
	Format   string        `json:"format,omitempty"`
	Options  ollamaOptions `json:"options"`
}

type ollamaResponse struct {
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

func (o *Ollama) Chat(ctx context.Context, req Request) (Response, error) {
	body := ollamaRequest{
		Model:    req.Model,
		Messages: req.Messages,
		Stream:   false,
		Options: ollamaOptions{
			Seed:        req.Seed,
			Temperature: req.Temperature,
			TopP:        req.TopP,
			NumCtx:      8192,
		},
	}
	if req.JSON {
		body.Format = "json"
	}
	buf, err := json.Marshal(body)
	if err != nil {
		return Response{}, err
	}
	start := time.Now()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", o.BaseURL+"/api/chat", bytes.NewReader(buf))
	if err != nil {
		return Response{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := o.Client.Do(httpReq)
	if err != nil {
		return Response{}, fmt.Errorf("ollama request: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Response{}, err
	}
	if resp.StatusCode != 200 {
		return Response{}, fmt.Errorf("ollama status %d: %s", resp.StatusCode, string(raw))
	}
	var parsed ollamaResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Response{}, fmt.Errorf("decode ollama response: %w; raw=%s", err, string(raw))
	}
	return Response{
		Content:    parsed.Message.Content,
		RawBody:    raw,
		DurationMs: time.Since(start).Milliseconds(),
	}, nil
}
