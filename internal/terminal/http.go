package terminal

import (
	"encoding/json"
	"fmt"
	"glyphdeck/internal/httpapi"
	"io"
	"log"
	"net/http"
	"time"
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
		httpapi.WriteError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusCreated, status)
}

// streamTerminal streams terminal output via SSE.
func (h *Handler) streamTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("terminalId")

	status, err := h.manager.Status(terminalID)
	if err != nil || !status.Running {
		httpapi.WriteError(w, http.StatusNotFound, "terminal_not_found", "Terminal not found or not running.")
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		httpapi.WriteError(w, http.StatusInternalServerError, "streaming_unsupported", "Streaming not supported.")
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

	// Use a goroutine to read stdout and push to a channel, so we can
	// flush frequently even when stdout is idle.
	ch := make(chan []byte, 32)
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer func() { close(ch) }()

		reader, err := h.manager.NewReader(terminalID)
		if err != nil {
			return
		}

		buf := make([]byte, 4096)
		for {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			n, readErr := reader.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case ch <- data:
				case <-r.Context().Done():
					return
				}
			}
			if readErr != nil {
				if readErr == io.EOF {
					select {
					case ch <- nil: // signal EOF
					case <-r.Context().Done():
					}
				} else {
					log.Printf("terminal stream read error for %s: %v", terminalID, readErr)
				}
				return
			}
		}
	}()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	var accumulator []byte
	flushAccumulator := func() {
		if len(accumulator) == 0 {
			return
		}
		fmt.Fprintf(w, "data: %s\n\n", escapeSSE(accumulator))
		flusher.Flush()
		accumulator = accumulator[:0]
	}

	for {
		select {
		case <-r.Context().Done():
			return
		case data, ok := <-ch:
			if data != nil {
				accumulator = append(accumulator, data...)
				// Flush on each chunk — the goroutine reads whenever
				// stdout has data, so flushing immediately is responsive.
				flushAccumulator()
			}
			if !ok || data == nil {
				// EOF or channel closed.
				flushAccumulator()
				fmt.Fprintf(w, "event: closed\ndata: terminal closed\n\n")
				flusher.Flush()
				return
			}
		case <-ticker.C:
			flushAccumulator()
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}

	if err := h.manager.Write(terminalID, []byte(req.Input)); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "terminal_error", err.Error())
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
		httpapi.WriteError(w, http.StatusBadRequest, "invalid_json", "Request body must be valid JSON.")
		return
	}

	if err := h.manager.Resize(terminalID, req.Rows, req.Cols); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// closeTerminal stops and cleans up a terminal.
func (h *Handler) closeTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("terminalId")
	if err := h.manager.Close(terminalID); err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// statusTerminal returns the current status of a terminal.
func (h *Handler) statusTerminal(w http.ResponseWriter, r *http.Request) {
	terminalID := r.PathValue("terminalId")
	status, err := h.manager.Status(terminalID)
	if err != nil {
		httpapi.WriteError(w, http.StatusInternalServerError, "terminal_error", err.Error())
		return
	}
	httpapi.WriteJSON(w, http.StatusOK, status)
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
