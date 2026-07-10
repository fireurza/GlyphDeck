package httpapi

import (
	"encoding/json"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func WriteJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func WriteError(w http.ResponseWriter, status int, code, message string) {
	WriteJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}

func HasJSONContentType(r *http.Request) bool {
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		return false
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "application/json"
}

func IsMutationMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func SameOriginMutation(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	return AllowLocalMutation(r)
}

func AllowLocalMutation(r *http.Request) bool {
	requestHost := NormalizedHostPort(r.Host)
	if !IsLoopbackHost(Hostname(requestHost)) {
		return false
	}

	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	parsed, err := url.Parse(origin)
	if err != nil || parsed.Scheme != "http" || parsed.Host == "" {
		return false
	}
	originHost := NormalizedHostPort(parsed.Host)
	if originHost == requestHost {
		return true
	}
	return DevToolsEnabled() && IsLoopbackHost(Hostname(originHost))
}

func DevToolsEnabled() bool {
	return os.Getenv("GLYPHDECK_DEV_TOOLS") == "1"
}

func NormalizedHostPort(hostPort string) string {
	return strings.ToLower(strings.TrimSpace(hostPort))
}

func Hostname(hostPort string) string {
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		host = hostPort
	}
	host = strings.TrimSpace(host)
	host = strings.TrimPrefix(host, "[")
	host = strings.TrimSuffix(host, "]")
	return strings.ToLower(host)
}

func IsLoopbackHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
