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
