package main

import (
	"backend/internal/api"
	"backend/internal/config"
	"backend/internal/store"
	"backend/internal/telemetry"
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.Printf("starting")

	otelShutdown, err := telemetry.Setup(context.Background(), "url-shortener")
	if err != nil {
		log.Fatalf("otel setup failed: %v", err)
	}
	defer func() {
		if err := otelShutdown(context.Background()); err != nil {
			log.Printf("otel shutdown error: %v", err)
		}
	}()

	logger := telemetry.Logger()

	cfg := config.Load()

	pgStore, err := store.NewPostgresStore(cfg.PostgresURL)

	if err != nil {
		log.Fatalf("postgres connection failed: %v", err)
	}
	defer pgStore.Close()

	if err := store.RunMigrationsWithDB(pgStore.DB(), "migrations"); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	redisStore, err := store.NewRedisStore(cfg.RedisURL)

	if err != nil {
		log.Fatalf("redis connection failed: %v", err)
	}
	defer redisStore.Close()

	router := api.NewRouter(cfg, pgStore, redisStore)

	srv := &http.Server{
		Addr:         ":" + cfg.ApplicationPort,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		logger.Info("backend listening", "port", cfg.ApplicationPort)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	logger.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}

	logger.Info("goodbye")
}
