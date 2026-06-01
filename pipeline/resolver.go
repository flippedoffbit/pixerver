package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"pixerver/internal/redisclient"

	"github.com/redis/go-redis/v9"
)

// BackendResolver resolves backend config tokens such as "s3-primary" into
// concrete JSON or URL backend configs.
type BackendResolver interface {
	ResolveBackendConfig(ctx context.Context, token string) (string, bool, error)
}

// BackendResolverFunc adapts a function into a BackendResolver.
type BackendResolverFunc func(ctx context.Context, token string) (string, bool, error)

func (f BackendResolverFunc) ResolveBackendConfig(ctx context.Context, token string) (string, bool, error) {
	return f(ctx, token)
}

// RedisBackendResolver stores backend config tokens at prefix+token.
// Example: prefix="backend-configs:" token="s3-primary" reads
// Redis key "backend-configs:s3-primary".
type RedisBackendResolver struct {
	client *redis.Client
	prefix string
}

func NewRedisBackendResolver(prefix string) (*RedisBackendResolver, error) {
	client, err := redisclient.NewClient()
	if err != nil {
		return nil, err
	}
	if prefix == "" {
		prefix = "backend-configs:"
	}
	return &RedisBackendResolver{client: client, prefix: prefix}, nil
}

func (r *RedisBackendResolver) Close() error {
	if r == nil || r.client == nil {
		return nil
	}
	return r.client.Close()
}

func (r *RedisBackendResolver) ResolveBackendConfig(ctx context.Context, token string) (string, bool, error) {
	if r == nil || r.client == nil {
		return "", false, nil
	}
	token = strings.TrimSpace(token)
	if token == "" {
		return "", false, nil
	}
	value, err := r.client.Get(ctx, r.prefix+token).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("resolve backend token %q: %w", token, err)
	}
	return value, true, nil
}
