package redisclient

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	// Check server version to ensure required stream commands are available.
	if info, err := client.Info(ctx, "server").Result(); err == nil {
		// look for a line like: redis_version:6.2.1
		for _, line := range strings.Split(info, "\n") {
			if strings.HasPrefix(line, "redis_version:") {
				ver := strings.TrimPrefix(line, "redis_version:")
				ver = strings.TrimSpace(ver)
				if maj, min, _ := parseSemver(ver); maj < 6 || (maj == 6 && min < 2) {
					return nil, fmt.Errorf("redis server version %s is too old: require >= 6.2.0", ver)
				}
				break
			}
		}
	} else {
		logger.Warnf("redisclient: failed to read INFO: %v", err)
	}

	logger.Infof("redisclient: connected addr=%s db=%d", addr, db)
	return client, nil
}

// parseSemver parses a version string like 6.2.1 and returns major, minor, patch.
func parseSemver(v string) (int, int, int) {
	parts := strings.Split(v, ".")
	maj, min, pat := 0, 0, 0
	if len(parts) > 0 {
		if n, err := strconv.Atoi(parts[0]); err == nil {
			maj = n
		}
	}
	if len(parts) > 1 {
		if n, err := strconv.Atoi(parts[1]); err == nil {
			min = n
		}
	}
	if len(parts) > 2 {
		// patch may have extra labels like 6.2.1-rc1; strip non-digits
		patchStr := parts[2]
		// keep only leading digits
		for i, r := range patchStr {
			if r < '0' || r > '9' {
				patchStr = patchStr[:i]
				break
			}
		}
		if n, err := strconv.Atoi(patchStr); err == nil {
			pat = n
		}
	}
	return maj, min, pat
}
