package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/AI-ANK/Forge/internal/session"
	"github.com/spf13/cobra"
)

func shareCmd() *cobra.Command {
	var storePath string
	var out string

	cmd := &cobra.Command{
		Use:   "share <session-id>",
		Short: "Export a session to a self-contained .forge file you can share.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessID := args[0]

			cwd, _ := os.Getwd()
			if storePath == "" {
				storePath = defaultStorePath(cwd)
			}
			if out == "" {
				out = filepath.Join(cwd, sessID+".forge")
			}
			store, err := session.Open(storePath)
			if err != nil {
				return err
			}
			defer store.Close()

			if err := session.Export(store, sessID, out); err != nil {
				return err
			}
			info, err := os.Stat(out)
			if err == nil {
				fmt.Printf("forge share: exported session %s to %s (%d bytes)\n", sessID, out, info.Size())
			} else {
				fmt.Printf("forge share: exported session %s to %s\n", sessID, out)
			}
			fmt.Printf("forge share: replay with `forge replay %s`\n", out)
			return nil
		},
	}
	cmd.Flags().StringVar(&storePath, "store", "", "Path to .forge session store (default .forge/sessions.forge)")
	cmd.Flags().StringVarP(&out, "out", "o", "", "Output .forge file path (default <session-id>.forge)")
	return cmd
}
