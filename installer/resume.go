package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type installPointer struct {
	Directory string `json:"installation_directory"`
}

func installPointerPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("find per-user config directory: %w", err)
	}
	return filepath.Join(configDir, "ClientInteractionCRM", "last-installation.json"), nil
}

func loadInstallPointer() (string, error) {
	path, err := installPointerPath()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var pointer installPointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return "", err
	}
	if strings.TrimSpace(pointer.Directory) == "" {
		return "", fmt.Errorf("last installation pointer is empty")
	}
	return filepath.Clean(pointer.Directory), nil
}

func saveInstallPointer(directory string) error {
	path, err := installPointerPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(installPointer{Directory: filepath.Clean(directory)}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func validInstallDir(directory string) bool {
	marker, err := os.ReadFile(filepath.Join(directory, "app", ".crm-version"))
	if err != nil || strings.TrimSpace(string(marker)) != version {
		return false
	}
	info, err := os.Stat(filepath.Join(directory, "app", "package.json"))
	return err == nil && info.Mode().IsRegular()
}
