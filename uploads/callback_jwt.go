package uploads

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type CallbackSigner struct {
	Secret   string
	Issuer   string
	Audience string
	TTL      time.Duration
	Now      func() time.Time
}

func NewCallbackSignerFromEnv() (*CallbackSigner, error) {
	secret := os.Getenv("PIXERVER_CALLBACK_JWT_SECRET")
	if secret == "" {
		return nil, errors.New("env PIXERVER_CALLBACK_JWT_SECRET is not set")
	}
	issuer := os.Getenv("PIXERVER_CALLBACK_JWT_ISSUER")
	if issuer == "" {
		issuer = "pixerver"
	}
	ttl := 5 * time.Minute
	if raw := os.Getenv("PIXERVER_CALLBACK_JWT_TTL"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return nil, err
		}
		ttl = parsed
	}
	return &CallbackSigner{
		Secret:   secret,
		Issuer:   issuer,
		Audience: os.Getenv("PIXERVER_CALLBACK_JWT_AUDIENCE"),
		TTL:      ttl,
	}, nil
}

func (s *CallbackSigner) SignUpload(ctx context.Context, uploadID string) (string, error) {
	if s == nil || s.Secret == "" {
		return "", errors.New("callback jwt signer is not configured")
	}
	nowFunc := s.Now
	if nowFunc == nil {
		nowFunc = time.Now
	}
	now := nowFunc()
	claims := jwt.MapClaims{
		"sub":      uploadID,
		"iss":      s.Issuer,
		"iat":      now.Unix(),
		"exp":      now.Add(s.TTL).Unix(),
		"uploadId": uploadID,
	}
	if s.Audience != "" {
		claims["aud"] = s.Audience
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.Secret))
}
