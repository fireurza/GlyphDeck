package auth

import (
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:?_journal_mode=WAL&_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := NewStore(newTestDB(t))
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func readJSON(resp *http.Response, v any) error {
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(v)
}

func readBody(resp *http.Response) string {
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}

// ---------------------------------------------------------------------------
// Password hashing
// ---------------------------------------------------------------------------

func TestHashPassword(t *testing.T) {
	hash, err := HashPassword("test-password-1234")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if len(hash) == 0 {
		t.Fatal("hash is empty")
	}
}

func TestHashPasswordTooShort(t *testing.T) {
	_, err := HashPassword("short")
	if err == nil {
		t.Fatal("expected error for short password")
	}
}

func TestVerifyPassword(t *testing.T) {
	hash, _ := HashPassword("correct-password")
	if !VerifyPassword(hash, "correct-password") {
		t.Fatal("correct password should verify")
	}
	if VerifyPassword(hash, "wrong-password") {
		t.Fatal("wrong password should not verify")
	}
	if VerifyPassword(nil, "anything") {
		t.Fatal("nil hash should not verify")
	}
}

// ---------------------------------------------------------------------------
// Admin setup + login flow
// ---------------------------------------------------------------------------

func TestSetupRequiredWhenNoAdmin(t *testing.T) {
	store := newTestStore(t)
	hasAdmin, err := store.HasAdmin(context.Background())
	if err != nil {
		t.Fatalf("HasAdmin: %v", err)
	}
	if hasAdmin {
		t.Fatal("expected no admin")
	}
}

func TestCreateAdmin(t *testing.T) {
	store := newTestStore(t)
	hash, _ := HashPassword("mypassword123")
	if err := store.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatalf("SetAdminHash: %v", err)
	}
	hasAdmin, _ := store.HasAdmin(context.Background())
	if !hasAdmin {
		t.Fatal("expected admin to exist")
	}
}

func TestDuplicateSetupRejected(t *testing.T) {
	store := newTestStore(t)
	hash, _ := HashPassword("first-password")
	_ = store.SetAdminHash(context.Background(), hash)

	err := store.SetAdminHash(context.Background(), hash)
	if err != ErrAdminExists {
		t.Fatalf("expected ErrAdminExists, got %v", err)
	}
}

func TestLoginSucceeds(t *testing.T) {
	store := newTestStore(t)
	hash, _ := HashPassword("login-password")
	_ = store.SetAdminHash(context.Background(), hash)

	stored, _ := store.GetAdminHash(context.Background())
	if !VerifyPassword(stored, "login-password") {
		t.Fatal("login should succeed")
	}
}

func TestLoginFailsWrongPassword(t *testing.T) {
	store := newTestStore(t)
	hash, _ := HashPassword("real-password")
	_ = store.SetAdminHash(context.Background(), hash)

	stored, _ := store.GetAdminHash(context.Background())
	if VerifyPassword(stored, "wrong-one") {
		t.Fatal("login should fail with wrong password")
	}
}

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

func TestSessionCreateAndValidate(t *testing.T) {
	store := newTestStore(t)
	token, err := store.CreateSession(context.Background())
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}
	if err := store.ValidateSession(context.Background(), token); err != nil {
		t.Fatalf("ValidateSession: %v", err)
	}
}

func TestInvalidSessionRejected(t *testing.T) {
	store := newTestStore(t)
	err := store.ValidateSession(context.Background(), "nonexistent")
	if err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound, got %v", err)
	}
}

