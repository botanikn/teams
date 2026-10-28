package security

import (
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTIssuerRoundTripAndRejectsOtherAlgorithms(t *testing.T) {
	issuer := NewJWTIssuer("this-secret-is-at-least-thirty-two-characters-long", time.Minute)
	raw, err := issuer.Issue(42)
	if err != nil {
		t.Fatal(err)
	}
	userID, err := issuer.Parse(raw)
	if err != nil || userID != 42 {
		t.Fatalf("Parse() = (%d, %v), want (42, nil)", userID, err)
	}

	forged := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.RegisteredClaims{
		Subject: "42", Issuer: "task-api",
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
	})
	forgedRaw, err := forged.SignedString([]byte("this-secret-is-at-least-thirty-two-characters-long"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := issuer.Parse(forgedRaw); err == nil {
		t.Fatal("Parse() accepted a token signed with HS512")
	}
}

func TestJWTIssuerRejectsExpiredToken(t *testing.T) {
	const secret = "this-secret-is-at-least-thirty-two-characters-long"
	issuer := NewJWTIssuer(secret, time.Minute)
	now := time.Now()
	expired := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   "42",
		Issuer:    "task-api",
		IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
		ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
	})
	raw, err := expired.SignedString([]byte(secret))
	if err != nil {
		t.Fatal(err)
	}

	if _, err := issuer.Parse(raw); err == nil {
		t.Fatal("Parse() accepted an expired token")
	}
}
