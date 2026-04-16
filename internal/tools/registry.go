package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
)

// Tool is a single capability the agent can invoke.
type Tool interface {
	Name() string
	Description() string
	Schema() json.RawMessage // JSON schema string for the tool's args
	Run(ctx context.Context, args json.RawMessage) (string, error)
}

// Registry is the lookup table of tools by name.
type Registry struct {
	tools map[string]Tool
}

func NewRegistry(tools ...Tool) *Registry {
	r := &Registry{tools: map[string]Tool{}}
	for _, t := range tools {
		r.tools[t.Name()] = t
	}
	return r
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) Dispatch(ctx context.Context, name string, args json.RawMessage) (string, error) {
	t, ok := r.Get(name)
	if !ok {
		return "", fmt.Errorf("unknown tool %q (available: %v)", name, r.Names())
	}
	return t.Run(ctx, args)
}

// Catalog renders a compact description of all tools for the system prompt.
func (r *Registry) Catalog() string {
	names := r.Names()
	out := ""
	for _, n := range names {
		t := r.tools[n]
		out += fmt.Sprintf("- %s: %s\n  args schema: %s\n", t.Name(), t.Description(), string(t.Schema()))
	}
	return out
}
