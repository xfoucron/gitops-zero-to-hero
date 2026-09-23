package api

import (
	"backend/internal/config"
	"backend/internal/store"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func NewRouter(cfg *config.Config, pg *store.PostgresStore, rd *store.RedisStore) http.Handler {
	h := &Handler{cfg: cfg, pg: pg, rs: rd}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(skipLoggingFor("/healthz", middleware.Logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Second))

	r.Get("/healthz", h.Health)
	r.Route("/api/links", func(r chi.Router) {
		r.Post("/", h.CreateLink)
		r.Get("/{slug}/stats", h.GetStats)
	})

	r.Get("/{slug}", h.Redirect)

	return otelhttp.NewHandler(r, "http.server",
		otelhttp.WithFilter(func(r *http.Request) bool {
			return r.URL.Path != "/healthz"
		}),
	)
}

func skipLoggingFor(path string, logging func(http.Handler) http.Handler) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		logged := logging(next)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == path {
				next.ServeHTTP(w, r)
				return
			}
			logged.ServeHTTP(w, r)
		})
	}
}
