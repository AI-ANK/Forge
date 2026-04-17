package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AI-ANK/Forge/internal/session"
)

// ApplyTurns reconstructs workspace state after turn `upToTurn` by re-executing
// file-modifying tool calls from a recorded session's event log into `dstDir`.
//
// Only write_file and edit_file are applied. Shell commands are skipped because
// re-executing them would re-run tests/builds and possibly have side effects
// outside the workspace; if a bisect depends on build artifacts, the caller's
// check command should produce them.
//
// `upToTurn` is 1-indexed. Pass 0 to apply nothing (initial state).
func ApplyTurns(events []session.Event, upToTurn int, dstDir string) error {
	turn := 0
	var pendingTool string
	var pendingArgs json.RawMessage
	for _, ev := range events {
		switch ev.Kind {
		case session.KindLLMResponse:
			turn++
			if turn > upToTurn {
				return nil
			}
			pendingTool = ""
			pendingArgs = nil
		case session.KindToolCall:
			if turn > upToTurn {
				continue
			}
			var tc struct {
				Tool string          `json:"tool"`
				Args json.RawMessage `json:"args"`
			}
			if err := json.Unmarshal(ev.Payload, &tc); err != nil {
				return fmt.Errorf("decode tool_call at turn %d: %w", turn, err)
			}
			pendingTool = tc.Tool
			pendingArgs = tc.Args
		case session.KindToolResult:
			if turn > upToTurn {
				continue
			}
			var tr struct {
				Tool   string `json:"tool"`
				Result string `json:"result"`
			}
			if err := json.Unmarshal(ev.Payload, &tr); err != nil {
				return fmt.Errorf("decode tool_result at turn %d: %w", turn, err)
			}
			// Skip turns whose tool call errored — the original run didn't
			// change the workspace either.
			if strings.HasPrefix(tr.Result, "ERROR:") {
				pendingTool = ""
				pendingArgs = nil
				continue
			}
			name := tr.Tool
			if name == "" {
				name = pendingTool
			}
			if err := applyOne(name, pendingArgs, dstDir); err != nil {
				return fmt.Errorf("apply turn %d %s: %w", turn, name, err)
			}
			pendingTool = ""
			pendingArgs = nil
		}
	}
	return nil
}

func applyOne(tool string, args json.RawMessage, dstDir string) error {
	switch tool {
	case "write_file":
		var a struct {
			Path    string `json:"path"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return err
		}
		p, err := safeJoin(dstDir, a.Path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		return os.WriteFile(p, []byte(a.Content), 0o644)
	case "edit_file":
		var a struct {
			Path string `json:"path"`
			Old  string `json:"old"`
			New  string `json:"new"`
		}
		if err := json.Unmarshal(args, &a); err != nil {
			return err
		}
		p, err := safeJoin(dstDir, a.Path)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		s := string(b)
		n := strings.Count(s, a.Old)
		if n != 1 {
			return fmt.Errorf("edit_file: old string matched %d times in %s", n, a.Path)
		}
		updated := strings.Replace(s, a.Old, a.New, 1)
		return os.WriteFile(p, []byte(updated), 0o644)
	default:
		// read_file, shell, grep, glob: no filesystem effect we can reconstruct.
		return nil
	}
}

func safeJoin(root, rel string) (string, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	p := rel
	if !filepath.IsAbs(p) {
		p = filepath.Join(absRoot, rel)
	}
	clean := filepath.Clean(p)
	if !strings.HasPrefix(clean, absRoot+string(os.PathSeparator)) && clean != absRoot {
		return "", fmt.Errorf("path %s escapes root %s", clean, absRoot)
	}
	return clean, nil
}
