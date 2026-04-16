package cli

import (
	"fmt"
	"os"

	"github.com/AI-ANK/Forge/internal/session"
	"github.com/spf13/cobra"
)

func sessionsCmd() *cobra.Command {
	var storePath string
	cmd := &cobra.Command{
		Use:   "sessions",
		Short: "List recorded sessions in the local store.",
		RunE: func(cmd *cobra.Command, args []string) error {
			cwd, _ := os.Getwd()
			if storePath == "" {
				storePath = defaultStorePath(cwd)
			}
			store, err := session.Open(storePath)
			if err != nil {
				return err
			}
			defer store.Close()
			sessions, err := store.ListSessions()
			if err != nil {
				return err
			}
			if len(sessions) == 0 {
				fmt.Println("(no sessions)")
				return nil
			}
			fmt.Printf("%-18s %-22s %-24s %s\n", "ID", "CREATED", "MODEL", "GOAL")
			for _, s := range sessions {
				goal := s.Goal
				if len(goal) > 60 {
					goal = goal[:57] + "..."
				}
				fmt.Printf("%-18s %-22s %-24s %s\n",
					s.ID,
					s.CreatedAt.Local().Format("2006-01-02 15:04:05"),
					s.Model,
					goal,
				)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&storePath, "store", "", "Path to .forge session store (default .forge/sessions.forge)")
	return cmd
}
