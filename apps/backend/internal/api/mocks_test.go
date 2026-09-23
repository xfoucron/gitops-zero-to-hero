package api

import (
	"context"

	"backend/internal/models"
)

type mockLinkStore struct {
	createLinkFn    func(ctx context.Context, slug, targetURL string) (*models.Link, error)
	getLinkBySlug   func(ctx context.Context, slug string) (*models.Link, error)
	countClicksFn   func(ctx context.Context, slug string) (int64, error)
	slugExistsFn    func(ctx context.Context, slug string) (bool, error)
	pingFn          func(ctx context.Context) error
	slugExistsCalls []string
	createLinkCalls []string
}

func (m *mockLinkStore) CreateLink(ctx context.Context, slug, targetURL string) (*models.Link, error) {
	m.createLinkCalls = append(m.createLinkCalls, slug)
	return m.createLinkFn(ctx, slug, targetURL)
}

func (m *mockLinkStore) GetLinkBySlug(ctx context.Context, slug string) (*models.Link, error) {
	return m.getLinkBySlug(ctx, slug)
}

func (m *mockLinkStore) CountClicks(ctx context.Context, slug string) (int64, error) {
	return m.countClicksFn(ctx, slug)
}

func (m *mockLinkStore) SlugExists(ctx context.Context, slug string) (bool, error) {
	m.slugExistsCalls = append(m.slugExistsCalls, slug)
	return m.slugExistsFn(ctx, slug)
}

func (m *mockLinkStore) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}

type mockCacheStore struct {
	cacheTargetFn     func(ctx context.Context, slug, targetURL string) error
	getCachedTargetFn func(ctx context.Context, slug string) (string, error)
	pingFn            func(ctx context.Context) error
	cachedCalls       []string
}

func (m *mockCacheStore) CacheTarget(ctx context.Context, slug, targetURL string) error {
	m.cachedCalls = append(m.cachedCalls, slug)
	return m.cacheTargetFn(ctx, slug, targetURL)
}

func (m *mockCacheStore) GetCachedTarget(ctx context.Context, slug string) (string, error) {
	return m.getCachedTargetFn(ctx, slug)
}

func (m *mockCacheStore) Ping(ctx context.Context) error {
	if m.pingFn != nil {
		return m.pingFn(ctx)
	}
	return nil
}
