package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type User struct {
	ID    string
	Email string
}

type Verifier struct {
	jwksURL string
	issuer  string
	client  *http.Client

	mu   sync.RWMutex
	keys map[string]*ecdsa.PublicKey
}

type jwksResponse struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Alg string `json:"alg"`
	Crv string `json:"crv"`
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func NewVerifier(supabaseURL string) *Verifier {
	base := strings.TrimRight(supabaseURL, "/")

	return &Verifier{
		jwksURL: base + "/auth/v1/.well-known/jwks.json",
		issuer:  base + "/auth/v1",
		client:  &http.Client{Timeout: 10 * time.Second},
		keys:    make(map[string]*ecdsa.PublicKey),
	}
}

func (v *Verifier) Verify(ctx context.Context, tokenString string) (*User, error) {
	token, err := jwt.Parse(
		tokenString,
		func(token *jwt.Token) (any, error) {
			if token.Method.Alg() != "ES256" {
				return nil, fmt.Errorf(
					"unexpected signing algorithm: %s",
					token.Method.Alg(),
				)
			}

			kid, ok := token.Header["kid"].(string)
			if !ok || kid == "" {
				return nil, fmt.Errorf("token has no key id")
			}

			return v.getKey(ctx, kid)
		},
		jwt.WithExpirationRequired(),
		jwt.WithAudience("authenticated"),
		jwt.WithIssuer(v.issuer),
	)

	if err != nil {
		return nil, fmt.Errorf("invalid access token: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("unexpected claims type")
	}

	sub, ok := claims["sub"].(string)
	if !ok || sub == "" {
		return nil, fmt.Errorf("token has no subject")
	}

	emailString, _ := claims["email"].(string)

	return &User{
		ID:    sub,
		Email: emailString,
	}, nil
}

func (v *Verifier) getKey(ctx context.Context, kid string) (*ecdsa.PublicKey, error) {
	v.mu.RLock()
	key := v.keys[kid]
	v.mu.RUnlock()

	if key != nil {
		return key, nil
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		v.jwksURL,
		nil,
	)
	if err != nil {
		return nil, err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetch JWKS: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS returned HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read JWKS: %w", err)
	}

	var data jwksResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, fmt.Errorf("decode JWKS: %w", err)
	}

	for _, k := range data.Keys {
		if k.Kid != kid {
			continue
		}

		if k.Kty != "EC" || k.Crv != "P-256" || k.Alg != "ES256" {
			return nil, fmt.Errorf("unsupported signing key %s", kid)
		}

		xBytes, err := base64.RawURLEncoding.DecodeString(k.X)
		if err != nil {
			return nil, fmt.Errorf("decode key x: %w", err)
		}

		yBytes, err := base64.RawURLEncoding.DecodeString(k.Y)
		if err != nil {
			return nil, fmt.Errorf("decode key y: %w", err)
		}

		curve := elliptic.P256()

		key := &ecdsa.PublicKey{
			Curve: curve,
			X:     new(big.Int).SetBytes(xBytes),
			Y:     new(big.Int).SetBytes(yBytes),
		}

		v.mu.Lock()
		v.keys[kid] = key
		v.mu.Unlock()

		return key, nil
	}

	return nil, fmt.Errorf("signing key %q not found in JWKS", kid)
}
