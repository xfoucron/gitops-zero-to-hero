package api

import (
	"backend/internal/config"
	"backend/internal/store"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

func NewRouter(cfg *config.Config, pg *store.PostgresStore, rd *store.RedisStore) http.Handler {
	h := &Handler{cfg: cfg, pg: pg, rs: rd}

	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(skipLoggingFor("/healthz", middleware.Logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(10 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: cfg.CORSAllowedOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type"},
		MaxAge:         300,
	}))

	r.Get("/healthz", h.Health)

	r.Route("/api/links", func(r chi.Router) {
		r.With(httprate.LimitByIP(cfg.CreateLinkRateLimit, cfg.CreateLinkRateLimitWindow)).
			Method(http.MethodPost, "/", traced("CreateLink", h.CreateLink))
		r.Method(http.MethodGet, "/{slug}/stats", traced("GetStats", h.GetStats))
	})

	r.Method(http.MethodGet, "/{slug}", traced("Redirect", h.Redirect))

	return r
}

func traced(operation string, handler http.HandlerFunc) http.Handler {
	return otelhttp.NewHandler(handler, operation)
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
