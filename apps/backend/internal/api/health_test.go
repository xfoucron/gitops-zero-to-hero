package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth_AllUp(t *testing.T) {
	pg := &mockLinkStore{pingFn: func(ctx context.Context) error { return nil }}
	rs := &mockCacheStore{pingFn: func(ctx context.Context) error { return nil }}
	h := newTestHandler(pg, rs)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestHealth_PostgresDown(t *testing.T) {
	pg := &mockLinkStore{pingFn: func(ctx context.Context) error { return errors.New("connection refused") }}
	rs := &mockCacheStore{pingFn: func(ctx context.Context) error { return nil }}
	h := newTestHandler(pg, rs)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestHealth_RedisDown(t *testing.T) {
	pg := &mockLinkStore{pingFn: func(ctx context.Context) error { return nil }}
	rs := &mockCacheStore{pingFn: func(ctx context.Context) error { return errors.New("connection refused") }}
	h := newTestHandler(pg, rs)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}
