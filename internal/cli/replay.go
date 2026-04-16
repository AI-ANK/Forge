package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AI-ANK/Forge/internal/session"
	"github.com/spf13/cobra"
)

func replayCmd() *cobra.Command {
	var storePath string
	var step bool

	cmd := &cobra.Command{
		Use:   "replay [session-id-or-file]",
		Short: "Replay a recorded session from its log. No LLM or tools are re-run.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]

			cwd, _ := os.Getwd()
			if storePath == "" {
				if isForgeFile(target) {
					storePath = target
				} else {
					storePath = defaultStorePath(cwd)
				}
			}
			store, err := session.Open(storePath)
			if err != nil {
				return err
			}
			defer store.Close()

			sessID := target
			if isForgeFile(target) {
				id, err := store.LatestSessionID()
				if err != nil {
					return fmt.Errorf("no sessions in %s: %w", target, err)
				}
				sessID = id
			}

			sess, err := store.GetSession(sessID)
			if err != nil {
				return fmt.Errorf("session %s: %w", sessID, err)
			}
			events, err := store.LoadEvents(sessID)
			if err != nil {
				return err
			}

			fmt.Printf("forge replay: session %s  seed=%d  model=%s\n", sess.ID, sess.Seed, sess.Model)
			fmt.Printf("forge replay: goal: %s\n\n", sess.Goal)

			turn := 0
			for _, ev := range events {
				switch ev.Kind {
				case session.KindUserMsg:
					fmt.Printf("[user] goal recorded\n")
				case session.KindLLMResponse:
					turn++
					var resp struct {
						Content string `json:"content"`
					}
					_ = json.Unmarshal(ev.Payload, &resp)
					action := parseThoughtToolFinal(resp.Content)
					printAction(turn, action)
				case session.KindToolResult:
					var tr struct {
						Tool   string `json:"tool"`
						Result string `json:"result"`
					}
					_ = json.Unmarshal(ev.Payload, &tr)
					fmt.Printf("  ← %s\n", shorten(tr.Result, 240))
				case session.KindAgentFinal:
					var a struct {
						Final string `json:"final"`
					}
					_ = json.Unmarshal(ev.Payload, &a)
					fmt.Printf("\nforge replay: done — %s\n", a.Final)
				}
				if step {
					fmt.Fprint(os.Stderr, "[press enter for next event] ")
					var buf [1]byte
					_, _ = os.Stdin.Read(buf[:])
				}
			}
			fmt.Printf("forge replay: %d events played\n", len(events))
			return nil
		},
	}
	cmd.Flags().StringVar(&storePath, "store", "", "Path to .forge session store (default .forge/sessions.forge)")
	cmd.Flags().BoolVar(&step, "step", false, "Pause for enter between events")
	return cmd
}

func isForgeFile(s string) bool {
	return strings.HasSuffix(s, ".forge") || strings.Contains(s, string(filepath.Separator))
}

type thoughtAction struct {
	Thought string          `json:"thought"`
	Tool    string          `json:"tool,omitempty"`
	Args    json.RawMessage `json:"args,omitempty"`
	Final   string          `json:"final,omitempty"`
}

func parseThoughtToolFinal(content string) thoughtAction {
	s := strings.TrimSpace(content)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	var a thoughtAction
	_ = json.Unmarshal([]byte(s), &a)
	return a
}

func printAction(turn int, a thoughtAction) {
	if a.Final != "" {
		return
	}
	fmt.Printf("[turn %d] %s\n", turn, a.Thought)
	if a.Tool != "" {
		fmt.Printf("  → %s %s\n", a.Tool, shorten(string(a.Args), 140))
	}
}
