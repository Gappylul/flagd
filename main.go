package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/gappylul/flagd/internal/api"
	"github.com/gappylul/flagd/internal/store"
)

func main() {
	redisURL := envOr("REDIS_URL", "redis://localhost:6379")
	addr := envOr("FLAGD_ADDR", ":8080")
	adminKey := os.Getenv("ADMIN_KEY")

	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		log.Fatal("invalid REDIS_URL:", err)
	}

	rdb := redis.NewClient(opt)
	if err := waitForRedis(rdb); err != nil {
		log.Fatal("redis never became ready:", err)
	}
	defer rdb.Close()

	s := store.New(rdb)
	h := api.New(s, adminKey)

	slog.Info("flagd started", "addr", addr, "redis", redisURL, "auth", adminKey != "")
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatal(err)
	}
}

func waitForRedis(rdb *redis.Client) error {
	ctx := context.Background()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		if err := rdb.Ping(ctx).Err(); err == nil {
			return nil
		}
		slog.Info("waiting for redis...")
		time.Sleep(time.Second)
	}
	return rdb.Ping(ctx).Err()
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