func TestLogoutInvalidatesSession(t *testing.T) {
	store := newTestStore(t)
	token, _ := store.CreateSession(context.Background())
	_ = store.DeleteSession(context.Background(), token)

	err := store.ValidateSession(context.Background(), token)
	if err != ErrSessionNotFound {
		t.Fatalf("expected ErrSessionNotFound after logout, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// HTTP status endpoint
// ---------------------------------------------------------------------------

func TestAuthStatus_SetupRequired(t *testing.T) {
	store := newTestStore(t)
	mux := http.NewServeMux()
	RegisterHandlers(mux, store)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/auth/status")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var status AuthStatus
	_ = readJSON(resp, &status)
	if !status.SetupRequired {
		t.Fatal("expected SetupRequired=true")
	}
}

func TestAuthStatus_LoginRequired(t *testing.T) {
	store := newTestStore(t)
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := store.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatalf("SetAdminHash: %v", err)
	}
	hasAdmin, _ := store.HasAdmin(context.Background())
	if !hasAdmin {
		t.Fatal("admin not created")
	}

	mux := http.NewServeMux()
	RegisterHandlers(mux, store)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := ts.Client().Get(ts.URL + "/api/auth/status")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status endpoint returned %d: %s", resp.StatusCode, readBody(resp))
	}

	var status AuthStatus
	if err := readJSON(resp, &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}
	if !status.LoginRequired {
		t.Fatalf("expected LoginRequired=true, got %+v", status)
	}
}

// ---------------------------------------------------------------------------
// Login/logout HTTP flow
// ---------------------------------------------------------------------------

func TestLoginHTTP(t *testing.T) {
	store := newTestStore(t)
	hash, err := HashPassword("admin-pass")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := store.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatalf("SetAdminHash: %v", err)
	}

	mux := http.NewServeMux()
	RegisterHandlers(mux, store)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := ts.Client().Post(ts.URL+"/api/auth/login", "application/json",
		strings.NewReader(`{"password":"admin-pass"}`))
	if err != nil {
		t.Fatalf("POST login: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200 (body: %s)", resp.StatusCode, readBody(resp))
	}

	cookies := resp.Cookies()
	if len(cookies) == 0 {
		t.Fatal("no session cookie set")
	}
}

func TestLoginWrongPassword(t *testing.T) {
	store := newTestStore(t)
	hash, err := HashPassword("correct123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := store.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatalf("SetAdminHash: %v", err)
	}

	mux := http.NewServeMux()
	RegisterHandlers(mux, store)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, _ := ts.Client().Post(ts.URL+"/api/auth/login", "application/json",
		strings.NewReader(`{"password":"wrong"}`))

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("login status = %d, want 401 (body: %s)", resp.StatusCode, readBody(resp))
	}
	resp.Body.Close()
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

