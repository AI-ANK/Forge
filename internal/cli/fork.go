package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AI-ANK/Forge/internal/agent"
	"github.com/AI-ANK/Forge/internal/llm"
	"github.com/AI-ANK/Forge/internal/rng"
	"github.com/AI-ANK/Forge/internal/session"
	"github.com/AI-ANK/Forge/internal/tools"
	"github.com/spf13/cobra"
)

func forkCmd() *cobra.Command {
	var storePath string
	var atTurn int
	var model string
	var ollamaURL string
	var maxTurns int

	cmd := &cobra.Command{
		Use:   "fork <parent-id> --at-turn <N> <new-guidance...>",
		Short: "Fork a recorded session at turn N and continue with new guidance.",
		Long: `Replays the parent session's first N turns from the recorded log (no
model calls, no cost) and then goes live from turn N+1 with your new
guidance. The fork is a self-contained new session.`,
		Args: cobra.MinimumNArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			parentID := args[0]
			guidance := strings.Join(args[1:], " ")

			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			if storePath == "" {
				storePath = defaultStorePath(cwd)
			}
			store, err := session.Open(storePath)
			if err != nil {
				return err
			}
			defer store.Close()

			parentSess, err := store.GetSession(parentID)
			if err != nil {
				return fmt.Errorf("parent session %s: %w", parentID, err)
			}
			parentEvents, err := store.LoadEvents(parentID)
			if err != nil {
				return err
			}

			// Child session: derive seed from parent+turn so forks are reproducible.
			childSeed := parentSess.Seed ^ (int64(atTurn) << 32) ^ time.Now().UnixNano()
			childID := rng.SessionIDFromSeed(childSeed)
			childSess := session.Session{
				ID:        childID,
				Seed:      childSeed,
				Model:     model,
				CreatedAt: time.Now().UTC(),
				Cwd:       cwd,
				Env:       map[string]string{},
				Goal:      fmt.Sprintf("(fork of %s @ turn %d) %s", parentID, atTurn, guidance),
			}
			if err := store.CreateSession(childSess); err != nil {
				return err
			}
			rec, err := session.NewRecorder(store, childID)
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

			fmt.Printf("forge fork: replaying parent %s up to turn %d (free, no model calls)\n", parentID, atTurn)
			initMsgs, nextTurn, err := agent.ForkPrep(parentSess, parentEvents, rec, reg, cwd, guidance, atTurn)
			if err != nil {
				return fmt.Errorf("fork prep: %w", err)
			}
			fmt.Printf("forge fork: resuming live at turn %d with new guidance: %s\n", nextTurn, guidance)
			fmt.Printf("forge fork: child session %s (model %s)\n\n", childID, model)

			src := rng.New(childSeed, time.Unix(0, 0).UTC())
			ada := llm.NewOllama(ollamaURL)

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
				Model:        model,
				Goal:         childSess.Goal,
				Cwd:          cwd,
				MaxTurns:     maxTurns,
				Recorder:     rec,
				LLM:          ada,
				Tools:        reg,
				Rng:          src,
				OnTurn:       onTurn,
				OnResult:     onResult,
				InitMessages: initMsgs,
				InitTurn:     nextTurn,
			})
			if err != nil {
				fmt.Printf("\nforge fork: error: %v\n", err)
				fmt.Printf("forge fork: session saved at %s (id %s)\n", storePath, childID)
				return err
			}
			fmt.Printf("\nforge fork: done — %s\n", final)
			fmt.Printf("forge fork: session saved at %s (id %s)\n", storePath, childID)
			fmt.Printf("forge fork: replay with `forge replay %s`\n", childID)
			return nil
		},
	}
	cmd.Flags().StringVar(&storePath, "store", "", "Path to .forge session store (default .forge/sessions.forge)")
	cmd.Flags().IntVar(&atTurn, "at-turn", 0, "Parent turn to fork from (1-indexed)")
	cmd.Flags().StringVarP(&model, "model", "m", "qwen2.5-coder:7b", "Ollama model for the live portion")
	cmd.Flags().StringVar(&ollamaURL, "ollama-url", "http://127.0.0.1:11434", "Ollama server URL")
	cmd.Flags().IntVar(&maxTurns, "max-turns", 25, "Max additional turns after fork")
	_ = cmd.MarkFlagRequired("at-turn")
	return cmd
}
