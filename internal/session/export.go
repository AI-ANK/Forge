package session

import (
	"fmt"
	"os"
)

// Export copies a single session and its events from src into a new store at dstPath.
// The destination file is overwritten if it exists. Any existing data in dst is replaced.
func Export(src *Store, sessionID, dstPath string) error {
	if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	dst, err := Open(dstPath)
	if err != nil {
		return err
	}
	defer dst.Close()

	sess, err := src.GetSession(sessionID)
	if err != nil {
		return fmt.Errorf("get session %s: %w", sessionID, err)
	}
	if err := dst.CreateSession(*sess); err != nil {
		return err
	}
	events, err := src.LoadEvents(sessionID)
	if err != nil {
		return err
	}
	for _, ev := range events {
		if err := dst.AppendEvent(sessionID, ev); err != nil {
			return err
		}
	}
	return nil
}
