package api

import (
	"backend/internal/models"
	"backend/internal/store"
	"backend/internal/telemetry"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"

	"github.com/go-chi/chi/v5"
)

var tracer = telemetry.Tracer("api")

const (
	maxGenerateTries = 5

	shortSlugLength   = 7
	shortSlugAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	var req models.CreateLinkRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.TargetURL == "" {
		respondError(w, http.StatusBadRequest, "target_url is required")
		return
	}

	ctx := r.Context()
	slug := req.Slug

	if slug == "" {
		generated, err := h.generateUniqueSlug(ctx)

		if err != nil {
			respondError(w, http.StatusInternalServerError, "could not generate slug")
			return
		}

		slug = generated
	} else {
		exists, err := h.pg.SlugExists(ctx, slug)

		if err != nil {
			respondError(w, http.StatusInternalServerError, "could not validate slug")
			return
		}

		if exists {
			respondError(w, http.StatusConflict, "slug already taken")
			return
		}
	}

	createCtx, createSpan := tracer.Start(ctx, "pg.CreateLink")
	link, err := h.pg.CreateLink(createCtx, slug, req.TargetURL)
	createSpan.End()

	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not create link")
		return
	}

	_ = h.rs.CacheTarget(ctx, link.Slug, link.TargetURL)

	respondJSON(w, http.StatusCreated, map[string]string{
		"short_url": h.cfg.ApplicationURL + "/" + link.Slug,

		"slug":       link.Slug,
		"target_url": link.TargetURL,
	})
}

func (h *Handler) Redirect(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	cacheCtx, cacheSpan := tracer.Start(ctx, "redis.GetCachedTarget")
	target, err := h.rs.GetCachedTarget(cacheCtx, slug)
	cacheSpan.End()
	if err != nil {
		target = ""
	}

	if target == "" {
		pgCtx, pgSpan := tracer.Start(ctx, "pg.GetLinkBySlug")
		link, err := h.pg.GetLinkBySlug(pgCtx, slug)
		pgSpan.End()

		if errors.Is(err, store.ErrNotFound) {
			respondError(w, http.StatusNotFound, "link not found")
			return
		}

		if err != nil {
			respondError(w, http.StatusInternalServerError, "lookup failed")
			return
		}

		target = link.TargetURL
		_ = h.rs.CacheTarget(ctx, slug, target)
	}

	http.Redirect(w, r, target, http.StatusFound)
}

func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	ctx := r.Context()

	pgCtx, pgSpan := tracer.Start(ctx, "pg.GetLinkBySlug")
	link, err := h.pg.GetLinkBySlug(pgCtx, slug)
	pgSpan.End()

	if errors.Is(err, store.ErrNotFound) {
		respondError(w, http.StatusNotFound, "link not found")
		return
	}

	if err != nil {
		respondError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	countCtx, countSpan := tracer.Start(ctx, "pg.CountClicks")
	clicks, err := h.pg.CountClicks(countCtx, slug)
	countSpan.End()

	if err != nil {
		respondError(w, http.StatusInternalServerError, "could not count clicks")
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"clicks":     clicks,
		"slug":       link.Slug,
		"target_url": link.TargetURL,
		"created_at": link.CreatedAt,
	})
}

func (h *Handler) generateUniqueSlug(ctx context.Context) (string, error) {
	for i := 0; i < maxGenerateTries; i++ {
		slug, err := randomSlug(shortSlugLength)

		if err != nil {
			return "", err
		}

		exists, err := h.pg.SlugExists(ctx, slug)

		if err != nil {
			return "", err
		}

		if !exists {
			return slug, nil
		}
	}

	return "", errors.New("could not generate a unique slug after several tries")
}

func randomSlug(length int) (string, error) {
	b := make([]byte, length)

	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(shortSlugAlphabet))))

		if err != nil {
			return "", err
		}

		b[i] = shortSlugAlphabet[n.Int64()]
	}

	return string(b), nil
}
