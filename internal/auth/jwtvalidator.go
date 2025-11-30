package auth

import (
	"errors"
	"fmt"
	"os"

	"github.com/golang-jwt/jwt/v5"
)

// Config describes how to validate JWT tokens.
type Config struct {
	Secret   string
	Audience string
	Issuer   string
}

// Validator validates JWT bearer tokens against a configured secret and optional
// issuer/audience.
type Validator struct {
	key      []byte
	audience string
	issuer   string
}

// NewValidator constructs a Validator from the provided configuration.
func NewValidator(cfg Config) (*Validator, error) {
	if cfg.Secret == "" {
		return nil, errors.New("jwt secret is required")
	}
	return &Validator{
		key:      []byte(cfg.Secret),
		audience: cfg.Audience,
		issuer:   cfg.Issuer,
	}, nil
}

// NewValidatorFromEnv builds a validator from environment variables.
func NewValidatorFromEnv(secretVar, audienceVar, issuerVar string) (*Validator, error) {
	secret := os.Getenv(secretVar)
	if secret == "" {
		return nil, fmt.Errorf("env %s is not set", secretVar)
	}
	cfg := Config{
		Secret:   secret,
		Audience: os.Getenv(audienceVar),
		Issuer:   os.Getenv(issuerVar),
	}
	return NewValidator(cfg)
}

// Validate parses and validates a JWT string.
func (v *Validator) Validate(token string) (*jwt.RegisteredClaims, error) {
	if token == "" {
		return nil, errors.New("empty token")
	}
	claims := &jwt.RegisteredClaims{}
	opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Name,
			jwt.SigningMethodHS384.Name,
			jwt.SigningMethodHS512.Name,
		}),
	}
	if v.audience != "" {
		opts = append(opts, jwt.WithAudience(v.audience))
	}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}

	parsed, err := jwt.ParseWithClaims(token, claims, func(tok *jwt.Token) (interface{}, error) {
		if _, ok := tok.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %T", tok.Method)
		}
		return v.key, nil
	}, opts...)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, errors.New("invalid token")
	}
	return claims, nil
}
