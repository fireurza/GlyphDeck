package auth

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"

	"glyphdeck/internal/httpapi"
)

const (
	sessionCookieName = "glyphdeck_session"
)

// Handler serves auth HTTP endpoints.
type Handler struct {
	store *Store
}

// NewHandler creates an auth Handler.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterHandlers mounts auth routes on the given mux. These routes are
// intentionally NOT behind auth middleware.
func RegisterHandlers(mux *http.ServeMux, store *Store) {
	h := NewHandler(store)
	mux.HandleFunc("GET /api/auth/status", h.handleStatus)
	mux.HandleFunc("POST /api/auth/setup", h.handleSetup)
	mux.HandleFunc("POST /api/auth/login", h.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", h.handleLogout)
	mux.HandleFunc("GET /api/auth/me", h.handleMe)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := h.store.HasAdmin(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "auth_error", "Could not check admin status.")
		return
	}

	status := AuthStatus{AdminExists: hasAdmin}
	if !hasAdmin {
		status.SetupRequired = true
	} else if !h.hasValidSession(r) {
		status.LoginRequired = true
	}

	httpapi.WriteJSON(w, http.StatusOK, status)
}

func (h *Handler) handleSetup(w http.ResponseWriter, r *http.Request) {
	hasAdmin, err := h.store.HasAdmin(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "auth_error", "Could not check admin status.")
		return
	}
	if hasAdmin {
		httpapi.WriteError(w, http.StatusConflict, "admin_exists", "Admin user already exists.")
		return
	}

	var req SetupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "weak_password", err.Error())
		return
	}
	if err := h.store.SetAdminHash(r.Context(), hash); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "auth_error", "Could not create admin.")
		return
	}

	log.Println("auth: admin setup completed")
	httpapi.WriteJSON(w, http.StatusCreated, map[string]string{"message": "Admin created. You can now log in."})
}

func (h *Handler) handleLogin(w http.ResponseWriter, r *http.Request) {
	hash, err := h.store.GetAdminHash(r.Context())
	if errors.Is(err, ErrNoAdmin) {
		httpapi.WriteError(w, http.StatusNotFound, "no_admin", "No admin configured. Run setup first.")
		return
	}
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "auth_error", "Could not verify credentials.")
		return
	}

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}

	if !VerifyPassword(hash, req.Password) {
		httpapi.WriteError(w, http.StatusUnauthorized, "invalid_credentials", "Invalid password.")
		return
	}

	token, err := h.store.CreateSession(r.Context())
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "auth_error", "Could not create session.")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   0, // session cookie (deleted on browser close)
	})

	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "Logged in."})
}

func (h *Handler) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := h.sessionToken(r)
	if token != "" {
		_ = h.store.DeleteSession(r.Context(), token)
	}

	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})

	httpapi.WriteJSON(w, http.StatusOK, map[string]string{"message": "Logged out."})
}

func (h *Handler) handleMe(w http.ResponseWriter, r *http.Request) {
	if !h.hasValidSession(r) {
		httpapi.WriteError(w, http.StatusUnauthorized, "unauthenticated", "Not logged in.")
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, map[string]any{
		"authenticated": true,
		"username":      "admin",
	})
}

// ----- helpers -----

func (h *Handler) sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return ""
	}
	return cookie.Value
}

func (h *Handler) hasValidSession(r *http.Request) bool {
	token := h.sessionToken(r)
	if token == "" {
		return false
	}
	return h.store.ValidateSession(r.Context(), token) == nil
}

// RequestHasValidSession is used by middleware to check auth without
// requiring the full Handler.
func RequestHasValidSession(store *Store, r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return false
	}
	return store.ValidateSession(r.Context(), cookie.Value) == nil
}

// Middleware returns HTTP middleware that requires authentication for protected routes.
// Public paths (health, auth endpoints, static assets) are allowed through.
func Middleware(store *Store) func(http.Handler) http.Handler {
	publicPrefixes := []string{
		"/healthz",
		"/api/auth/",
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Allow public paths.
			for _, prefix := range publicPrefixes {
				if strings.HasPrefix(path, prefix) {
					next.ServeHTTP(w, r)
					return
				}
			}

			// Allow static frontend assets (no /api/ prefix).
			if !strings.HasPrefix(path, "/api/") {
				next.ServeHTTP(w, r)
				return
			}

			// Require auth for all other API endpoints.
			if !RequestHasValidSession(store, r) {
				if strings.HasPrefix(r.Header.Get("Accept"), "application/json") ||
					strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") ||
					r.Method != "GET" {
					httpapi.WriteError(w, http.StatusUnauthorized, "unauthenticated", "Authentication required.")
					return
				}
				// For page loads (Accept: text/html), redirect to login.
				http.Error(w, "Authentication required.", http.StatusUnauthorized)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
