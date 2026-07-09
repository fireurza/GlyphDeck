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
