package main

import (
	"log"
	"os"
	"time"

	"adp/internal/worker"
)

func main() {
	pollInterval := durationOrDefault("ADP_WORKER_POLL_INTERVAL", 5*time.Second)
	client := worker.NewClient(
		envOrDefault("ADP_SERVER_URL", "http://127.0.0.1:8080"),
		envOrDefault("ADP_WORKER_SHARED_TOKEN", "adp-worker-secret"),
		envOrDefault("ADP_WORKER_NAME", "worker-1"),
		envOrDefault("ADP_WORKER_TYPE", "shell"),
		pollInterval,
	)

	log.Printf("ADP worker starting: name=%s type=%s", envOrDefault("ADP_WORKER_NAME", "worker-1"), envOrDefault("ADP_WORKER_TYPE", "shell"))
	if err := client.Run(); err != nil {
		log.Fatalf("worker failed: %v", err)
	}
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func durationOrDefault(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
