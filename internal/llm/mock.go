package llm

import (
	"context"
	"fmt"
	"strings"
	"sync"
)

// Mock is a scripted LLM adapter used for zero-config demos and tests.
// It requires no network, API key, or Ollama install. The script is selected
// by the model name: `mock-hello` writes hello.go + a test and finishes.
//
// Mock is deterministic: calls return script entries in order. The seed and
// message history are ignored. This is fine for demos — the recording
// substrate still captures the full request/response bytes, so replay, fork,
// share, and bisect all work against mock sessions exactly as with a real
// provider.
type Mock struct {
	model  string
	script []string

	mu   sync.Mutex
	step int
}

// NewMock returns a mock adapter for the given model name. Returns an error
// if the name does not match a built-in script.
func NewMock(model string) (*Mock, error) {
	s, ok := mockScripts[model]
	if !ok {
		names := make([]string, 0, len(mockScripts))
		for k := range mockScripts {
			names = append(names, k)
		}
		return nil, fmt.Errorf("no mock script for %q (available: %v)", model, names)
	}
	return &Mock{model: model, script: s}, nil
}

func (*Mock) Name() string { return "mock" }

func (m *Mock) Chat(ctx context.Context, req Request) (Response, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.step >= len(m.script) {
		return Response{Content: `{"final":"mock script exhausted"}`}, nil
	}
	content := m.script[m.step]
	m.step++
	return Response{Content: content, DurationMs: 0}, nil
}

// mockScripts maps model name → scripted JSON responses, one per turn.
// Each response must parse as an agent.Action ({thought, tool, args} or {final}).
var mockScripts = map[string][]string{
	"mock-hello": {
		`{"thought":"list existing .go files","tool":"glob","args":{"pattern":"**/*.go"}}`,
		`{"thought":"create hello.go","tool":"write_file","args":{"path":"hello.go","content":"package main\n\nimport \"fmt\"\n\nfunc hello() string { return \"hi\" }\n\nfunc main() { fmt.Println(hello()) }\n"}}`,
		`{"thought":"create test","tool":"write_file","args":{"path":"hello_test.go","content":"package main\n\nimport \"testing\"\n\nfunc TestHello(t *testing.T) { if hello() != \"hi\" { t.Fatal(\"expected hi\") } }\n"}}`,
		`{"thought":"verify both files exist","tool":"shell","args":{"cmd":"ls -1 hello.go hello_test.go"}}`,
		`{"final":"Added hello.go and a passing test."}`,
	},
	// mock-break simulates an agent that breaks its own test at turn 3.
	// Useful for demoing forge bisect.
	"mock-break": {
		`{"thought":"write source","tool":"write_file","args":{"path":"answer.txt","content":"good\n"}}`,
		`{"thought":"write a second file","tool":"write_file","args":{"path":"notes.txt","content":"unrelated\n"}}`,
		`{"thought":"oops, refactor breaks it","tool":"edit_file","args":{"path":"answer.txt","old":"good","new":"bad"}}`,
		`{"thought":"add one more file","tool":"write_file","args":{"path":"trailing.txt","content":"x\n"}}`,
		`{"final":"done (but the check fails now)"}`,
	},
}

// isMockModel returns true when the model name matches a mock script prefix.
func isMockModel(name string) bool {
	return strings.HasPrefix(name, "mock-")
}
