package main

import (
	"backend/internal/api"
	"backend/internal/config"
	"backend/internal/store"
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

	cfg := config.Load()

	pgStore, err := store.NewPostgresStore(cfg.PostgresURL)

	if err != nil {
		log.Fatalf("postgres connection failed: %v", err)
	}
	defer pgStore.Close()

	redisStore, err := store.NewRedisStore(cfg.RedisURL)

	if err != nil {
		log.Fatalf("redis connection failed: %v", err)
	}
	defer redisStore.Close()

	if err := store.RunMigrations(cfg.PostgresURL, "migrations"); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	router := api.NewRouter(cfg, pgStore, redisStore)

	srv := &http.Server{
		Addr:         ":" + cfg.ApplicationPort,
		Handler:      router,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		log.Printf("backend listing on :%s", cfg.ApplicationPort)

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("graceful shutdown failed: %v", err)
	}

	log.Printf("goodbye")
}
