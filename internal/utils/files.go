package utils

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// EnsureProjectDir ensures the provided directory exists.
func EnsureProjectDir(dir string) error {
	return os.MkdirAll(dir, 0o755)
}

// SafeWriteFile writes data to a temp file and atomically renames it into place.
func SafeWriteFile(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("atomic rename: %w", err)
	}
	return nil
}

// PrettyJSON marshals a value as indented JSON.
func PrettyJSON(v any) ([]byte, error) {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return b, nil
}

// FindProjectRoot attempts to find a directory containing a project.json by walking up.
// If the input path is a file, it starts from its directory.
func FindProjectRoot(start string) (string, error) {
	if start == "" {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		start = wd
	}
	info, err := os.Stat(start)
	if err != nil {
		return "", err
	}
	dir := start
	if !info.IsDir() {
		dir = filepath.Dir(start)
	}
	for {
		candidate := filepath.Join(dir, "project.json")
		if _, err := os.Stat(candidate); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir { // reached filesystem root
			break
		}
		dir = parent
	}
	return "", errors.New("project root not found (project.json)")
}
