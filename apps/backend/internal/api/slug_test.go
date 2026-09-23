package api

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"backend/internal/models"
	"backend/internal/store"
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

func TestCreateLinkWithGeneratedSlug_FirstTrySucceeds(t *testing.T) {
	pg := &mockLinkStore{
		createLinkFn: func(ctx context.Context, slug, targetURL string) (*models.Link, error) {
			return &models.Link{Slug: slug, TargetURL: targetURL, CreatedAt: time.Now()}, nil
		},
	}
	h := &Handler{pg: pg}

	link, err := h.createLinkWithGeneratedSlug(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(link.Slug) != shortSlugLength {
		t.Fatalf("expected slug length %d, got %d", shortSlugLength, len(link.Slug))
	}
	if len(pg.createLinkCalls) != 1 {
		t.Fatalf("expected CreateLink to be called once, got %d", len(pg.createLinkCalls))
	}
}

func TestCreateLinkWithGeneratedSlug_RetriesOnCollision(t *testing.T) {
	calls := 0
	pg := &mockLinkStore{
		createLinkFn: func(ctx context.Context, slug, targetURL string) (*models.Link, error) {
			calls++
			if calls < 3 {
				return nil, store.ErrSlugTaken
			}
			return &models.Link{Slug: slug, TargetURL: targetURL}, nil
		},
	}
	h := &Handler{pg: pg}

	_, err := h.createLinkWithGeneratedSlug(context.Background(), "https://example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if calls != 3 {
		t.Fatalf("expected 3 attempts, got %d", calls)
	}
}

func TestCreateLinkWithGeneratedSlug_ExhaustsRetries(t *testing.T) {
	pg := &mockLinkStore{
		createLinkFn: func(ctx context.Context, slug, targetURL string) (*models.Link, error) {
			return nil, store.ErrSlugTaken
		},
	}
	h := &Handler{pg: pg}

	_, err := h.createLinkWithGeneratedSlug(context.Background(), "https://example.com")
	if err == nil {
		t.Fatal("expected an error after exhausting retries")
	}
	if len(pg.createLinkCalls) != maxGenerateTries {
		t.Fatalf("expected %d attempts, got %d", maxGenerateTries, len(pg.createLinkCalls))
	}
}

func TestCreateLinkWithGeneratedSlug_PropagatesOtherStoreErrors(t *testing.T) {
	wantErr := errors.New("db unreachable")
	pg := &mockLinkStore{
		createLinkFn: func(ctx context.Context, slug, targetURL string) (*models.Link, error) {
			return nil, wantErr
		},
	}
	h := &Handler{pg: pg}

	_, err := h.createLinkWithGeneratedSlug(context.Background(), "https://example.com")
	if !errors.Is(err, wantErr) {
		t.Fatalf("expected wrapped store error, got %v", err)
	}
	if len(pg.createLinkCalls) != 1 {
		t.Fatalf("expected to stop retrying on a non-conflict error, got %d attempts", len(pg.createLinkCalls))
	}
}

func TestValidateTargetURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid https", "https://example.com", false},
		{"valid http", "http://example.com/path", false},
		{"empty", "", true},
		{"missing scheme", "example.com", true},
		{"javascript scheme", "javascript:alert(1)", true},
		{"missing host", "https://", true},
		{"not a url", "://not a url", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTargetURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("expected an error for %q, got nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.url, err)
			}
		})
	}
}

func TestValidateSlug(t *testing.T) {
	cases := []struct {
		name    string
		slug    string
		wantErr bool
	}{
		{"simple", "my-slug_123", false},
		{"too long", strings.Repeat("a", 33), true},
		{"contains slash", "my/slug", true},
		{"contains space", "my slug", true},
		{"empty", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSlug(tc.slug)
			if tc.wantErr && err == nil {
				t.Fatalf("expected an error for %q, got nil", tc.slug)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("expected no error for %q, got %v", tc.slug, err)
			}
		})
	}
}
