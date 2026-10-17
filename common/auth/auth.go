package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	issuer   = "distributed-computing"
	audience = "employees-api"
)

type Config struct {
	Secret string `yaml:"secret" env:"JWT_SECRET" env-required:"true"`
}

type Auth struct {
	key []byte
}

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int    `json:"expires_in"`
}

type tokenKey struct{}

func New(secret string) (*Auth, error) {
	if len(secret) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must contain at least 32 bytes")
	}
	return &Auth{key: []byte(secret)}, nil
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("WWW-Authenticate", "Bearer")
	w.WriteHeader(http.StatusUnauthorized)
	if _, err := io.WriteString(w, `{"code":401,"message":"unauthorized"}`); err != nil {
		slog.Error("write authentication error", "error", err)
	}
}

func (a *Auth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Fields(r.Header.Get("Authorization"))
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			unauthorized(w)
			return
		}
		claims := &jwt.RegisteredClaims{}
		_, err := jwt.ParseWithClaims(parts[1], claims, func(*jwt.Token) (any, error) {
			return a.key, nil
		}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer), jwt.WithAudience(audience), jwt.WithExpirationRequired(), jwt.WithIssuedAt())
		if err != nil || claims.Subject == "" {
			unauthorized(w)
			return
		}
		ctx := context.WithValue(r.Context(), tokenKey{}, "Bearer "+parts[1])
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LoginHandler
//
// @Summary Issue a JWT token
// @Description Use AUTH_USERNAME and AUTH_PASSWORD from employees/.env. The token expires in 15 minutes.
// @Tags auth
// @Accept json
// @Produce json
// @Param credentials body Credentials true "Login credentials"
// @Success 200 {object} TokenResponse
// @Failure 400 {string} string "Invalid request body"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {string} string "Failed to issue token"
// @Router /auth/login [post]
func (a *Auth) LoginHandler(username, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var credentials Credentials
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&credentials); err != nil {
			http.Error(w, "invalid credentials body", http.StatusBadRequest)
			return
		}
		providedUser, expectedUser := sha256.Sum256([]byte(credentials.Username)), sha256.Sum256([]byte(username))
		providedPass, expectedPass := sha256.Sum256([]byte(credentials.Password)), sha256.Sum256([]byte(password))
		userOK := subtle.ConstantTimeCompare(providedUser[:], expectedUser[:])
		passOK := subtle.ConstantTimeCompare(providedPass[:], expectedPass[:])
		if userOK&passOK != 1 {
			unauthorized(w)
			return
		}
		now := time.Now()
		claims := jwt.RegisteredClaims{
			Issuer: issuer, Audience: jwt.ClaimStrings{audience}, Subject: username,
			IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(15 * time.Minute)),
		}
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(a.key)
		if err != nil {
			http.Error(w, "failed to issue token", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		if err := json.NewEncoder(w).Encode(TokenResponse{AccessToken: token, TokenType: "Bearer", ExpiresIn: 900}); err != nil { //nolint:gosec // G117: login intentionally returns the issued access token.
			slog.ErrorContext(r.Context(), "write token response", "error", err)
		}
	}
}

// Transport forwards the validated bearer token only on internal service calls.
type Transport struct {
	Base http.RoundTripper
}

func (t Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	req := r.Clone(r.Context())
	if token, ok := r.Context().Value(tokenKey{}).(string); ok {
		req.Header.Set("Authorization", token)
	}
	return t.Base.RoundTrip(req)
}
