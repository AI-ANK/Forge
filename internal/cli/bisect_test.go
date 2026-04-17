package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AI-ANK/Forge/internal/session"
)

// TestBisect_FindsBreakingTurn simulates a 4-turn session where turn 3 is the
// one that introduces a regression (edits a.txt from "good" to "bad"), then
// runs forge bisect with a check that greps for "good" in a.txt.
func TestBisect_FindsBreakingTurn(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("bisect e2e uses sh; skip on windows")
	}

	tmp := t.TempDir()
	storePath := filepath.Join(tmp, "sessions.forge")
	store, err := session.Open(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	sess := session.Session{
		ID:        "bisect-test",
		Seed:      1,
		Model:     "test",
		CreatedAt: time.Now().UTC(),
		Cwd:       tmp,
		Env:       map[string]string{},
		Goal:      "demo",
	}
	if err := store.CreateSession(sess); err != nil {
		t.Fatal(err)
	}

	step := 0
	append := func(kind session.EventKind, payload any) {
		b, err := json.Marshal(payload)
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendEvent(sess.ID, session.Event{Step: step, Kind: kind, Payload: b, TS: time.Now()}); err != nil {
			t.Fatal(err)
		}
		step++
	}
	resp := func(content string) { append(session.KindLLMResponse, map[string]string{"content": content}) }
	call := func(tool string, args string) {
		append(session.KindToolCall, map[string]any{"tool": tool, "args": json.RawMessage(args)})
	}
	result := func(tool, res string) {
		append(session.KindToolResult, map[string]string{"tool": tool, "result": res})
	}

	append(session.KindUserMsg, map[string]string{"goal": "demo"})

	// turn 1: write a.txt=good
	resp(`{"thought":"init","tool":"write_file","args":{"path":"a.txt","content":"good\n"}}`)
	call("write_file", `{"path":"a.txt","content":"good\n"}`)
	result("write_file", "wrote 5 bytes")

	// turn 2: write b.txt (unrelated — should still be good)
	resp(`{"thought":"side","tool":"write_file","args":{"path":"b.txt","content":"hello\n"}}`)
	call("write_file", `{"path":"b.txt","content":"hello\n"}`)
	result("write_file", "wrote 6 bytes")

	// turn 3: edit a.txt good→bad (this is the breaking turn)
	resp(`{"thought":"break","tool":"edit_file","args":{"path":"a.txt","old":"good","new":"bad"}}`)
	call("edit_file", `{"path":"a.txt","old":"good","new":"bad"}`)
	result("edit_file", "edited a.txt")

	// turn 4: write c.txt (after the break)
	resp(`{"thought":"more","tool":"write_file","args":{"path":"c.txt","content":"x\n"}}`)
	call("write_file", `{"path":"c.txt","content":"x\n"}`)
	result("write_file", "wrote 2 bytes")

	// Invoke the bisect command.
	cmd := bisectCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs([]string{
		"bisect-test",
		"--store", storePath,
		"--good", "1",
		"--bad", "4",
		"--", "sh", "-c", "grep -q good a.txt",
	})

	// Capture os.Stdout because the command uses fmt.Printf.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	err = cmd.Execute()
	_ = w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String() + out.String()
	if err != nil {
		t.Fatalf("bisect: %v\noutput:\n%s", err, output)
	}

	if !strings.Contains(output, "first bad turn is 3") {
		t.Errorf("expected 'first bad turn is 3' in output, got:\n%s", output)
	}
}
