package api

import (
	"context"
	"net/http"
	"time"
)

const healthCheckTimeout = 2 * time.Second

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), healthCheckTimeout)
	defer cancel()

	if err := h.pg.Ping(ctx); err != nil {
		respondError(w, http.StatusServiceUnavailable, "postgres unavailable")
		return
	}

	if err := h.rs.Ping(ctx); err != nil {
		respondError(w, http.StatusServiceUnavailable, "redis unavailable")
		return
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
