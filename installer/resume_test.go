package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidInstallDirRequiresVersionMarkerAndApplication(t *testing.T) {
	root := filepath.Join(t.TempDir(), "CRM Install ünicode with spaces")
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if validInstallDir(root) {
		t.Fatal("empty directory must not be offered for resume")
	}
	if err := os.WriteFile(filepath.Join(appDir, ".crm-version"), []byte(version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, "package.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !validInstallDir(root) {
		t.Fatal("prepared installation was not recognized")
	}
}

func TestBrandingCanBeReadForResume(t *testing.T) {
	path := filepath.Join(t.TempDir(), "branding.json")
	if err := os.WriteFile(path, []byte(`{"organizationName":"NYDP","logoPath":"branding/logo.svg"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	config, ok := readBranding(path)
	if !ok || config["organizationName"] != "NYDP" {
		t.Fatalf("branding was not reusable: %#v, %v", config, ok)
	}
}
