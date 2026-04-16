package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestOllama_Chat verifies the adapter sends a well-formed payload (seed +
// temp=0 + json format) and parses a canned response.
func TestOllama_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Errorf("path: %s", r.URL.Path)
		}
		var body ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.Options.Seed != 42 || body.Options.Temperature != 0 || body.Options.TopP != 1 {
			t.Errorf("options: %+v", body.Options)
		}
		if body.Format != "json" {
			t.Errorf("expected JSON format, got %q", body.Format)
		}
		resp := ollamaResponse{
			Message: Message{Role: "assistant", Content: `{"final":"ok"}`},
			Done:    true,
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	o := NewOllama(srv.URL)
	resp, err := o.Chat(context.Background(), Request{
		Model:       "test",
		Messages:    []Message{{Role: "user", Content: "hi"}},
		Seed:        42,
		Temperature: 0,
		TopP:        1,
		JSON:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != `{"final":"ok"}` {
		t.Errorf("content: %q", resp.Content)
	}
	if len(resp.RawBody) == 0 {
		t.Error("raw body empty")
	}
}
