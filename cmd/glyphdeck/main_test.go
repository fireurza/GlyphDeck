package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServeFrontendUsesEmbeddedAssetsOutsideRepository(t *testing.T) {
	t.Chdir(t.TempDir())

	for _, requestPath := range []string{"/", "/client-side-route"} {
		t.Run(requestPath, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, requestPath, nil)
			response := httptest.NewRecorder()

			serveFrontend(response, request)

			if response.Code != http.StatusOK {
				t.Fatalf("GET %s status = %d, want %d", requestPath, response.Code, http.StatusOK)
			}
			if !strings.Contains(response.Body.String(), `<div id="root"></div>`) {
				t.Fatalf("GET %s did not return the embedded frontend", requestPath)
			}
		})
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{host: "127.0.0.1", want: true},
		{host: "::1", want: true},
		{host: "localhost", want: true},
		{host: "0.0.0.0", want: false},
		{host: "192.168.1.10", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			if got := isLoopbackHost(tt.host); got != tt.want {
				t.Fatalf("isLoopbackHost(%q) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}
}

func TestLocalMutationGuard(t *testing.T) {
	guard := localMutationGuard(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	tests := []struct {
		name   string
		method string
		host   string
		origin string
		want   int
	}{
		{name: "loopback same origin", method: http.MethodPost, host: "127.0.0.1:8756", origin: "http://127.0.0.1:8756", want: http.StatusNoContent},
		{name: "loopback Vite origin", method: http.MethodPost, host: "127.0.0.1:8756", origin: "http://localhost:5173", want: http.StatusNoContent},
		{name: "cross origin mutation", method: http.MethodPost, host: "127.0.0.1:8756", origin: "http://evil.example", want: http.StatusForbidden},
		{name: "non-loopback host", method: http.MethodPost, host: "example.com", want: http.StatusForbidden},
		{name: "cross origin read", method: http.MethodGet, host: "127.0.0.1:8756", origin: "http://evil.example", want: http.StatusNoContent},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, "http://"+tt.host+"/api/test", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			res := httptest.NewRecorder()

			guard.ServeHTTP(res, req)
			if res.Code != tt.want {
				t.Fatalf("status = %d, want %d", res.Code, tt.want)
			}
		})
	}
}
