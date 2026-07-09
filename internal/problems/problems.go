// Package problems manages GlyphDeck app-level error tracking for the Problems tab.
package problems

import (
	"sync"
	"time"
)

// Problem represents a GlyphDeck app-level issue.
type Problem struct {
	ID        string `json:"id"`
	Level     string `json:"level"` // "error" or "warn"
	Source    string `json:"source"`
	Message   string `json:"message"`
	CreatedAt string `json:"createdAt"`
}

// Manager tracks problems with a bounded ring buffer.
type Manager struct {
	mu       sync.RWMutex
	problems []Problem
	maxSize  int
	nextID   int
}

// NewManager creates a problem manager with the given max problem count.
func NewManager(maxSize int) *Manager {
	return &Manager{
		problems: make([]Problem, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Add records a new problem.
func (m *Manager) Add(level, source, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.nextID++
	p := Problem{
		ID:        formatID(m.nextID),
		Level:     level,
		Source:    source,
		Message:   message,
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	}

	if len(m.problems) >= m.maxSize {
		m.problems = m.problems[1:]
	}
	m.problems = append(m.problems, p)
}

// List returns a copy of all tracked problems, newest first.
func (m *Manager) List() []Problem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Problem, len(m.problems))
	for i := len(m.problems) - 1; i >= 0; i-- {
		result[len(m.problems)-1-i] = m.problems[i]
	}
	return result
}

// Clear removes all tracked problems.
func (m *Manager) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.problems = m.problems[:0]
}

func formatID(n int) string {
	return "prob-" + itoa(n)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
