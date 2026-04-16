package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type ReadFile struct{ Root string }

func (ReadFile) Name() string        { return "read_file" }
func (ReadFile) Description() string { return "Read a text file. Args: {\"path\":string}" }
func (ReadFile) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)
}
func (t ReadFile) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	p, err := resolve(t.Root, a.Path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type WriteFile struct{ Root string }

func (WriteFile) Name() string { return "write_file" }
func (WriteFile) Description() string {
	return "Create or overwrite a file. Args: {\"path\":string,\"content\":string}"
}
func (WriteFile) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)
}
func (t WriteFile) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	p, err := resolve(t.Root, a.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, []byte(a.Content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), a.Path), nil
}

type EditFile struct{ Root string }

func (EditFile) Name() string { return "edit_file" }
func (EditFile) Description() string {
	return "Replace a unique substring in a file. Args: {\"path\":string,\"old\":string,\"new\":string}. Fails if old is not unique or not found."
}
func (EditFile) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"},"old":{"type":"string"},"new":{"type":"string"}},"required":["path","old","new"]}`)
}
func (t EditFile) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
		Old  string `json:"old"`
		New  string `json:"new"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	p, err := resolve(t.Root, a.Path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	s := string(b)
	n := strings.Count(s, a.Old)
	if n == 0 {
		return "", fmt.Errorf("old string not found in %s", a.Path)
	}
	if n > 1 {
		return "", fmt.Errorf("old string matched %d times in %s; make it unique", n, a.Path)
	}
	updated := strings.Replace(s, a.Old, a.New, 1)
	if err := os.WriteFile(p, []byte(updated), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %s (%d bytes)", a.Path, len(updated)), nil
}

// resolve constrains paths to the workspace root to avoid the agent escaping.
func resolve(root, rel string) (string, error) {
	if root == "" {
		root = "."
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	p := rel
	if !filepath.IsAbs(p) {
		p = filepath.Join(absRoot, rel)
	}
	clean := filepath.Clean(p)
	if !strings.HasPrefix(clean, absRoot+string(os.PathSeparator)) && clean != absRoot {
		return "", fmt.Errorf("path %s escapes workspace root %s", clean, absRoot)
	}
	return clean, nil
}
