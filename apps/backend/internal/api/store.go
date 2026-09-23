package api

import (
	"context"

	"backend/internal/models"
)

type LinkStore interface {
	CreateLink(ctx context.Context, slug, targetURL string) (*models.Link, error)
	GetLinkBySlug(ctx context.Context, slug string) (*models.Link, error)
	CountClicks(ctx context.Context, slug string) (int64, error)
	SlugExists(ctx context.Context, slug string) (bool, error)
}

type CacheStore interface {
	CacheTarget(ctx context.Context, slug, targetURL string) error
	GetCachedTarget(ctx context.Context, slug string) (string, error)
}
