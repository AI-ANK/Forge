package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Grep struct{ Root string }

func (Grep) Name() string { return "grep" }
func (Grep) Description() string {
	return "Search text files for a regex. Args: {\"pattern\":string,\"path\":string(optional,default=.)}. Returns up to 200 matches."
}
func (Grep) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"},"path":{"type":"string"}},"required":["pattern"]}`)
}

func (t Grep) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	if a.Path == "" {
		a.Path = "."
	}
	base, err := resolve(t.Root, a.Path)
	if err != nil {
		return "", err
	}
	re, err := regexp.Compile(a.Pattern)
	if err != nil {
		return "", fmt.Errorf("compile regex: %w", err)
	}
	var sb strings.Builder
	hits := 0
	err = filepath.WalkDir(base, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			if d != nil && d.IsDir() && shouldSkipDir(d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if hits >= 200 {
			return filepath.SkipAll
		}
		f, err := os.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 64*1024), 1024*1024)
		line := 0
		for sc.Scan() {
			line++
			if re.MatchString(sc.Text()) {
				rel, _ := filepath.Rel(t.Root, p)
				fmt.Fprintf(&sb, "%s:%d:%s\n", rel, line, sc.Text())
				hits++
				if hits >= 200 {
					break
				}
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if hits == 0 {
		return "(no matches)", nil
	}
	return sb.String(), nil
}

type Glob struct{ Root string }

func (Glob) Name() string { return "glob" }
func (Glob) Description() string {
	return "List files matching a glob pattern (e.g. **/*.go). Args: {\"pattern\":string}. Returns up to 500 paths."
}
func (Glob) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"pattern":{"type":"string"}},"required":["pattern"]}`)
}

func (t Glob) Run(ctx context.Context, args json.RawMessage) (string, error) {
	var a struct {
		Pattern string `json:"pattern"`
	}
	if err := json.Unmarshal(args, &a); err != nil {
		return "", err
	}
	var out []string
	err := filepath.WalkDir(t.Root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(t.Root, p)
		if matchDoubleStar(a.Pattern, rel) {
			out = append(out, rel)
			if len(out) >= 500 {
				return filepath.SkipAll
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(out) == 0 {
		return "(no matches)", nil
	}
	return strings.Join(out, "\n"), nil
}

func shouldSkipDir(name string) bool {
	switch name {
	case ".git", "node_modules", "vendor", ".venv", "__pycache__", ".forge":
		return true
	}
	return false
}

// matchDoubleStar supports ** for recursive directory wildcards in addition to
// filepath.Match's semantics.
func matchDoubleStar(pattern, name string) bool {
	if !strings.Contains(pattern, "**") {
		ok, _ := filepath.Match(pattern, name)
		if ok {
			return true
		}
		ok, _ = filepath.Match(pattern, filepath.Base(name))
		return ok
	}
	// Translate ** to a regex segment.
	re := regexp.QuoteMeta(pattern)
	re = strings.ReplaceAll(re, `\*\*`, `.*`)
	re = strings.ReplaceAll(re, `\*`, `[^/]*`)
	re = strings.ReplaceAll(re, `\?`, `.`)
	compiled, err := regexp.Compile("^" + re + "$")
	if err != nil {
		return false
	}
	return compiled.MatchString(name)
}
