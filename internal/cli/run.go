package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AI-ANK/Forge/internal/agent"
	"github.com/AI-ANK/Forge/internal/llm"
	"github.com/AI-ANK/Forge/internal/rng"
	"github.com/AI-ANK/Forge/internal/session"
	"github.com/AI-ANK/Forge/internal/tools"
	"github.com/spf13/cobra"
)

func runCmd() *cobra.Command {
	var model string
	var ollamaURL string
	var seed int64
	var maxTurns int
	var storePath string

	cmd := &cobra.Command{
		Use:   "run [goal...]",
		Short: "Run the agent on a goal. Records every step to a .forge session.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			goal := strings.Join(args, " ")

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if storePath == "" {
				storePath = defaultStorePath(cwd)
			}
			if err := os.MkdirAll(filepath.Dir(storePath), 0o755); err != nil {
				return err
			}
			if seed == 0 {
				seed = time.Now().UnixNano()
			}

			store, err := session.Open(storePath)
			if err != nil {
				return err
			}
			defer store.Close()

			src := rng.New(seed, time.Unix(0, 0).UTC())
			sessID := rng.SessionIDFromSeed(seed)

			sess := session.Session{
				ID:        sessID,
				Seed:      seed,
				Model:     model,
				CreatedAt: time.Now().UTC(),
				Cwd:       cwd,
				Env:       map[string]string{},
				Goal:      goal,
			}
			if err := store.CreateSession(sess); err != nil {
				return err
			}
			rec, err := session.NewRecorder(store, sessID)
			if err != nil {
				return err
			}

			reg := tools.NewRegistry(
				tools.ReadFile{Root: cwd},
				tools.WriteFile{Root: cwd},
				tools.EditFile{Root: cwd},
				tools.Shell{Root: cwd, Timeout: 60 * time.Second},
				tools.Grep{Root: cwd},
				tools.Glob{Root: cwd},
			)

			ada, err := llm.Pick(model, llm.ProviderConfig{OllamaURL: ollamaURL})
			if err != nil {
				return err
			}

			fmt.Printf("forge: session %s (seed %d, model %s, provider %s)\n", sessID, seed, model, ada.Name())
			fmt.Printf("forge: goal: %s\n\n", goal)

			onTurn := func(turn int, a agent.Action) {
				if a.Final != "" {
					return
				}
				fmt.Printf("[turn %d] %s\n", turn, a.Thought)
				if a.Tool != "" {
					fmt.Printf("  → %s %s\n", a.Tool, shorten(string(a.Args), 140))
				}
			}
			onResult := func(turn int, result string) {
				fmt.Printf("  ← %s\n", shorten(result, 240))
			}

			ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Minute)
			defer cancel()

			final, err := agent.Run(ctx, agent.Config{
				Model:    model,
				Goal:     goal,
				Cwd:      cwd,
				MaxTurns: maxTurns,
				Recorder: rec,
				LLM:      ada,
				Tools:    reg,
				Rng:      src,
				OnTurn:   onTurn,
				OnResult: onResult,
			})
			if err != nil {
				fmt.Printf("\nforge: error: %v\n", err)
				fmt.Printf("forge: session saved at %s (id %s)\n", storePath, sessID)
				return err
			}
			fmt.Printf("\nforge: done — %s\n", final)
			fmt.Printf("forge: session saved at %s (id %s)\n", storePath, sessID)
			fmt.Printf("forge: replay with `forge replay %s`\n", sessID)
			return nil
		},
	}
	cmd.Flags().StringVarP(&model, "model", "m", "qwen2.5-coder:7b", "Ollama model name")
	cmd.Flags().StringVar(&ollamaURL, "ollama-url", "http://127.0.0.1:11434", "Ollama server URL")
	cmd.Flags().Int64Var(&seed, "seed", 0, "Session seed (0 = random)")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 25, "Max agent turns")
	cmd.Flags().StringVar(&storePath, "store", "", "Path to .forge session store (default .forge/sessions.forge)")
	return cmd
}

func defaultStorePath(cwd string) string {
	return filepath.Join(cwd, ".forge", "sessions.forge")
}

func shorten(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
