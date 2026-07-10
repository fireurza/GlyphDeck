package servers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"glyphdeck/internal/opencode"
)

// ---------------------------------------------------------------------------
// HTTP test fixtures
// ---------------------------------------------------------------------------

func setupTestServer(t *testing.T, detector opencode.Detector, resolver ProjectResolver) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	manager := NewManager(detector, resolver)
	RegisterHandlers(mux, manager)
	return httptest.NewServer(mux)
}

func postSameOrigin(t *testing.T, ts *httptest.Server, path, contentType string, body io.Reader) (*http.Response, error) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, ts.URL+path, body)
	if err != nil {
		t.Fatalf("create POST request: %v", err)
	}
	req.Header.Set("Origin", ts.URL)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return ts.Client().Do(req)
}

type testErrorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

func readErrorResponse(t *testing.T, body []byte) testErrorResponse {
	t.Helper()
	var er testErrorResponse
	if err := json.Unmarshal(body, &er); err != nil {
		t.Fatalf("unmarshal error response: %v\nbody: %s", err, body)
	}
	return er
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func TestHandleOpencodeStatus_200(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{projects: map[string]ProjectInfo{}}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/opencode")
	if err != nil {
		t.Fatalf("GET /api/opencode: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result opencode.DetectionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if !result.Installed {
		t.Error("Installed = false, want true")
	}
	if result.Status != "ready" {
		t.Errorf("Status = %q, want %q", result.Status, "ready")
	}
}

func TestHandleOpencodeStatus_NotInstalled(t *testing.T) {
	detector := mockDetector{installed: false}
	resolver := mockResolver{projects: map[string]ProjectInfo{}}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/opencode")
	if err != nil {
		t.Fatalf("GET /api/opencode: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var result opencode.DetectionResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if result.Installed {
		t.Error("Installed = true, want false")
	}
	if result.Status != "not-installed" {
		t.Errorf("Status = %q, want %q", result.Status, "not-installed")
	}
}

func TestHandleServerStatus_Stopped(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: t.TempDir()},
		},
	}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/projects/proj-1/server")
	if err != nil {
		t.Fatalf("GET server status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var status ServerStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if status.Status != StateStopped {
		t.Errorf("Status = %q, want %q", status.Status, StateStopped)
	}
}

func TestHandleServerStatus_UnknownProject(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{projects: map[string]ProjectInfo{}}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/api/projects/nonexistent/server")
	if err != nil {
		t.Fatalf("GET server status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var status ServerStatus
	if err := json.NewDecoder(resp.Body).Decode(&status); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if status.Status != StateUnknown {
		t.Errorf("Status = %q, want %q", status.Status, StateUnknown)
	}
}

func TestHandleServerStart_NotInstalled(t *testing.T) {
	detector := mockDetector{installed: false}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: t.TempDir()},
		},
	}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/server/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST start: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}

func TestHandleServerStart_ProjectNotFound(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{projects: map[string]ProjectInfo{}}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	resp, err := postSameOrigin(t, ts, "/api/projects/nonexistent/server/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST start: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleServerStart_SameOriginRejection(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{projects: map[string]ProjectInfo{}}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/proj-1/server/start", nil)
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST start with cross-origin: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestHandleServerStop_NotRunning(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: t.TempDir()},
		},
	}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/server/stop", "application/json", nil)
	if err != nil {
		t.Fatalf("POST stop: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestHandleServerStop_SameOriginRejection(t *testing.T) {
	detector := mockDetector{installed: true, executable: "opencode", version: "v1.0.0"}
	resolver := mockResolver{projects: map[string]ProjectInfo{}}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/api/projects/proj-1/server/stop", nil)
	req.Header.Set("Origin", "http://evil.com")
	req.Header.Set("Content-Type", "application/json")

	resp, err := ts.Client().Do(req)
	if err != nil {
		t.Fatalf("POST stop with cross-origin: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
	}
}

func TestHandleServerStart_JSONResponseShape(t *testing.T) {
	// Test that error responses use the correct shape.
	detector := mockDetector{installed: false}
	resolver := mockResolver{
		projects: map[string]ProjectInfo{
			"proj-1": {ID: "proj-1", Name: "Test", Path: t.TempDir()},
		},
	}
	ts := setupTestServer(t, detector, resolver)
	defer ts.Close()

	resp, err := postSameOrigin(t, ts, "/api/projects/proj-1/server/start", "application/json", nil)
	if err != nil {
		t.Fatalf("POST start: %v", err)
	}
	defer resp.Body.Close()

	// Read full body.
	buf := make([]byte, 4096)
	n, _ := resp.Body.Read(buf)
	body := buf[:n]

	er := readErrorResponse(t, body)
	if er.Error.Code != "opencode_not_installed" {
		t.Errorf("Error.Code = %q, want %q", er.Error.Code, "opencode_not_installed")
	}
	if er.Error.Message == "" {
		t.Error("Error.Message is empty")
	}
}
