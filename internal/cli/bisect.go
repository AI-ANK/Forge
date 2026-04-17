package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AI-ANK/Forge/internal/agent"
	"github.com/AI-ANK/Forge/internal/session"
	"github.com/spf13/cobra"
)

func bisectCmd() *cobra.Command {
	var storePath string
	var good, bad int
	var initDir string
	var keep bool

	cmd := &cobra.Command{
		Use:   "bisect <session-id-or-file> -- <check-cmd> [args...]",
		Short: "Binary-search which turn of a recorded session broke an external check.",
		Long: `Binary-search a recorded session to find the first turn at which an external
check command starts failing.

For each candidate turn N, forge reconstructs the workspace by applying every
recorded write_file and edit_file operation from turns 1..N into a fresh temp
directory, then runs <check-cmd> in that directory. The check is "good" when
exit code is 0 and "bad" otherwise.

Shell tool calls from the session are not re-executed. If your check depends on
build artifacts, make the check command produce them (e.g., 'go test ./...').`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			target := args[0]
			checkCmd := args[1:]
			if len(checkCmd) == 0 {
				return fmt.Errorf("no check command provided; use `--` before the check")
			}

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
			totalTurns := countTurns(events)
			if bad <= 0 {
				bad = totalTurns
			}
			if good < 0 || good > totalTurns {
				return fmt.Errorf("--good %d out of range (session has %d turns)", good, totalTurns)
			}
			if bad < 1 || bad > totalTurns {
				return fmt.Errorf("--bad %d out of range (session has %d turns)", bad, totalTurns)
			}
			if good >= bad {
				return fmt.Errorf("--good (%d) must be less than --bad (%d)", good, bad)
			}

			fmt.Printf("forge bisect: session %s  turns=%d  good=%d  bad=%d\n", sess.ID, totalTurns, good, bad)
			fmt.Printf("forge bisect: check: %v\n\n", checkCmd)

			runAt := func(n int) (pass bool, dir string, err error) {
				dir, err = os.MkdirTemp("", fmt.Sprintf("forge-bisect-%d-*", n))
				if err != nil {
					return false, "", err
				}
				if initDir != "" {
					if err := copyTree(initDir, dir); err != nil {
						return false, dir, fmt.Errorf("copy init dir: %w", err)
					}
				}
				if err := agent.ApplyTurns(events, n, dir); err != nil {
					return false, dir, err
				}
				c := exec.Command(checkCmd[0], checkCmd[1:]...)
				c.Dir = dir
				out, runErr := c.CombinedOutput()
				exitCode := 0
				if runErr != nil {
					if ee, ok := runErr.(*exec.ExitError); ok {
						exitCode = ee.ExitCode()
					} else {
						return false, dir, fmt.Errorf("check failed to run: %w (output: %s)", runErr, shorten(string(out), 400))
					}
				}
				pass = exitCode == 0
				return pass, dir, nil
			}

			cleanup := func(dir string) {
				if keep || dir == "" {
					return
				}
				_ = os.RemoveAll(dir)
			}

			goodPass, goodDir, err := runAt(good)
			if err != nil {
				return err
			}
			cleanup(goodDir)
			if !goodPass {
				return fmt.Errorf("check FAILED at --good=%d; endpoint is not actually good", good)
			}
			fmt.Printf("  turn %d: PASS (good endpoint)\n", good)

			badPass, badDir, err := runAt(bad)
			if err != nil {
				return err
			}
			cleanup(badDir)
			if badPass {
				return fmt.Errorf("check PASSED at --bad=%d; endpoint is not actually bad", bad)
			}
			fmt.Printf("  turn %d: FAIL (bad endpoint)\n", bad)

			// Binary search for the first bad turn.
			// Invariant: state at `lo` is good; state at `hi` is bad.
			lo, hi := good, bad
			for hi-lo > 1 {
				mid := (lo + hi) / 2
				pass, dir, err := runAt(mid)
				if err != nil {
					return err
				}
				cleanup(dir)
				if pass {
					fmt.Printf("  turn %d: PASS\n", mid)
					lo = mid
				} else {
					fmt.Printf("  turn %d: FAIL\n", mid)
					hi = mid
				}
			}

			fmt.Printf("\nforge bisect: first bad turn is %d\n", hi)
			printTurnSummary(events, hi)
			return nil
		},
	}
	cmd.Flags().StringVar(&storePath, "store", "", "Path to .forge session store (default .forge/sessions.forge)")
	cmd.Flags().IntVar(&good, "good", 0, "Turn known to be good (default: 0 = pre-run state)")
	cmd.Flags().IntVar(&bad, "bad", 0, "Turn known to be bad (default: last turn in session)")
	cmd.Flags().StringVar(&initDir, "init-dir", "", "Directory to copy as the pre-run workspace (default: empty)")
	cmd.Flags().BoolVar(&keep, "keep", false, "Keep temp directories for inspection")
	return cmd
}

func countTurns(events []session.Event) int {
	n := 0
	for _, ev := range events {
		if ev.Kind == session.KindLLMResponse {
			n++
		}
	}
	return n
}

func printTurnSummary(events []session.Event, turn int) {
	cur := 0
	for _, ev := range events {
		if ev.Kind != session.KindLLMResponse {
			continue
		}
		cur++
		if cur != turn {
			continue
		}
		var resp struct {
			Content string `json:"content"`
		}
		_ = json.Unmarshal(ev.Payload, &resp)
		a := parseThoughtToolFinal(resp.Content)
		fmt.Printf("  [turn %d] %s\n", turn, a.Thought)
		if a.Tool != "" {
			fmt.Printf("    → %s %s\n", a.Tool, shorten(string(a.Args), 200))
		}
		return
	}
}

func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		out := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(out, info.Mode())
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		return os.WriteFile(out, b, info.Mode())
	})
}
