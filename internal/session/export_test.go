package session

import (
	"path/filepath"
	"testing"
	"time"
)

func TestExport_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "src.forge")
	dst := filepath.Join(tmp, "dst.forge")

	s1, err := Open(src)
	if err != nil {
		t.Fatal(err)
	}
	sess := Session{
		ID: "abc", Seed: 9, Model: "m", CreatedAt: time.Unix(1700000000, 0),
		Cwd: "/x", Env: map[string]string{"K": "V"}, Goal: "goal",
	}
	if err := s1.CreateSession(sess); err != nil {
		t.Fatal(err)
	}
	rec, _ := NewRecorder(s1, "abc")
	for i := 0; i < 5; i++ {
		if err := rec.Record(KindLLMResponse, map[string]int{"i": i}); err != nil {
			t.Fatal(err)
		}
	}
	if err := Export(s1, "abc", dst); err != nil {
		t.Fatalf("export: %v", err)
	}
	s1.Close()

	s2, err := Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	got, err := s2.GetSession("abc")
	if err != nil {
		t.Fatal(err)
	}
	if got.Goal != "goal" || got.Seed != 9 {
		t.Errorf("session fields drifted: %+v", got)
	}
	events, err := s2.LoadEvents("abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 5 {
		t.Errorf("events: got %d want 5", len(events))
	}
}
