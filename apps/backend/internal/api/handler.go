package api

import (
	"backend/internal/config"
	"backend/internal/store"
	"encoding/json"
	"net/http"
)

type Handler struct {
	cfg *config.Config
	pg  *store.PostgresStore
	rs  *store.RedisStore
}

func respondJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
