package main

import (
	"fmt"
	"os"

	"github.com/AI-ANK/Forge/internal/cli"
)

func main() {
	if err := cli.Root().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "forge:", err)
		os.Exit(1)
	}
}
