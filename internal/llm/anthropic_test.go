package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAnthropic_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-test" {
			t.Errorf("missing api key")
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Errorf("missing anthropic-version")
		}
		var body anthropicRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.System == "" {
			t.Error("system should be populated")
		}
		if body.Temperature != 0 {
			t.Errorf("temperature: %v", body.Temperature)
		}
		// Consecutive tool-converted user messages must be merged.
		for i := 1; i < len(body.Messages); i++ {
			if body.Messages[i].Role == body.Messages[i-1].Role {
				t.Errorf("consecutive %s messages at %d", body.Messages[i].Role, i)
			}
		}
		_ = json.NewEncoder(w).Encode(anthropicResponse{
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{{Type: "text", Text: `{"final":"done"}`}},
			StopReason: "end_turn",
		})
	}))
	defer srv.Close()

	a := NewAnthropic("sk-test")
	a.BaseURL = srv.URL

	resp, err := a.Chat(context.Background(), Request{
		Model: "claude-sonnet-4-6",
		Messages: []Message{
			{Role: "system", Content: "you are forge"},
			{Role: "user", Content: "goal"},
			{Role: "assistant", Content: `{"tool":"x"}`},
			{Role: "tool", Name: "x", Content: "result1"},
			{Role: "tool", Name: "x", Content: "result2"}, // consecutive tools → must merge
		},
		Temperature: 0,
		TopP:        1,
		JSON:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != `{"final":"done"}` {
		t.Errorf("content: %q", resp.Content)
	}
}

func TestSplitSystem_MergesToolMessages(t *testing.T) {
	sys, msgs := splitSystem([]Message{
		{Role: "system", Content: "sys1"},
		{Role: "system", Content: "sys2"},
		{Role: "user", Content: "u"},
		{Role: "assistant", Content: "a"},
		{Role: "tool", Name: "grep", Content: "r1"},
		{Role: "tool", Name: "grep", Content: "r2"},
	})
	if !strings.Contains(sys, "sys1") || !strings.Contains(sys, "sys2") {
		t.Errorf("system merge: %q", sys)
	}
	if len(msgs) != 3 {
		t.Errorf("msg count: got %d want 3", len(msgs))
	}
	if !strings.Contains(msgs[2].Content, "TOOL_RESULT") {
		t.Errorf("tool prefix missing: %q", msgs[2].Content)
	}
	if !strings.Contains(msgs[2].Content, "r1") || !strings.Contains(msgs[2].Content, "r2") {
		t.Errorf("consecutive tools not merged: %q", msgs[2].Content)
	}
}
