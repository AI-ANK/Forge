package llm

import (
	"context"
	"strings"
	"testing"
)

func TestMock_ReturnsScriptedResponsesInOrder(t *testing.T) {
	m, err := NewMock("mock-hello")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name() != "mock" {
		t.Errorf("name: %s", m.Name())
	}

	ctx := context.Background()
	seen := 0
	for {
		resp, err := m.Chat(ctx, Request{Model: "mock-hello"})
		if err != nil {
			t.Fatal(err)
		}
		seen++
		if strings.Contains(resp.Content, `"final"`) {
			break
		}
		if seen > 20 {
			t.Fatal("mock did not terminate within 20 turns")
		}
	}
	if seen != 5 {
		t.Errorf("mock-hello: got %d turns, want 5", seen)
	}
}

func TestMock_UnknownModelErrors(t *testing.T) {
	if _, err := NewMock("mock-nope"); err == nil {
		t.Error("expected error for unknown mock script")
	}
}

func TestPick_RoutesMockWithoutKeys(t *testing.T) {
	a, err := Pick("mock-hello", ProviderConfig{})
	if err != nil {
		t.Fatalf("pick mock: %v", err)
	}
	if a.Name() != "mock" {
		t.Errorf("got %s want mock", a.Name())
	}
}
