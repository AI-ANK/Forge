package agent

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/AI-ANK/Forge/internal/llm"
	"github.com/AI-ANK/Forge/internal/rng"
	"github.com/AI-ANK/Forge/internal/session"
	"github.com/AI-ANK/Forge/internal/tools"
)

// TestForkPrep_ResumesFromMidRun verifies the headline fork mechanic:
// a child session reuses the parent's recorded turns 1..N (no model calls)
// and then runs live from turn N+1 with new guidance.
func TestForkPrep_ResumesFromMidRun(t *testing.T) {
	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "sessions.forge")

	// --- PARENT RUN: writes parent.txt, runs shell, finishes. ---
	parent := runMockSession(t, storePath, "parent-goal", tmp, 100, []string{
		`{"thought":"write","tool":"write_file","args":{"path":"parent.txt","content":"A\n"}}`,
		`{"thought":"ls","tool":"shell","args":{"cmd":"ls parent.txt"}}`,
		`{"thought":"done","final":"parent done"}`,
	})

	// --- FORK at turn 2 (after write, after shell): tell it to write child.txt. ---
	store, err := session.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	parentSess, err := store.GetSession(parent.id)
	if err != nil {
		t.Fatal(err)
	}
	parentEvents, err := store.LoadEvents(parent.id)
	if err != nil {
		t.Fatal(err)
	}

	childID := "child1"
	childSeed := int64(200)
	if err := store.CreateSession(session.Session{
		ID: childID, Seed: childSeed, Model: "mock", CreatedAt: time.Now(), Cwd: tmp, Env: map[string]string{}, Goal: "fork",
	}); err != nil {
		t.Fatal(err)
	}
	childRec, err := session.NewRecorder(store, childID)
	if err != nil {
		t.Fatal(err)
	}

	reg := tools.NewRegistry(tools.WriteFile{Root: tmp}, tools.Shell{Root: tmp, Timeout: time.Second})

	msgs, nextTurn, err := ForkPrep(parentSess, parentEvents, childRec, reg, tmp, "write child.txt instead", 2)
	if err != nil {
		t.Fatalf("ForkPrep: %v", err)
	}
	if nextTurn != 3 {
		t.Errorf("nextTurn: got %d want 3", nextTurn)
	}
	// Message history should include: system, user-goal, asst1, tool1, asst2, tool2, user-guidance.
	if len(msgs) != 7 {
		t.Fatalf("messages: got %d want 7", len(msgs))
	}
	if msgs[0].Role != "system" || msgs[1].Role != "user" {
		t.Errorf("header roles wrong: %+v %+v", msgs[0].Role, msgs[1].Role)
	}
	if msgs[6].Role != "user" {
		t.Errorf("guidance role: %s", msgs[6].Role)
	}

	// Now resume live (mock) at turn 3.
	mock := &mockLLM{responses: []string{
		`{"thought":"write child","tool":"write_file","args":{"path":"child.txt","content":"B\n"}}`,
		`{"thought":"done","final":"child done"}`,
	}}
	final, err := Run(context.Background(), Config{
		Model:        "mock",
		Goal:         "fork",
		Cwd:          tmp,
		MaxTurns:     5,
		Recorder:     childRec,
		LLM:          mock,
		Tools:        reg,
		Rng:          rng.New(childSeed, time.Unix(0, 0).UTC()),
		InitMessages: msgs,
		InitTurn:     nextTurn,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if final != "child done" {
		t.Errorf("final: %s", final)
	}

	// Child session should have: user_msg (goal), replayed llm_req/resp pairs + tool events for turns 1..2,
	// user_msg (guidance), plus live llm_req/resp for turns 3..4 and an agent_final.
	events, err := store.LoadEvents(childID)
	if err != nil {
		t.Fatal(err)
	}
	// Count kinds.
	counts := map[session.EventKind]int{}
	for _, ev := range events {
		counts[ev.Kind]++
	}
	if counts[session.KindLLMRequest] != 4 {
		t.Errorf("llm_req count: %d want 4 (2 copied + 2 live)", counts[session.KindLLMRequest])
	}
	if counts[session.KindLLMResponse] != 4 {
		t.Errorf("llm_resp count: %d want 4", counts[session.KindLLMResponse])
	}
	if counts[session.KindAgentFinal] != 1 {
		t.Errorf("agent_final count: %d want 1", counts[session.KindAgentFinal])
	}
	if counts[session.KindUserMsg] != 2 {
		t.Errorf("user_msg count: %d want 2 (goal + guidance)", counts[session.KindUserMsg])
	}
}

type sessionRef struct {
	id   string
	seed int64
}

// runMockSession helper: start a session, run the agent with a canned LLM,
// return the session id.
func runMockSession(t *testing.T, storePath, goal, cwd string, seed int64, responses []string) sessionRef {
	t.Helper()
	store, err := session.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	id := rng.SessionIDFromSeed(seed) + "-p"
	if err := store.CreateSession(session.Session{
		ID: id, Seed: seed, Model: "mock", CreatedAt: time.Now(), Cwd: cwd, Env: map[string]string{}, Goal: goal,
	}); err != nil {
		t.Fatal(err)
	}
	rec, _ := session.NewRecorder(store, id)
	reg := tools.NewRegistry(tools.WriteFile{Root: cwd}, tools.Shell{Root: cwd, Timeout: time.Second})
	mock := &mockLLM{responses: responses}
	if _, err := Run(context.Background(), Config{
		Model:    "mock",
		Goal:     goal,
		Cwd:      cwd,
		MaxTurns: 10,
		Recorder: rec,
		LLM:      mock,
		Tools:    reg,
		Rng:      rng.New(seed, time.Unix(0, 0).UTC()),
	}); err != nil {
		t.Fatal(err)
	}
	return sessionRef{id: id, seed: seed}
}

// Ensure unused import warning is avoided.
var _ = llm.Message{}
