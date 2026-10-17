package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWTValidation(t *testing.T) {
	secret := strings.Repeat("s", 32)
	a, err := New(secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New("short"); err == nil {
		t.Fatal("weak JWT key accepted")
	}
	for _, name := range []string{"valid", "expired", "missing expiry", "wrong issuer", "wrong audience", "no subject", "future issue", "wrong key", "wrong algorithm", "unsigned"} {
		t.Run(name, func(t *testing.T) {
			claims := jwt.RegisteredClaims{Issuer: issuer, Audience: jwt.ClaimStrings{audience}, Subject: "admin", ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Minute))}
			method := jwt.SigningMethod(jwt.SigningMethodHS256)
			key := any([]byte(secret))
			switch name {
			case "expired":
				claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Minute))
			case "missing expiry":
				claims.ExpiresAt = nil
			case "wrong issuer":
				claims.Issuer = "other"
			case "wrong audience":
				claims.Audience = jwt.ClaimStrings{"other"}
			case "no subject":
				claims.Subject = ""
			case "future issue":
				claims.IssuedAt = jwt.NewNumericDate(time.Now().Add(time.Hour))
			case "wrong key":
				key = []byte(strings.Repeat("x", 32))
			case "wrong algorithm":
				method = jwt.SigningMethodHS384
			case "unsigned":
				method, key = jwt.SigningMethodNone, jwt.UnsafeAllowNoneSignatureType
			}
			token, err := jwt.NewWithClaims(method, claims).SignedString(key)
			if err != nil {
				t.Fatal(err)
			}
			r := httptest.NewRequest(http.MethodGet, "/api/v1/employees", nil)
			r.Header.Set("Authorization", "Bearer "+token)
			w := httptest.NewRecorder()
			a.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })).ServeHTTP(w, r)
			want := http.StatusUnauthorized
			if name == "valid" {
				want = http.StatusNoContent
			}
			if w.Code != want {
				t.Fatalf("status=%d want=%d", w.Code, want)
			}
		})
	}
	w := httptest.NewRecorder()
	a.Middleware(http.NotFoundHandler()).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusUnauthorized {
		t.Fatal("missing token accepted")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestLoginAndTokenForwarding(t *testing.T) {
	a, err := New(strings.Repeat("s", 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{`{"username":"admin","password":"wrong"}`, `{"username":"other","password":"password"}`} {
		w := httptest.NewRecorder()
		a.LoginHandler("admin", "password").ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(body)))
		if w.Code != http.StatusUnauthorized {
			t.Fatal("invalid login accepted")
		}
	}
	w := httptest.NewRecorder()
	a.LoginHandler("admin", "password").ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/auth/login", strings.NewReader(`{"username":"admin","password":"password"}`)))
	var response TokenResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.AccessToken == "" || response.TokenType != "Bearer" || response.ExpiresIn != 900 || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("login did not issue a token")
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Authorization", "Bearer "+response.AccessToken)
	a.Middleware(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		request, err := http.NewRequestWithContext(r.Context(), http.MethodGet, "http://internal/api/v1/employees", nil)
		if err != nil {
			t.Fatal(err)
		}
		transport := Transport{Base: roundTripFunc(func(forwarded *http.Request) (*http.Response, error) {
			if forwarded.Header.Get("Authorization") != "Bearer "+response.AccessToken {
				t.Fatal("validated token not forwarded")
			}
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(""))}, nil
		})}
		resp, err := transport.RoundTrip(request)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close() //nolint:errcheck
		if request.Header.Get("Authorization") != "" {
			t.Fatal("transport mutated the caller's request")
		}
	})).ServeHTTP(httptest.NewRecorder(), r)
}
