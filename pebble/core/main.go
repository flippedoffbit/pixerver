package pebblecore

import (
	"context"
	"encoding/hex"
	"time"

	"pixerver/logger"

	"github.com/redis/go-redis/v9"
)

// Open creates a Redis client. The path parameter is ignored for Redis;
// configuration is read from environment variables or defaults.
func Open(path string) (*redis.Client, error) {
	// Default to localhost:6379; allow overrides through env in future.
	opt := &redis.Options{
		Addr:        "localhost:6379",
		DialTimeout: 5 * time.Second,
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		logger.Errorf("failed to open redis client: %v", err)
		return nil, err
	}
	logger.Infof("connected to redis %s", opt.Addr)
	return client, nil
}

// Close closes the Redis client.
func Close(db *redis.Client) error {
	if db == nil {
		return nil
	}
	if err := db.Close(); err != nil {
		logger.Errorf("error closing redis client: %v", err)
		return err
	}
	logger.Info("redis client closed")
	return nil
}

// AddEntry sets a key/value pair in Redis. The provided key is used as-is
// (converted to a string via hex encoding) to avoid binary issues.
func AddEntry(db *redis.Client, key, value []byte) error {
	if db == nil {
		return redis.ErrClosed
	}
	k := hex.EncodeToString(key)
	ctx := context.Background()
	if err := db.Set(ctx, k, value, 0).Err(); err != nil {
		logger.Errorf("redis set failed: %v", err)
		return err
	}
	logger.Debugf("set key=%s", k)
	return nil
}

// GetEntry retrieves a value for the provided key.
func GetEntry(db *redis.Client, key []byte) ([]byte, error) {
	if db == nil {
		return nil, redis.ErrClosed
	}
	k := hex.EncodeToString(key)
	ctx := context.Background()
	v, err := db.Get(ctx, k).Bytes()
	if err != nil {
		logger.Debugf("redis get key=%s: %v", k, err)
		return nil, err
	}
	logger.Debugf("got key=%s len=%d", k, len(v))
	return append([]byte(nil), v...), nil
}

// DelEntry deletes a key from Redis.
func DelEntry(db *redis.Client, key []byte) error {
	if db == nil {
		return redis.ErrClosed
	}
	k := hex.EncodeToString(key)
	ctx := context.Background()
	if err := db.Del(ctx, k).Err(); err != nil {
		logger.Errorf("redis del key=%s: %v", k, err)
		return err
	}
	logger.Debugf("deleted key=%s", k)
	return nil
}
