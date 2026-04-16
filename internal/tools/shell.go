package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"
)

type Shell struct {
	Root    string
	Timeout time.Duration
}

func (Shell) Name() string { return "shell" }
func (Shell) Description() string {
	return "Run a bash command in the workspace root. Args: {\"cmd\":string}. Captures stdout+stderr+exit code. Timeout 60s."
}
func (Shell) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"cmd":{"type":"string"}},"required":["cmd"]}`)
}

func (t Shell) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Cmd string `json:"cmd"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	timeout := t.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-lc", a.Cmd)
	if t.Root != "" {
		cmd.Dir = t.Root
	}
	out, err := cmd.CombinedOutput()
	exit := 0
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			exit = ee.ExitCode()
		} else {
			return "", err
		}
	}
	return fmt.Sprintf("exit=%d\n%s", exit, string(out)), nil
}
