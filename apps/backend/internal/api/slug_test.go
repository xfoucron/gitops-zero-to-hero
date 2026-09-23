package api

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestRandomSlug(t *testing.T) {
	slug, err := randomSlug(shortSlugLength)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(slug) != shortSlugLength {
		t.Fatalf("expected length %d, got %d (%q)", shortSlugLength, len(slug), slug)
	}

	for _, c := range slug {
		if !strings.ContainsRune(shortSlugAlphabet, c) {
			t.Fatalf("slug %q contains character %q outside alphabet", slug, c)
		}
	}
}

func TestGenerateUniqueSlug_FirstTrySucceeds(t *testing.T) {
	pg := &mockLinkStore{
		slugExistsFn: func(ctx context.Context, slug string) (bool, error) {
			return false, nil
		},
	}
	h := &Handler{pg: pg}

	slug, err := h.generateUniqueSlug(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(slug) != shortSlugLength {
		t.Fatalf("expected length %d, got %d", shortSlugLength, len(slug))
	}
	if len(pg.slugExistsCalls) != 1 {
		t.Fatalf("expected SlugExists to be called once, got %d", len(pg.slugExistsCalls))
	}
}

func TestGenerateUniqueSlug_RetriesOnCollision(t *testing.T) {
	calls := 0
	pg := &mockLinkStore{
		slugExistsFn: func(ctx context.Context, slug string) (bool, error) {
			calls++
			return calls < 3, nil // collides twice, then free
		},
	}
	h := &Handler{pg: pg}

	_, err := h.generateUniqueSlug(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestGenerateUniqueSlug_ExhaustsRetries(t *testing.T) {
	pg := &mockLinkStore{
		slugExistsFn: func(ctx context.Context, slug string) (bool, error) {
			return true, nil // always collides
		},
	}
	h := &Handler{pg: pg}

	_, err := h.generateUniqueSlug(context.Background())
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if len(pg.slugExistsCalls) != maxGenerateTries {
		t.Fatalf("expected %d attempts, got %d", maxGenerateTries, len(pg.slugExistsCalls))
	}
}

func TestGenerateUniqueSlug_PropagatesStoreError(t *testing.T) {
	wantErr := errors.New("db unreachable")
	pg := &mockLinkStore{
		slugExistsFn: func(ctx context.Context, slug string) (bool, error) {
			return false, wantErr
		},
	}
	h := &Handler{pg: pg}

	_, err := h.generateUniqueSlug(context.Background())
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped store error, got %v", err)
	}
}
