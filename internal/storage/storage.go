// Package storage provides small persistence helpers.
package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// LoadJSON decodes path into target. A missing file leaves target unchanged.
func LoadJSON(path string, target any) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open json file: %w", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(target); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("decode json file: %w", err)
	}

	return nil
}

// SaveJSON writes target as stable, human-readable JSON.
func SaveJSON(path string, target any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create json directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".tmp-*.json")
	if err != nil {
		return fmt.Errorf("create temp json file: %w", err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	encoder := json.NewEncoder(tmp)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(target); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("encode json file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp json file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		if removeErr := os.Remove(path); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
			return fmt.Errorf("replace json file: %w", removeErr)
		}
		if renameErr := os.Rename(tmpPath, path); renameErr != nil {
			return fmt.Errorf("rename json file: %w", renameErr)
		}
	}

	return nil
}
