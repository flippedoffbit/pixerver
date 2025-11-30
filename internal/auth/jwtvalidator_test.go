package auth

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestValidatorValidate(t *testing.T) {
	secret := "unit-secret"
	v, err := NewValidator(Config{Secret: secret, Audience: "aud", Issuer: "iss"})
	if err != nil {
		t.Fatalf("NewValidator: %v", err)
	}

	claims := jwt.RegisteredClaims{
		Audience:  jwt.ClaimStrings{"aud"},
		Issuer:    "iss",
		Subject:   "example",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}

	if _, err := v.Validate(signed); err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}

	// wrong audience should fail
	badClaims := jwt.RegisteredClaims{
		Audience:  jwt.ClaimStrings{"other"},
		Issuer:    "iss",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute)),
	}
	bad := jwt.NewWithClaims(jwt.SigningMethodHS256, badClaims)
	badSigned, err := bad.SignedString([]byte(secret))
	if err != nil {
		t.Fatalf("SignedString: %v", err)
	}
	if _, err := v.Validate(badSigned); err == nil {
		t.Fatalf("expected audience validation failure")
	}
}

func TestValidatorMissingSecret(t *testing.T) {
	if _, err := NewValidator(Config{}); err == nil {
		t.Fatalf("expected error for missing secret")
	}
}
