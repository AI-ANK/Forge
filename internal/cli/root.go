package cli

import (
	"github.com/spf13/cobra"
)

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "forge",
		Short:         "Forge — local-first coding agent with deterministic replay",
		Long:          "Forge is a coding agent that runs locally via Ollama and records every step to a .forge session file you can replay, fork, or share.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(runCmd())
	root.AddCommand(replayCmd())
	root.AddCommand(forkCmd())
	root.AddCommand(shareCmd())
	root.AddCommand(sessionsCmd())
	root.AddCommand(bisectCmd())
	return root
}
