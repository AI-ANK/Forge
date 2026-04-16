package session

import (
	"path/filepath"
	"testing"
	"time"
)

// TestRecorderReplayRoundTrip verifies the core determinism substrate:
// events written by the Recorder are read back in the same order with
// byte-identical payloads.
func TestRecorderReplayRoundTrip(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "s.forge")

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	sess := Session{
		ID: "abc", Seed: 7, Model: "m", CreatedAt: time.Unix(1700000000, 0),
		Cwd: "/x", Env: map[string]string{"K": "V"}, Goal: "g",
	}
	if err := store.CreateSession(sess); err != nil {
		t.Fatal(err)
	}
	rec, err := NewRecorder(store, "abc")
	if err != nil {
		t.Fatal(err)
	}
	type payload struct {
		A string `json:"a"`
		B int    `json:"b"`
	}
	writes := []struct {
		kind EventKind
		p    payload
	}{
		{KindUserMsg, payload{"hello", 1}},
		{KindLLMRequest, payload{"req", 2}},
		{KindLLMResponse, payload{"resp", 3}},
		{KindToolCall, payload{"call", 4}},
		{KindToolResult, payload{"result", 5}},
	}
	for _, w := range writes {
		if err := rec.Record(w.kind, w.p); err != nil {
			t.Fatal(err)
		}
	}
	store.Close()

	store2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store2.Close()
	events, err := store2.LoadEvents("abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != len(writes) {
		t.Fatalf("count: got %d want %d", len(events), len(writes))
	}
	for i, ev := range events {
		if ev.Step != i {
			t.Errorf("step %d: got %d", i, ev.Step)
		}
		if ev.Kind != writes[i].kind {
			t.Errorf("kind %d: got %s want %s", i, ev.Kind, writes[i].kind)
		}
	}

	// Player returns events in order, Next mismatches fail loudly.
	p := NewPlayer(events)
	var got payload
	if err := p.NextAs(KindUserMsg, &got); err != nil {
		t.Fatal(err)
	}
	if got != writes[0].p {
		t.Fatal("payload mismatch")
	}
	if _, err := p.Next(KindToolCall); err == nil {
		t.Fatal("expected mismatch error")
	}
}
