package debrid

import (
	"context"
	"errors"
	"testing"
)

type fakeProvider struct {
	name   string
	result ResolvedFile
	err    error
}

func (f fakeProvider) Name() string { return f.name }
func (f fakeProvider) Resolve(context.Context, string, *int) (ResolvedFile, error) {
	return f.result, f.err
}

func TestManagerResolveFallsThroughToNextProvider(t *testing.T) {
	m := NewManager(
		fakeProvider{name: "A", err: ErrNotCached},
		fakeProvider{name: "B", result: ResolvedFile{URL: "https://b.example/movie.mkv"}},
	)

	got, provider, err := m.Resolve(context.Background(), "hash", nil)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if provider != "B" {
		t.Errorf("provider = %q, want B (first should have failed over)", provider)
	}
	if got.URL != "https://b.example/movie.mkv" {
		t.Errorf("URL = %q, want B's URL", got.URL)
	}
}

func TestManagerResolveAllFail(t *testing.T) {
	m := NewManager(
		fakeProvider{name: "A", err: ErrNotCached},
		fakeProvider{name: "B", err: errors.New("boom")},
	)

	_, _, err := m.Resolve(context.Background(), "hash", nil)
	if err == nil {
		t.Fatal("Resolve() should fail when every provider fails")
	}
}

func TestManagerResolveNoProviders(t *testing.T) {
	m := NewManager()
	if !m.Empty() {
		t.Error("Empty() = false, want true for a manager with no providers")
	}
	_, _, err := m.Resolve(context.Background(), "hash", nil)
	if err == nil {
		t.Fatal("Resolve() with no providers configured should return an error")
	}
}