func TestMiddleware_RejectsUnauthenticated(t *testing.T) {
	store := newTestStore(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Middleware(store)(mux)
	ts := httptest.NewServer(wrapped)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/protected")
	if err != nil {
		t.Fatalf("GET /api/protected: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("protected status = %d, want 401", resp.StatusCode)
	}
}

func TestMiddleware_AllowsAuthenticated(t *testing.T) {
	store := newTestStore(t)
	hash, err := HashPassword("password123")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if err := store.SetAdminHash(context.Background(), hash); err != nil {
		t.Fatalf("SetAdminHash: %v", err)
	}

	mux := http.NewServeMux()
	RegisterHandlers(mux, store) // auth endpoints
	mux.HandleFunc("GET /api/protected", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	wrapped := Middleware(store)(mux)
	ts := httptest.NewServer(wrapped)
	defer ts.Close()

	// Login on the same server to get a session cookie scoped to this domain.
	loginResp, err := ts.Client().Post(ts.URL+"/api/auth/login", "application/json",
		strings.NewReader(`{"password":"password123"}`))
	if err != nil {
		t.Fatalf("POST login: %v", err)
	}
	cookies := loginResp.Cookies()
	loginResp.Body.Close()

	// Protected request with the session cookie.
	req, _ := http.NewRequest("GET", ts.URL+"/api/protected", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("GET protected: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("authenticated status = %d (body: %s), want 200", resp.StatusCode, readBody(resp))
	}
}

func TestMiddleware_AllowsPublicPaths(t *testing.T) {
	store := newTestStore(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := Middleware(store)(mux)
	ts := httptest.NewServer(wrapped)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("healthz status = %d, want 200", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Env bootstrap
// ---------------------------------------------------------------------------

func TestEnvBootstrapCreatesAdmin(t *testing.T) {
	os.Setenv("GLYPHDECK_ADMIN_PASSWORD", "bootstrap-pass")
	defer os.Unsetenv("GLYPHDECK_ADMIN_PASSWORD")

	store := newTestStore(t)

	password := os.Getenv("GLYPHDECK_ADMIN_PASSWORD")
	if password != "" {
		hasAdmin, _ := store.HasAdmin(context.Background())
		if !hasAdmin {
			hash2, _ := HashPassword(password)
			_ = store.SetAdminHash(context.Background(), hash2)
		}
	}

	hasAdmin, _ := store.HasAdmin(context.Background())
	if !hasAdmin {
		t.Fatal("expected admin after bootstrap")
	}

	stored, _ := store.GetAdminHash(context.Background())
	if !VerifyPassword(stored, "bootstrap-pass") {
		t.Fatal("bootstrap password should work")
	}
}

func TestEnvBootstrapDoesNotOverwrite(t *testing.T) {
	store := newTestStore(t)
	hash, _ := HashPassword("original-pass")
	_ = store.SetAdminHash(context.Background(), hash)

	os.Setenv("GLYPHDECK_ADMIN_PASSWORD", "bootstrap-pass")
	defer os.Unsetenv("GLYPHDECK_ADMIN_PASSWORD")

	password := os.Getenv("GLYPHDECK_ADMIN_PASSWORD")
	if password != "" {
		hasAdmin, _ := store.HasAdmin(context.Background())
		if !hasAdmin {
			hash2, _ := HashPassword(password)
			_ = store.SetAdminHash(context.Background(), hash2)
		}
	}

	stored, _ := store.GetAdminHash(context.Background())
	if !VerifyPassword(stored, "original-pass") {
		t.Fatal("original password should still work")
	}
	if VerifyPassword(stored, "bootstrap-pass") {
		t.Fatal("bootstrap password should not work")
	}
}

func TestCookieSecurePolicyHTTPS(t *testing.T) {
	r, _ := http.NewRequest("GET", "https://127.0.0.1:8756/", nil)
	r.TLS = &tls.ConnectionState{}
	r.Host = "127.0.0.1:8756"
	c := newSessionCookie("token", 0, r)
	if !c.Secure {
		t.Fatal("Secure must be true for TLS connection")
	}
}

func TestCookieSecurePolicyLaxMode(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://127.0.0.1:8756/", nil)
	r.Host = "127.0.0.1:8756"
	c := newSessionCookie("token", 0, r)
	if c.SameSite != http.SameSiteLaxMode {
		t.Fatalf("SameSite = %v, want Lax", c.SameSite)
	}
}

func TestCookieLoginLogoutSameAttributes(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://127.0.0.1:8756/", nil)
	r.Host = "127.0.0.1:8756"
	loginCookie := newSessionCookie("abc", 0, r)
	logoutCookie := newSessionCookie("", -1, r)
	if loginCookie.Path != logoutCookie.Path {
		t.Fatal("login and logout must use same Path")
	}
	if loginCookie.HttpOnly != logoutCookie.HttpOnly {
		t.Fatal("login and logout must use same HttpOnly")
	}
	if loginCookie.SameSite != logoutCookie.SameSite {
		t.Fatal("login and logout must use same SameSite")
	}
}

func TestCookieSecureFalseForLoopback(t *testing.T) {
	for _, host := range []string{
		"127.0.0.1:8756",
		"127.0.0.1",
		"[::1]:8756",
	} {
		t.Run(host, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "http://"+host+"/", nil)
			r.Host = host
			c := newSessionCookie("token", 0, r)
			if c.Secure {
				t.Fatalf("Secure must be false for loopback host %q", host)
			}
		})
	}
}

func TestCookieSecureTrueForNonLoopback(t *testing.T) {
	for _, host := range []string{
		"192.168.1.1:443",
		"example.com:443",
		"example.com",
		"mylocalhost.internal:8080",
	} {
		t.Run(host, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "http://"+host+"/", nil)
			r.Host = host
			c := newSessionCookie("token", 0, r)
			if !c.Secure {
				t.Fatalf("Secure must be true for non-loopback host %q", host)
			}
		})
	}
}

func TestCookieSecureNoXForwardedProtoTrust(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://127.0.0.1:8756/", nil)
	r.Host = "127.0.0.1:8756"
	r.Header.Set("X-Forwarded-Proto", "https")
	c := newSessionCookie("token", 0, r)
	if c.Secure {
		t.Fatal("Secure must not trust X-Forwarded-Proto header")
	}
}

func TestCookieMalformedHost(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://127.0.0.1:8756/", nil)
	r.Host = ":bad"
	c := newSessionCookie("token", 0, r)
	if !c.HttpOnly {
		t.Fatal("HttpOnly must be true even with malformed host")
	}
	if !c.Secure {
		t.Fatal("malformed host must default to Secure=true (cannot confirm loopback)")
	}
}

func TestCookieDeletionAttributes(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://127.0.0.1:8756/", nil)
	r.Host = "127.0.0.1:8756"
	c := newSessionCookie("", -1, r)
	if c.MaxAge != -1 {
		t.Fatalf("MaxAge = %d, want -1 for deletion", c.MaxAge)
	}
	if c.Value != "" {
		t.Fatal("Value must be empty for deletion cookie")
	}
	if c.Path != "/" {
		t.Fatal("Path must be / for deletion cookie")
	}
}

func TestIsLoopbackIPv6(t *testing.T) {
	r, _ := http.NewRequest("GET", "/", nil)
	r.Host = "[::1]:8756"
	if !isLoopbackRequest(r) {
		t.Fatal("[::1] must be detected as loopback")
	}
}

func FuzzCookieSecurity(f *testing.F) {
	seeds := []string{
		"127.0.0.1:8756",
		"127.0.0.1",
		"localhost:8756",
		"localhost",
		"192.168.1.1:443",
		"example.com:443",
		"example.com",
		"[::1]:8756",
		"[::1]",
		"bad-host",
		"",
		":8756",
	}
	for _, s := range seeds {
		f.Add(s, 0)
		f.Add(s, -1)
		f.Add(s, 3600)
	}

	f.Fuzz(func(t *testing.T, host string, maxAge int) {
		r, _ := http.NewRequest("GET", "/", nil)
		r.Host = host

		// Non-TLS request should not panic.
		cookie := newSessionCookie("test-token", maxAge, r)
		if cookie.Name != sessionCookieName {
			t.Errorf("cookie name = %q, want %q", cookie.Name, sessionCookieName)
		}
		if cookie.HttpOnly != true {
			t.Errorf("cookie HttpOnly = %v, want true", cookie.HttpOnly)
		}
		if cookie.Path != "/" {
			t.Errorf("cookie Path = %q, want /", cookie.Path)
		}
		if cookie.MaxAge != maxAge {
			t.Errorf("cookie MaxAge = %d, want %d", cookie.MaxAge, maxAge)
		}
		if cookie.Secure && isLoopbackRequest(r) {
			t.Errorf("cookie Secure=true for loopback request host=%q (expected false)", host)
		}
	})
}

func FuzzIsLoopbackRequest(f *testing.F) {
	seeds := []string{
		"127.0.0.1:8756",
		"127.0.0.1",
		"[::1]:8756",
		"[::1]",
		"localhost:8756",
		"localhost",
		"192.168.1.1",
		"example.com",
		"",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, host string) {
		r, _ := http.NewRequest("GET", "/", nil)
		r.Host = host
		_ = isLoopbackRequest(r)
	})
}
