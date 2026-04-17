package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOpenAI_Chat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer sk-test" {
			t.Errorf("missing bearer")
		}
		var body openaiRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body.ResponseFormat == nil || body.ResponseFormat.Type != "json_object" {
			t.Errorf("json format not set: %+v", body.ResponseFormat)
		}
		if body.Seed == nil || *body.Seed != 42 {
			t.Errorf("seed: %+v", body.Seed)
		}
		if body.Temperature != 0 {
			t.Errorf("temperature: %v", body.Temperature)
		}
		resp := openaiResponse{
			Choices: []struct {
				Message openaiMsg `json:"message"`
			}{
				{Message: openaiMsg{Role: "assistant", Content: `{"final":"ok"}`}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	o := NewOpenAI("sk-test")
	o.BaseURL = srv.URL

	resp, err := o.Chat(context.Background(), Request{
		Model: "gpt-4.1",
		Messages: []Message{
			{Role: "system", Content: "sys"},
			{Role: "user", Content: "hi"},
			{Role: "tool", Name: "grep", Content: "result"},
		},
		Seed:        42,
		Temperature: 0,
		JSON:        true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Content != `{"final":"ok"}` {
		t.Errorf("content: %q", resp.Content)
	}
}
