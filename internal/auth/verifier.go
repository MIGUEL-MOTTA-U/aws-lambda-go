// Package auth validates Cognito-issued JWTs (RS256 via the pool's JWKS).
// It is the server-side guard for mutation routes: even if the frontend gate
// or the API Gateway authorizer is bypassed, the Lambda rejects unsigned or
// tampered tokens. Implemented with the standard library only.
package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

var ErrInvalidToken = errors.New("invalid token")

const (
	cognitoIssuerEnv   = "COGNITO_ISSUER"
	cognitoAudienceEnv = "COGNITO_AUDIENCE"
)

// Verifier validates RS256 JWTs against a JWKS endpoint, caching keys in
// memory (persists across invocations on a warm Lambda).
type Verifier struct {
	issuer   string
	audience string
	jwksURL  string
	client   *http.Client

	mu   sync.RWMutex
	keys map[string]*rsa.PublicKey
}

// NewVerifierFromEnv builds a Verifier from COGNITO_ISSUER and
// COGNITO_AUDIENCE. Returns nil when they are not configured (guard
// disabled: local development pre-Cognito).
func NewVerifierFromEnv() *Verifier {
	issuer := strings.TrimRight(strings.TrimSpace(os.Getenv(cognitoIssuerEnv)), "/")
	audience := strings.TrimSpace(os.Getenv(cognitoAudienceEnv))
	if issuer == "" || audience == "" {
		return nil
	}
	return NewVerifier(issuer, audience, issuer+"/.well-known/jwks.json")
}

func NewVerifier(issuer, audience, jwksURL string) *Verifier {
	return &Verifier{
		issuer:   strings.TrimRight(issuer, "/"),
		audience: audience,
		jwksURL:  jwksURL,
		client:   &http.Client{Timeout: 10 * time.Second},
		keys:     make(map[string]*rsa.PublicKey),
	}
}

// Verify checks signature, expiry, issuer and audience of a Cognito JWT and
// returns its claims as strings (the same shape API Gateway injects into
// Authorizer.JWT.Claims).
func (v *Verifier) Verify(ctx context.Context, token string) (map[string]string, error) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("%w: malformed JWT", ErrInvalidToken)
	}

	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid header encoding", ErrInvalidToken)
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		return nil, fmt.Errorf("%w: invalid header", ErrInvalidToken)
	}
	if header.Alg != "RS256" {
		return nil, fmt.Errorf("%w: unsupported algorithm %q", ErrInvalidToken, header.Alg)
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid payload encoding", ErrInvalidToken)
	}
	var claims map[string]any
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, fmt.Errorf("%w: invalid claims", ErrInvalidToken)
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("%w: invalid signature encoding", ErrInvalidToken)
	}

	key, err := v.key(ctx, header.Kid)
	if err != nil {
		return nil, err
	}
	hashed := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(key, crypto.SHA256, hashed[:], signature); err != nil {
		return nil, fmt.Errorf("%w: signature mismatch", ErrInvalidToken)
	}

	if err := v.validateClaims(claims); err != nil {
		return nil, err
	}
	return stringifyClaims(claims), nil
}

func (v *Verifier) validateClaims(claims map[string]any) error {
	exp, ok := claims["exp"].(float64)
	if !ok || time.Now().Unix() >= int64(exp) {
		return fmt.Errorf("%w: token expired", ErrInvalidToken)
	}
	if iss, _ := claims["iss"].(string); strings.TrimRight(iss, "/") != v.issuer {
		return fmt.Errorf("%w: unexpected issuer", ErrInvalidToken)
	}

	// Cognito: los ID tokens llevan la audiencia en `aud`; los access tokens
	// en `client_id`. Ambos se aceptan.
	switch use, _ := claims["token_use"].(string); use {
	case "id":
		if !audienceMatches(claims["aud"], v.audience) {
			return fmt.Errorf("%w: unexpected audience", ErrInvalidToken)
		}
	case "access":
		if clientID, _ := claims["client_id"].(string); clientID != v.audience {
			return fmt.Errorf("%w: unexpected client_id", ErrInvalidToken)
		}
	default:
		return fmt.Errorf("%w: unexpected token_use %q", ErrInvalidToken, use)
	}
	return nil
}

func audienceMatches(aud any, expected string) bool {
	switch value := aud.(type) {
	case string:
		return value == expected
	case []any:
		for _, item := range value {
			if s, ok := item.(string); ok && s == expected {
				return true
			}
		}
	}
	return false
}

func stringifyClaims(claims map[string]any) map[string]string {
	out := make(map[string]string, len(claims))
	for k, value := range claims {
		switch typed := value.(type) {
		case string:
			out[k] = typed
		case float64:
			out[k] = fmt.Sprintf("%.0f", typed)
		case bool:
			out[k] = fmt.Sprintf("%t", typed)
		}
	}
	return out
}

// ── JWKS ─────────────────────────────────────────────────────────────────────

func (v *Verifier) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key := v.keys[kid]
	v.mu.RUnlock()
	if key != nil {
		return key, nil
	}

	// kid desconocido: refresca el JWKS (cubre rotación de llaves del pool).
	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}

	v.mu.RLock()
	key = v.keys[kid]
	v.mu.RUnlock()
	if key == nil {
		return nil, fmt.Errorf("%w: unknown signing key %q", ErrInvalidToken, kid)
	}
	return key, nil
}

func (v *Verifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("building JWKS request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching JWKS: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetching JWKS: unexpected status %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			Kty string `json:"kty"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return fmt.Errorf("decoding JWKS: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(jwks.Keys))
	for _, jwk := range jwks.Keys {
		if jwk.Kty != "RSA" {
			continue
		}
		nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
		if err != nil {
			continue
		}
		keys[jwk.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(nBytes),
			E: int(new(big.Int).SetBytes(eBytes).Int64()),
		}
	}

	v.mu.Lock()
	v.keys = keys
	v.mu.Unlock()
	return nil
}
