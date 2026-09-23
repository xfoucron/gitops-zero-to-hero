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
	"net/url"
	"regexp"

	"github.com/go-chi/chi/v5"
)

var tracer = telemetry.Tracer("api")

const (
	maxGenerateTries = 5

	shortSlugLength   = 7
	shortSlugAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

	maxSlugLength = 32

	maxCreateLinkBodyBytes = 1 << 20
)

var slugPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

func (h *Handler) CreateLink(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxCreateLinkBodyBytes)

	var req models.CreateLinkRequest

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := validateTargetURL(req.TargetURL); err != nil {
		respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	ctx := r.Context()

	var link *models.Link
	var err error

	if req.Slug != "" {
		if err := validateSlug(req.Slug); err != nil {
			respondError(w, http.StatusBadRequest, err.Error())
			return
		}

		exists, existsErr := h.pg.SlugExists(ctx, req.Slug)
		if existsErr != nil {
			respondError(w, http.StatusInternalServerError, "could not validate slug")
			return
		}
		if exists {
			respondError(w, http.StatusConflict, "slug already taken")
			return
		}

		link, err = h.createLink(ctx, req.Slug, req.TargetURL)
		if errors.Is(err, store.ErrSlugTaken) {
			respondError(w, http.StatusConflict, "slug already taken")
			return
		}
	} else {
		link, err = h.createLinkWithGeneratedSlug(ctx, req.TargetURL)
	}

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

func (h *Handler) createLink(ctx context.Context, slug, targetURL string) (*models.Link, error) {
	spanCtx, span := tracer.Start(ctx, "pg.CreateLink")
	defer span.End()

	return h.pg.CreateLink(spanCtx, slug, targetURL)
}

func (h *Handler) createLinkWithGeneratedSlug(ctx context.Context, targetURL string) (*models.Link, error) {
	for i := 0; i < maxGenerateTries; i++ {
		slug, err := randomSlug(shortSlugLength)
		if err != nil {
			return nil, err
		}

		link, err := h.createLink(ctx, slug, targetURL)
		if err == nil {
			return link, nil
		}

		if errors.Is(err, store.ErrSlugTaken) {
			continue
		}

		return nil, err
	}

	return nil, errors.New("could not generate a unique slug after several tries")
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

func validateTargetURL(raw string) error {
	if raw == "" {
		return errors.New("target_url is required")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return errors.New("target_url is not a valid URL")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return errors.New("target_url must use http or https")
	}

	if u.Host == "" {
		return errors.New("target_url must include a host")
	}

	return nil
}

func validateSlug(slug string) error {
	if len(slug) > maxSlugLength {
		return errors.New("slug must be at most 32 characters")
	}

	if !slugPattern.MatchString(slug) {
		return errors.New("slug may only contain letters, digits, hyphens and underscores")
	}

	return nil
}
