package auth

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

const (
	testIssuer   = "https://cognito-idp.us-east-1.amazonaws.com/us-east-1_test"
	testAudience = "test-client-id"
	testKid      = "test-key-1"
)

// signToken builds a real RS256 JWT signed with the given key.
func signToken(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	header, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	payload, _ := json.Marshal(claims)
	signingInput := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(payload)
	hashed := sha256.Sum256([]byte(signingInput))
	signature, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed[:])
	if err != nil {
		t.Fatalf("signing token: %v", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(signature)
}

// jwksServer serves the public part of the key in JWKS format.
func jwksServer(t *testing.T, key *rsa.PrivateKey, kid string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jwks := map[string]any{
			"keys": []map[string]string{{
				"kid": kid,
				"kty": "RSA",
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(key.PublicKey.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.PublicKey.E)).Bytes()),
			}},
		}
		_ = json.NewEncoder(w).Encode(jwks)
	}))
}

func validIDClaims() map[string]any {
	return map[string]any{
		"sub":       "user-sub-123",
		"aud":       testAudience,
		"iss":       testIssuer,
		"token_use": "id",
		"exp":       time.Now().Add(time.Hour).Unix(),
		"email":     "aura@example.com",
	}
}

func newTestVerifier(t *testing.T) (*Verifier, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	server := jwksServer(t, key, testKid)
	t.Cleanup(server.Close)
	return NewVerifier(testIssuer, testAudience, server.URL), key
}

func TestVerifyValidIDToken(t *testing.T) {
	verifier, key := newTestVerifier(t)
	token := signToken(t, key, testKid, validIDClaims())

	claims, err := verifier.Verify(context.Background(), token)
	if err != nil {
		t.Fatalf("expected valid token, got error: %v", err)
	}
	if claims["sub"] != "user-sub-123" {
		t.Fatalf("expected sub claim, got %q", claims["sub"])
	}
	if claims["email"] != "aura@example.com" {
		t.Fatalf("expected email claim, got %q", claims["email"])
	}
}

func TestVerifyValidAccessToken(t *testing.T) {
	verifier, key := newTestVerifier(t)
	claims := map[string]any{
		"sub":       "user-sub-123",
		"client_id": testAudience,
		"iss":       testIssuer,
		"token_use": "access",
		"exp":       time.Now().Add(time.Hour).Unix(),
	}
	if _, err := verifier.Verify(context.Background(), signToken(t, key, testKid, claims)); err != nil {
		t.Fatalf("expected valid access token, got error: %v", err)
	}
}

func TestVerifyRejectsInvalidTokens(t *testing.T) {
	verifier, key := newTestVerifier(t)
	otherKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	expired := validIDClaims()
	expired["exp"] = time.Now().Add(-time.Minute).Unix()

	wrongIssuer := validIDClaims()
	wrongIssuer["iss"] = "https://evil.example.com"

	wrongAudience := validIDClaims()
	wrongAudience["aud"] = "another-client"

	wrongUse := validIDClaims()
	wrongUse["token_use"] = "refresh"

	cases := []struct {
		name  string
		token string
	}{
		{"expired", signToken(t, key, testKid, expired)},
		{"wrong issuer", signToken(t, key, testKid, wrongIssuer)},
		{"wrong audience", signToken(t, key, testKid, wrongAudience)},
		{"wrong token_use", signToken(t, key, testKid, wrongUse)},
		{"signed by another key", signToken(t, otherKey, testKid, validIDClaims())},
		{"unknown kid", signToken(t, key, "other-kid", validIDClaims())},
		{"malformed", "not-a-jwt"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := verifier.Verify(context.Background(), tc.token); err == nil {
				t.Fatal("expected verification error, got nil")
			}
		})
	}
}

func TestVerifyRejectsAlgNone(t *testing.T) {
	verifier, _ := newTestVerifier(t)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","kid":"test-key-1"}`))
	payload, _ := json.Marshal(validIDClaims())
	token := fmt.Sprintf("%s.%s.", header, base64.RawURLEncoding.EncodeToString(payload))

	if _, err := verifier.Verify(context.Background(), token); err == nil {
		t.Fatal("expected alg=none to be rejected")
	}
}

func TestNewVerifierFromEnvDisabledWithoutConfig(t *testing.T) {
	t.Setenv(cognitoIssuerEnv, "")
	t.Setenv(cognitoAudienceEnv, "")
	if v := NewVerifierFromEnv(); v != nil {
		t.Fatal("expected nil verifier without configuration")
	}
}

func TestNewVerifierFromEnvEnabled(t *testing.T) {
	t.Setenv(cognitoIssuerEnv, testIssuer+"/")
	t.Setenv(cognitoAudienceEnv, testAudience)
	v := NewVerifierFromEnv()
	if v == nil {
		t.Fatal("expected verifier with configuration present")
	}
	if v.jwksURL != testIssuer+"/.well-known/jwks.json" {
		t.Fatalf("unexpected JWKS URL: %s", v.jwksURL)
	}
}
