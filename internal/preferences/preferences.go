// Package preferences manages typed GlyphDeck preferences backed by SQLite
// with revision tracking, automatic backups, and optimistic concurrency.
package preferences

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Prefs holds all user-configurable GlyphDeck preferences.
type Prefs struct {
	Appearance              string `json:"appearance"`              // "system", "light", "dark"
	InterfaceDensity        string `json:"interfaceDensity"`        // "comfortable", "compact"
	TerminalFontSize        int    `json:"terminalFontSize"`        // 11-24
	TranscriptAutoScroll    bool   `json:"transcriptAutoScroll"`    // true/false
	DefaultRightPanelTab    string `json:"defaultRightPanelTab"`    // "review", "usage", "agents"
	DestructiveConfirmations bool  `json:"destructiveConfirmations"` // true/false
}

// PrefsDocument is the envelope returned by the API, including revision info.
type PrefsDocument struct {
	Data     Prefs  `json:"data"`
	Revision int    `json:"revision"`
	UpdatedAt string `json:"updatedAt,omitempty"`
}

// Defaults returns preferences with safe defaults that preserve current GlyphDeck behavior.
func Defaults() Prefs {
	return Prefs{
		Appearance:               "system",
		InterfaceDensity:         "comfortable",
		TerminalFontSize:         14,
		TranscriptAutoScroll:     true,
		DefaultRightPanelTab:     "review",
		DestructiveConfirmations: true,
	}
}

// Validate returns field-level validation errors.
func (p *Prefs) Validate() []FieldError {
	var errs []FieldError
	if p.Appearance != "" && p.Appearance != "system" && p.Appearance != "light" && p.Appearance != "dark" {
		errs = append(errs, FieldError{Field: "appearance", Message: "must be system, light, or dark"})
	}
	if p.InterfaceDensity != "" && p.InterfaceDensity != "comfortable" && p.InterfaceDensity != "compact" {
		errs = append(errs, FieldError{Field: "interfaceDensity", Message: "must be comfortable or compact"})
	}
	if p.TerminalFontSize != 0 && (p.TerminalFontSize < 11 || p.TerminalFontSize > 24) {
		errs = append(errs, FieldError{Field: "terminalFontSize", Message: "must be between 11 and 24"})
	}
	if p.DefaultRightPanelTab != "" && p.DefaultRightPanelTab != "review" && p.DefaultRightPanelTab != "usage" && p.DefaultRightPanelTab != "agents" {
		errs = append(errs, FieldError{Field: "defaultRightPanelTab", Message: "must be review, usage, or agents"})
	}
	return errs
}

// FieldError describes a single validation error.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// ChangeSummary describes how a preview or update changes preferences.
type ChangeSummary struct {
	Fields []FieldChange `json:"fields"`
}

// FieldChange describes a single field-level change.
type FieldChange struct {
	Field    string `json:"field"`
	OldValue any    `json:"oldValue"`
	NewValue any    `json:"newValue"`
}

// PreviewResult is returned by the preview endpoint.
type PreviewResult struct {
	Normalized PrefsDocument `json:"normalized"`
	Changes    ChangeSummary `json:"changes"`
	Errors     []FieldError  `json:"errors,omitempty"`
}

// BackupEntry describes a stored backup.
type BackupEntry struct {
	ID        int            `json:"id"`
	Revision  int            `json:"revision"`
	CreatedAt string         `json:"createdAt"`
	Summary   ChangeSummary  `json:"summary"`
}

var (
	ErrRevisionConflict = errors.New("revision conflict")
	ErrValidationFailed = errors.New("validation failed")
	ErrNotFound         = errors.New("not found")
)

// UpdateRequest is the body for PUT /api/settings.
type UpdateRequest struct {
	Data             Prefs `json:"data"`
	ExpectedRevision int   `json:"expectedRevision"`
}

// Diff computes the change summary between old and new preferences.
func Diff(old, new Prefs) ChangeSummary {
	var fields []FieldChange
	if old.Appearance != new.Appearance {
		fields = append(fields, FieldChange{Field: "appearance", OldValue: old.Appearance, NewValue: new.Appearance})
	}
	if old.InterfaceDensity != new.InterfaceDensity {
		fields = append(fields, FieldChange{Field: "interfaceDensity", OldValue: old.InterfaceDensity, NewValue: new.InterfaceDensity})
	}
	if old.TerminalFontSize != new.TerminalFontSize {
		fields = append(fields, FieldChange{Field: "terminalFontSize", OldValue: old.TerminalFontSize, NewValue: new.TerminalFontSize})
	}
	if old.TranscriptAutoScroll != new.TranscriptAutoScroll {
		fields = append(fields, FieldChange{Field: "transcriptAutoScroll", OldValue: old.TranscriptAutoScroll, NewValue: new.TranscriptAutoScroll})
	}
	if old.DefaultRightPanelTab != new.DefaultRightPanelTab {
		fields = append(fields, FieldChange{Field: "defaultRightPanelTab", OldValue: old.DefaultRightPanelTab, NewValue: new.DefaultRightPanelTab})
	}
	if old.DestructiveConfirmations != new.DestructiveConfirmations {
		fields = append(fields, FieldChange{Field: "destructiveConfirmations", OldValue: old.DestructiveConfirmations, NewValue: new.DestructiveConfirmations})
	}
	return ChangeSummary{Fields: fields}
}

// Normalize fills zero values with defaults and clamps numeric ranges.
func (p *Prefs) Normalize() {
	def := Defaults()
	if p.Appearance == "" {
		p.Appearance = def.Appearance
	}
	if p.InterfaceDensity == "" {
		p.InterfaceDensity = def.InterfaceDensity
	}
	if p.TerminalFontSize == 0 {
		p.TerminalFontSize = def.TerminalFontSize
	}
	if p.DefaultRightPanelTab == "" {
		p.DefaultRightPanelTab = def.DefaultRightPanelTab
	}
	// Clamp terminal font size.
	if p.TerminalFontSize < 11 {
		p.TerminalFontSize = 11
	}
	if p.TerminalFontSize > 24 {
		p.TerminalFontSize = 24
	}
}

// --- JSON/YAML helpers for preview parsing ---

// ParsePrefsJSON unmarshals JSON into Prefs. Unknown fields are rejected.
func ParsePrefsJSON(data []byte) (*Prefs, error) {
	var p Prefs
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	// Check for unknown fields by re-decoding with DisallowUnknownFields.
	dec := json.NewDecoder(strings.NewReader(string(data)))
	dec.DisallowUnknownFields()
	var check Prefs
	if err := dec.Decode(&check); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	return &p, nil
}

// ParsePrefsYAML unmarshals YAML into Prefs. Unknown fields are rejected.
// Implemented in yaml_parse.go (no-op if yaml support not compiled).
func ParsePrefsYAML(data []byte) (*Prefs, error) {
	return parsePrefsYAML(data)
}

// nowFunc is overridden in tests.
var nowFunc = time.Now

func timeStr() string {
	return nowFunc().UTC().Format(time.RFC3339)
}
