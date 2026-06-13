package main

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// settingsPath returns the location of the persisted settings file, which lives
// alongside the app's managed dependencies in appDir()
// (e.g. ~/Library/Application Support/ytpgui/settings.json on macOS).
func settingsPath() (string, error) {
	base, err := appDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "settings.json"), nil
}

// LoadSettings returns the raw JSON contents of the settings file, or an empty
// string when no settings have been saved yet. The frontend owns the schema and
// (de)serialization, so this layer stays agnostic to what's stored — which means
// new settings can be added without touching Go. Bound to the frontend.
func (a *App) LoadSettings() (string, error) {
	path, err := settingsPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// A missing file is the normal first-run case, not an error.
		if errors.Is(err, fs.ErrNotExist) {
			return "", nil
		}
		return "", err
	}
	return string(data), nil
}

// SaveSettings writes the given JSON string to the settings file, creating the
// app directory if needed. The write is atomic (temp file + rename) so a crash
// mid-write can't leave a half-written, unparseable settings file behind. Bound
// to the frontend.
func (a *App) SaveSettings(data string) error {
	path, err := settingsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(data), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
