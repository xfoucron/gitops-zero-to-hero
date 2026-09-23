package api

import (
	"backend/internal/config"
	"backend/internal/store"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func NewRouter(cfg *config.Config, pg *store.PostgresStore, rd *store.RedisStore) http.Handler {
	h := &Handler{cfg: cfg}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Second))

	r.Get("/healthz", h.Health)
	r.Route("/api/links", func(r chi.Router) {
		r.Post("/", h.CreateLink)
		r.Get("/{slug}/stats", h.GetStats)
	})

	r.Get("/{slug}", h.Redirect)

	return r
}
