package provider

import (
	"context"
	"testing"
)

type mockProvider struct {
	name string
}

func (m *mockProvider) Name() string         { return m.name }
func (m *mockProvider) SupportsVision() bool { return true }
func (m *mockProvider) ReadFromImage(ctx context.Context, image []byte, systemPrompt, userPrompt string) (string, error) {
	return "", nil
}
func (m *mockProvider) Translate(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "", nil
}

func TestRegistryRegisterAndGet(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "test"})

	p, err := reg.Get("test")
	if err != nil {
		t.Fatalf("Get() error: %v", err)
	}
	if p.Name() != "test" {
		t.Errorf("expected name 'test', got %q", p.Name())
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	reg := NewRegistry()
	_, err := reg.Get("nonexistent")
	if err == nil {
		t.Fatal("Get() expected error for unknown provider")
	}
}

func TestRegistryNames(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "alpha"})
	reg.Register(&mockProvider{name: "beta"})

	names := reg.Names()
	if len(names) != 2 {
		t.Fatalf("expected 2 names, got %d", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["alpha"] || !nameSet["beta"] {
		t.Errorf("expected alpha and beta, got %v", names)
	}
}

func TestRegistryOverwrite(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockProvider{name: "test"})
	reg.Register(&mockProvider{name: "test"}) // overwrite

	names := reg.Names()
	if len(names) != 1 {
		t.Fatalf("expected 1 name after overwrite, got %d", len(names))
	}
}
