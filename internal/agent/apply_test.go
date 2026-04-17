package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AI-ANK/Forge/internal/session"
)

// buildEvents synthesizes a recorded session with tool calls so we can exercise
// ApplyTurns without needing a live LLM.
//
// The sequence models: turn 1 writes a.txt; turn 2 writes b.txt; turn 3 edits
// a.txt (swaps v1→v2); turn 4 shell (no filesystem effect reproducible).
func buildEvents(t *testing.T) []session.Event {
	t.Helper()
	mk := func(kind session.EventKind, payload any) session.Event {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		return session.Event{Kind: kind, Payload: b, TS: time.Now()}
	}
	mkResp := func(content string) session.Event {
		return mk(session.KindLLMResponse, map[string]string{"content": content})
	}

	var evs []session.Event
	evs = append(evs, mk(session.KindUserMsg, map[string]string{"goal": "test"}))

	// turn 1: write a.txt=v1
	evs = append(evs, mkResp(`{"thought":"","tool":"write_file","args":{"path":"a.txt","content":"v1"}}`))
	evs = append(evs, mk(session.KindToolCall, map[string]any{
		"tool": "write_file",
		"args": json.RawMessage(`{"path":"a.txt","content":"v1"}`),
	}))
	evs = append(evs, mk(session.KindToolResult, map[string]string{"tool": "write_file", "result": "wrote 2 bytes"}))

	// turn 2: write b.txt=hello
	evs = append(evs, mkResp(`{"thought":"","tool":"write_file","args":{"path":"b.txt","content":"hello"}}`))
	evs = append(evs, mk(session.KindToolCall, map[string]any{
		"tool": "write_file",
		"args": json.RawMessage(`{"path":"b.txt","content":"hello"}`),
	}))
	evs = append(evs, mk(session.KindToolResult, map[string]string{"tool": "write_file", "result": "wrote 5 bytes"}))

	// turn 3: edit a.txt v1→v2
	evs = append(evs, mkResp(`{"thought":"","tool":"edit_file","args":{"path":"a.txt","old":"v1","new":"v2"}}`))
	evs = append(evs, mk(session.KindToolCall, map[string]any{
		"tool": "edit_file",
		"args": json.RawMessage(`{"path":"a.txt","old":"v1","new":"v2"}`),
	}))
	evs = append(evs, mk(session.KindToolResult, map[string]string{"tool": "edit_file", "result": "edited a.txt"}))

	// turn 4: shell (ignored by Apply)
	evs = append(evs, mkResp(`{"thought":"","tool":"shell","args":{"cmd":"echo hi"}}`))
	evs = append(evs, mk(session.KindToolCall, map[string]any{
		"tool": "shell",
		"args": json.RawMessage(`{"cmd":"echo hi"}`),
	}))
	evs = append(evs, mk(session.KindToolResult, map[string]string{"tool": "shell", "result": "exit=0\nhi"}))

	return evs
}

func TestApplyTurns_ProgressiveState(t *testing.T) {
	events := buildEvents(t)

	cases := []struct {
		turn int
		want map[string]string // file → expected contents; missing key = file must not exist
	}{
		{0, map[string]string{}},
		{1, map[string]string{"a.txt": "v1"}},
		{2, map[string]string{"a.txt": "v1", "b.txt": "hello"}},
		{3, map[string]string{"a.txt": "v2", "b.txt": "hello"}},
		{4, map[string]string{"a.txt": "v2", "b.txt": "hello"}}, // shell is a no-op
	}
	for _, c := range cases {
		dir := t.TempDir()
		if err := ApplyTurns(events, c.turn, dir); err != nil {
			t.Fatalf("turn %d: ApplyTurns: %v", c.turn, err)
		}
		for name, want := range c.want {
			b, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				t.Errorf("turn %d: read %s: %v", c.turn, name, err)
				continue
			}
			if string(b) != want {
				t.Errorf("turn %d: %s = %q, want %q", c.turn, name, string(b), want)
			}
		}
		// Check that no unexpected files exist.
		entries, _ := os.ReadDir(dir)
		if len(entries) != len(c.want) {
			names := []string{}
			for _, e := range entries {
				names = append(names, e.Name())
			}
			t.Errorf("turn %d: dir has %v, want %d files", c.turn, names, len(c.want))
		}
	}
}

func TestApplyTurns_SkipsErroredCalls(t *testing.T) {
	mk := func(kind session.EventKind, payload any) session.Event {
		b, _ := json.Marshal(payload)
		return session.Event{Kind: kind, Payload: b, TS: time.Now()}
	}
	mkResp := func(content string) session.Event {
		return mk(session.KindLLMResponse, map[string]string{"content": content})
	}

	// turn 1: edit_file that errored (nonexistent file). The tool result starts
	// with "ERROR:" so Apply should skip it rather than blow up.
	evs := []session.Event{
		mkResp(`{"thought":"","tool":"edit_file","args":{"path":"missing.txt","old":"x","new":"y"}}`),
		mk(session.KindToolCall, map[string]any{
			"tool": "edit_file",
			"args": json.RawMessage(`{"path":"missing.txt","old":"x","new":"y"}`),
		}),
		mk(session.KindToolResult, map[string]string{"tool": "edit_file", "result": "ERROR: file not found"}),
	}

	dir := t.TempDir()
	if err := ApplyTurns(evs, 1, dir); err != nil {
		t.Fatalf("ApplyTurns: %v", err)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Errorf("expected empty dir after errored edit, got %d entries", len(entries))
	}
}
