package llm

import "context"

// Message is a single turn in the chat transcript.
type Message struct {
	Role    string `json:"role"`    // "system" | "user" | "assistant" | "tool"
	Content string `json:"content"`
	Name    string `json:"name,omitempty"` // for tool messages: which tool produced this
}

// Request is what gets sent to the LLM and recorded verbatim.
type Request struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Seed        int64     `json:"seed"`
	Temperature float64   `json:"temperature"`
	TopP        float64   `json:"top_p"`
	JSON        bool      `json:"json"`
}

// Response is what comes back from the LLM and gets recorded verbatim.
type Response struct {
	Content    string `json:"content"`
	RawBody    []byte `json:"raw_body,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// Adapter is the LLM interface. Live implementations call a real model;
// replay implementations return recorded bytes.
type Adapter interface {
	Chat(ctx context.Context, req Request) (Response, error)
	Name() string
}
