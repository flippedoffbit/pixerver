package handlers

import (
	"errors"
	"net/http"
	"strings"
)

func extractBearerToken(r *http.Request) (string, error) {
	const prefix = "Bearer "
	authz := r.Header.Get("Authorization")
	if authz == "" {
		return "", errors.New("missing Authorization header")
	}
	if !strings.HasPrefix(authz, prefix) {
		return "", errors.New("invalid Authorization header")
	}
	token := strings.TrimSpace(authz[len(prefix):])
	if token == "" {
		return "", errors.New("empty bearer token")
	}
	return token, nil
}

// Note: validators should be provided to handlers via Server struct
// (dependency injection). The helper below only parses a Bearer header
// and is intentionally stateless so it can be tested independently.
