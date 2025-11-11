package redisclient

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"pixerver/logger"

	"github.com/redis/go-redis/v9"
)

// NewClient creates a redis client using environment variables:
// REDIS_ADDR (default: localhost:6379), REDIS_PASSWORD (default: empty), REDIS_DB (default: 0).
func NewClient() (*redis.Client, error) {
	addr := os.Getenv("REDIS_ADDR")
	if addr == "" {
		addr = "localhost:6379"
	}
	pass := os.Getenv("REDIS_PASSWORD")
	db := 0
	if s := os.Getenv("REDIS_DB"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			db = v
		}
	}
	opt := &redis.Options{
		Addr:        addr,
		Password:    pass,
		DB:          db,
		DialTimeout: 5 * time.Second,
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Errorf("redisclient: ping failed: %v", err)
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}
	logger.Infof("redisclient: connected addr=%s db=%d", addr, db)
	return client, nil
}
