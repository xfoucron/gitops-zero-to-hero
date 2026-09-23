package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"backend/internal/config"
	"backend/internal/models"
	"backend/internal/store"
)

func newTestHandler(pg *mockLinkStore, rs *mockCacheStore) *Handler {
	return &Handler{
		cfg: &config.Config{ApplicationURL: "http://short.test"},
		pg:  pg,
		rs:  rs,
	}
}

func withSlug(r *http.Request, slug string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", slug)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func TestCreateLink_MissingTargetURL(t *testing.T) {
	h := newTestHandler(&mockLinkStore{}, &mockCacheStore{})

	req := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()

	h.CreateLink(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateLink_InvalidJSON(t *testing.T) {
	h := newTestHandler(&mockLinkStore{}, &mockCacheStore{})

	req := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(`not json`))
	w := httptest.NewRecorder()

	h.CreateLink(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestCreateLink_CustomSlugAlreadyTaken(t *testing.T) {
	pg := &mockLinkStore{
		slugExistsFn: func(ctx context.Context, slug string) (bool, error) { return true, nil },
	}
	h := newTestHandler(pg, &mockCacheStore{})

	body := `{"slug":"taken","target_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.CreateLink(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", w.Code)
	}
}

func TestCreateLink_CustomSlugSuccess(t *testing.T) {
	pg := &mockLinkStore{
		slugExistsFn: func(ctx context.Context, slug string) (bool, error) { return false, nil },
		createLinkFn: func(ctx context.Context, slug, targetURL string) (*models.Link, error) {
			return &models.Link{ID: 1, Slug: slug, TargetURL: targetURL, CreatedAt: time.Now()}, nil
		},
	}
	rs := &mockCacheStore{
		cacheTargetFn: func(ctx context.Context, slug, targetURL string) error { return nil },
	}
	h := newTestHandler(pg, rs)

	body := `{"slug":"mycustom","target_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.CreateLink(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
	}

	var resp map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response body: %v", err)
	}
	if resp["slug"] != "mycustom" {
		t.Fatalf("expected slug 'mycustom', got %q", resp["slug"])
	}
	if resp["short_url"] != "http://short.test/mycustom" {
		t.Fatalf("unexpected short_url: %q", resp["short_url"])
	}
	if len(rs.cachedCalls) != 1 {
		t.Fatalf("expected cache to be primed once, got %d", len(rs.cachedCalls))
	}
}

func TestCreateLink_GeneratesSlugWhenEmpty(t *testing.T) {
	pg := &mockLinkStore{
		slugExistsFn: func(ctx context.Context, slug string) (bool, error) { return false, nil },
		createLinkFn: func(ctx context.Context, slug, targetURL string) (*models.Link, error) {
			return &models.Link{ID: 1, Slug: slug, TargetURL: targetURL, CreatedAt: time.Now()}, nil
		},
	}
	rs := &mockCacheStore{cacheTargetFn: func(ctx context.Context, slug, targetURL string) error { return nil }}
	h := newTestHandler(pg, rs)

	body := `{"target_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.CreateLink(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestCreateLink_StoreErrorReturns500(t *testing.T) {
	pg := &mockLinkStore{
		slugExistsFn: func(ctx context.Context, slug string) (bool, error) { return false, nil },
		createLinkFn: func(ctx context.Context, slug, targetURL string) (*models.Link, error) {
			return nil, errors.New("insert failed")
		},
	}
	h := newTestHandler(pg, &mockCacheStore{})

	body := `{"slug":"x","target_url":"https://example.com"}`
	req := httptest.NewRequest(http.MethodPost, "/api/links", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	h.CreateLink(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}

func TestRedirect_CacheHit(t *testing.T) {
	pg := &mockLinkStore{
		getLinkBySlug: func(ctx context.Context, slug string) (*models.Link, error) {
			t.Fatal("postgres should not be queried on a cache hit")
			return nil, nil
		},
	}
	rs := &mockCacheStore{
		getCachedTargetFn: func(ctx context.Context, slug string) (string, error) {
			return "https://cached.example.com", nil
		},
	}
	h := newTestHandler(pg, rs)

	req := withSlug(httptest.NewRequest(http.MethodGet, "/abc123", nil), "abc123")
	w := httptest.NewRecorder()

	h.Redirect(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://cached.example.com" {
		t.Fatalf("unexpected redirect location: %q", loc)
	}
}

func TestRedirect_CacheMissFallsBackToPostgres(t *testing.T) {
	pg := &mockLinkStore{
		getLinkBySlug: func(ctx context.Context, slug string) (*models.Link, error) {
			return &models.Link{Slug: slug, TargetURL: "https://pg.example.com"}, nil
		},
	}
	rs := &mockCacheStore{
		getCachedTargetFn: func(ctx context.Context, slug string) (string, error) { return "", nil },
		cacheTargetFn:     func(ctx context.Context, slug, targetURL string) error { return nil },
	}
	h := newTestHandler(pg, rs)

	req := withSlug(httptest.NewRequest(http.MethodGet, "/abc123", nil), "abc123")
	w := httptest.NewRecorder()

	h.Redirect(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected 302, got %d", w.Code)
	}
	if loc := w.Header().Get("Location"); loc != "https://pg.example.com" {
		t.Fatalf("unexpected redirect location: %q", loc)
	}
	if len(rs.cachedCalls) != 1 {
		t.Fatalf("expected cache to be re-primed once, got %d", len(rs.cachedCalls))
	}
}

func TestRedirect_RedisErrorFallsBackToPostgres(t *testing.T) {
	pg := &mockLinkStore{
		getLinkBySlug: func(ctx context.Context, slug string) (*models.Link, error) {
			return &models.Link{Slug: slug, TargetURL: "https://pg.example.com"}, nil
		},
	}
	rs := &mockCacheStore{
		getCachedTargetFn: func(ctx context.Context, slug string) (string, error) {
			return "", errors.New("redis is down")
		},
		cacheTargetFn: func(ctx context.Context, slug, targetURL string) error { return nil },
	}
	h := newTestHandler(pg, rs)

	req := withSlug(httptest.NewRequest(http.MethodGet, "/abc123", nil), "abc123")
	w := httptest.NewRecorder()

	h.Redirect(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("expected redirect to succeed despite redis error, got %d (%s)", w.Code, w.Body.String())
	}
}

func TestRedirect_NotFound(t *testing.T) {
	pg := &mockLinkStore{
		getLinkBySlug: func(ctx context.Context, slug string) (*models.Link, error) {
			return nil, store.ErrNotFound
		},
	}
	rs := &mockCacheStore{
		getCachedTargetFn: func(ctx context.Context, slug string) (string, error) { return "", nil },
	}
	h := newTestHandler(pg, rs)

	req := withSlug(httptest.NewRequest(http.MethodGet, "/missing", nil), "missing")
	w := httptest.NewRecorder()

	h.Redirect(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}

func TestGetStats_Success(t *testing.T) {
	created := time.Now()
	pg := &mockLinkStore{
		getLinkBySlug: func(ctx context.Context, slug string) (*models.Link, error) {
			return &models.Link{Slug: slug, TargetURL: "https://example.com", CreatedAt: created}, nil
		},
		countClicksFn: func(ctx context.Context, slug string) (int64, error) { return 42, nil },
	}
	h := newTestHandler(pg, &mockCacheStore{})

	req := withSlug(httptest.NewRequest(http.MethodGet, "/api/links/abc/stats", nil), "abc")
	w := httptest.NewRecorder()

	h.GetStats(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d (%s)", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid response body: %v", err)
	}
	if resp["clicks"].(float64) != 42 {
		t.Fatalf("expected 42 clicks, got %v", resp["clicks"])
	}
}

func TestGetStats_NotFound(t *testing.T) {
	pg := &mockLinkStore{
		getLinkBySlug: func(ctx context.Context, slug string) (*models.Link, error) {
			return nil, store.ErrNotFound
		},
	}
	h := newTestHandler(pg, &mockCacheStore{})

	req := withSlug(httptest.NewRequest(http.MethodGet, "/api/links/missing/stats", nil), "missing")
	w := httptest.NewRecorder()

	h.GetStats(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
