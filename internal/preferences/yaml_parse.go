package preferences

import (
	"errors"

	"gopkg.in/yaml.v3"
)

func parsePrefsYAML(data []byte) (*Prefs, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	// Reject unknown fields.
	known := map[string]bool{
		"appearance":                true,
		"interfaceDensity":         true,
		"terminalFontSize":         true,
		"transcriptAutoScroll":     true,
		"defaultRightPanelTab":     true,
		"destructiveConfirmations": true,
	}
	for k := range raw {
		if !known[k] {
			return nil, errors.New("unknown field: " + k)
		}
	}

	var p Prefs
	if v, ok := raw["appearance"]; ok {
		if s, ok := v.(string); ok {
			p.Appearance = s
		}
	}
	if v, ok := raw["interfaceDensity"]; ok {
		if s, ok := v.(string); ok {
			p.InterfaceDensity = s
		}
	}
	if v, ok := raw["terminalFontSize"]; ok {
		switch n := v.(type) {
		case int:
			p.TerminalFontSize = n
		case float64:
			p.TerminalFontSize = int(n)
		}
	}
	if v, ok := raw["transcriptAutoScroll"]; ok {
		if b, ok := v.(bool); ok {
			p.TranscriptAutoScroll = b
		}
	}
	if v, ok := raw["defaultRightPanelTab"]; ok {
		if s, ok := v.(string); ok {
			p.DefaultRightPanelTab = s
		}
	}
	if v, ok := raw["destructiveConfirmations"]; ok {
		if b, ok := v.(bool); ok {
			p.DestructiveConfirmations = b
		}
	}
	return &p, nil
}
