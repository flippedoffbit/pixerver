package store

import (
	"context"
	"encoding/hex"
	"fmt"

	"pixerver/internal/redisclient"
	"pixerver/logger"

	"github.com/redis/go-redis/v9"
)

// Store is a small Redis-backed key/value store which prefixes keys and
// maintains a simple index set for listing keys.
type Store struct {
	client *redis.Client
	prefix string // prefix applied to every stored key
	idxKey string // key used to store index set of member hex keys
}

// New creates a Store that will namespace keys with the provided prefix.
// Example: prefix="tasks:" will store values with keys like "tasks:<hex>".
func New(prefix string) (*Store, error) {
	// Create a redis client using environment-aware helper. This keeps
	// configuration in one place (REDIS_ADDR, REDIS_PASSWORD, REDIS_DB).
	client, err := redisclient.NewClient()
	if err != nil {
		return nil, err
	}
	s := &Store{client: client, prefix: prefix}
	s.idxKey = prefix + "index"
	logger.Infof("store: connected to redis, prefix=%s", prefix)
	return s, nil
}

// Close closes the underlying Redis client.
func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

// Set stores a value by binary key. The final redis key is prefix + hex(key).
func (s *Store) Set(key, value []byte) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("store: client not initialized")
	}
	hexk := hex.EncodeToString(key)
	redisKey := s.prefix + hexk
	ctx := context.Background()
	if err := s.client.Set(ctx, redisKey, value, 0).Err(); err != nil {
		return err
	}
	if err := s.client.SAdd(ctx, s.idxKey, hexk).Err(); err != nil {
		return err
	}
	return nil
}

// Get retrieves a previously stored value.
func (s *Store) Get(key []byte) ([]byte, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("store: client not initialized")
	}
	hexk := hex.EncodeToString(key)
	redisKey := s.prefix + hexk
	ctx := context.Background()
	b, err := s.client.Get(ctx, redisKey).Bytes()
	if err != nil {
		return nil, err
	}
	return append([]byte(nil), b...), nil
}

// Del deletes the key and removes it from the index.
func (s *Store) Del(key []byte) error {
	if s == nil || s.client == nil {
		return fmt.Errorf("store: client not initialized")
	}
	hexk := hex.EncodeToString(key)
	redisKey := s.prefix + hexk
	ctx := context.Background()
	if err := s.client.Del(ctx, redisKey).Err(); err != nil {
		return err
	}
	if err := s.client.SRem(ctx, s.idxKey, hexk).Err(); err != nil {
		return err
	}
	return nil
}

// KV is a convenience type returned by List.
type KV struct {
	Key   []byte
	Value []byte
}

// List returns all key/value pairs under this Store's prefix.
func (s *Store) List() ([]KV, error) {
	if s == nil || s.client == nil {
		return nil, fmt.Errorf("store: client not initialized")
	}
	ctx := context.Background()
	members, err := s.client.SMembers(ctx, s.idxKey).Result()
	if err != nil {
		return nil, err
	}
	var out []KV
	for _, m := range members {
		k, err := hex.DecodeString(m)
		if err != nil {
			continue
		}
		v, err := s.Get(k)
		if err != nil {
			continue
		}
		out = append(out, KV{Key: append([]byte(nil), k...), Value: append([]byte(nil), v...)})
	}
	return out, nil
}
