package terminal

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// Handler serves terminal HTTP endpoints and SSE streams.
type Handler struct {
	manager *Manager
}

// NewHandler creates a terminal HTTP handler.
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterHandlers mounts terminal routes on the given mux.
func RegisterHandlers(mux *http.ServeMux, manager *Manager) {
	h := NewHandler(manager)
	mux.HandleFunc("POST /api/projects/{projectId}/terminals", h.startTerminal)
	mux.HandleFunc("GET /api/terminals/{terminalId}/stream", h.streamTerminal)
	mux.HandleFunc("POST /api/terminals/{terminalId}/input", h.writeTerminal)
	mux.HandleFunc("POST /api/terminals/{terminalId}/resize", h.resizeTerminal)
	mux.HandleFunc("POST /api/terminals/{terminalId}/close", h.closeTerminal)
	mux.HandleFunc("GET /api/terminals/{terminalId}/status", h.statusTerminal)
}

// startTerminal creates a new terminal session.
func (h *Handler) startTerminal(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	var req struct {
		Cwd string `json:"cwd"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	status, err := h.manager.Start(r.Context(), projectID, req.Cwd)
	if err != nil {
		log.Printf("terminal start error for project %s: %v", projectID, err)
		writeError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, status)
}

// streamTerminal streams terminal output via SSE.
func (h *Handler) streamTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("terminalId")

	// Verify terminal exists.
	status, err := h.manager.Status(terminalID)
	if err != nil || !status.Running {
		writeError(w, http.StatusNotFound, "terminal_not_found", "Terminal not found or not running.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming not supported.")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	// Initial status event.
	statusJSON, _ := json.Marshal(status)
	fmt.Fprintf(w, "data: %s\n\n", statusJSON)
	flusher.Flush()

	buf := make([]byte, 4096)
	for {
		select {
		case <-r.Context().Done():
			return
		default:
		}

		n, err := h.manager.Read(terminalID, buf)
		if n > 0 {
			// Escape newlines for SSE data lines.
			output := buf[:n]
			fmt.Fprintf(w, "data: %s\n\n", escapeSSE(output))
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				fmt.Fprintf(w, "event: closed\ndata: terminal closed\n\n")
			} else {
				log.Printf("terminal stream read error for %s: %v", terminalID, err)
			}
			flusher.Flush()
			return
		}
	}
}

// writeTerminal sends input to the terminal.
func (h *Handler) writeTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("terminalId")

	var req struct {
		Input string `json:"input"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}

	if err := h.manager.Write(terminalID, []byte(req.Input)); err != nil {
		writeError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// resizeTerminal resizes the terminal.
func (h *Handler) resizeTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("terminalId")

	var req struct {
		Rows uint16 `json:"rows"`
		Cols uint16 `json:"cols"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}

	if err := h.manager.Resize(terminalID, req.Rows, req.Cols); err != nil {
		writeError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// closeTerminal stops and cleans up a terminal.
func (h *Handler) closeTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("terminalId")
	if err := h.manager.Close(terminalID); err != nil {
		writeError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// statusTerminal returns the current status of a terminal.
func (h *Handler) statusTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("terminalId")
	status, err := h.manager.Status(terminalID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, status)
}

// escapeSSE replaces newlines with SSE-safe data lines.
func escapeSSE(data []byte) string {
	s := string(data)
	// Replace \n with \ndata: for SSE multi-line data.
	result := ""
	for i, ch := range s {
		if ch == '\n' {
			if i == len(s)-1 {
				result += "\n"
			} else {
				result += "\ndata: "
			}
		} else {
			result += string(ch)
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorResponse struct {
	Error apiError `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Error: apiError{Code: code, Message: message}})
}
