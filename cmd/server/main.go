package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"adp/internal/api"
)

func main() {
	cfg := api.Config{
		Addr:              envOrDefault("ADP_SERVER_ADDR", ":8080"),
		AdminUsername:     envOrDefault("ADP_ADMIN_USERNAME", "admin"),
		AdminPassword:     envOrDefault("ADP_ADMIN_PASSWORD", "admin123"),
		AuthSecret:        envOrDefault("ADP_AUTH_SECRET", "adp-dev-secret"),
		WorkerSharedToken: envOrDefault("ADP_WORKER_SHARED_TOKEN", "adp-worker-secret"),
		LLMBaseURL:        os.Getenv("ADP_LLM_BASE_URL"),
		LLMAPIKey:         os.Getenv("ADP_LLM_API_KEY"),
		LLMModel:          envOrDefault("ADP_LLM_MODEL", "gpt-4"),
	}

	server := api.NewServer(cfg)

	go func() {
		log.Printf("ADP server listening on %s", cfg.Addr)
		if err := server.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server start failed: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("server shutdown failed: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}
