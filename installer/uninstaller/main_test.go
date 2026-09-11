package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCancellationBeforeFinalConfirmationChangesNothing(t *testing.T) {
	base := t.TempDir()
	configureTestUserDirs(t, base)
	root := createOwnedInstall(t, filepath.Join(base, "installed-crm"))
	manifestPath, _ := defaultManifestPath()
	_, local, _ := crmDataDirs()
	manifest := installManifest{
		Version: version, InstallationDirectory: root,
		NPMCacheDirectory:  filepath.Join(local, "npm-cache"),
		AppDataDirectories: []string{filepath.Join(base, "roaming", "ClientInteractionCRM"), local},
		CRMInfoPath:        filepath.Join(root, "CRM-INFO.txt"),
		UninstallerPath:    filepath.Join(local, "Uninstall Client Interaction CRM.exe"),
	}
	writeTestManifest(t, manifestPath, manifest)
	output := &bytes.Buffer{}
	if err := runWithManifest(strings.NewReader("\n\n\n\n"), output, manifestPath); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "app", ".crm-version")); err != nil {
		t.Fatalf("cancelled uninstall changed installation: %v", err)
	}
	for _, expected := range []string{"Remove the local CRM installation", "Sign out of Supabase CLI [y/N]", "Sign out of Vercel CLI [y/N]", "Proceed [y/N]", "Online Supabase/Vercel projects will NOT be deleted", "Uninstall cancelled"} {
		if !strings.Contains(output.String(), expected) {
			t.Fatalf("output missing %q: %s", expected, output.String())
		}
	}
}

func TestManifestCannotRedirectCleanup(t *testing.T) {
	base := t.TempDir()
	configureTestUserDirs(t, base)
	root := createOwnedInstall(t, filepath.Join(base, "installed-crm"))
	manifestPath, _ := defaultManifestPath()
	_, local, _ := crmDataDirs()
	manifest := installManifest{
		Version: version, InstallationDirectory: root,
		NPMCacheDirectory:  filepath.Join(base, "unrelated-npm-cache"),
		AppDataDirectories: []string{local}, CRMInfoPath: filepath.Join(root, "CRM-INFO.txt"),
		UninstallerPath: filepath.Join(local, "Uninstall Client Interaction CRM.exe"),
	}
	writeTestManifest(t, manifestPath, manifest)
	err := validateManifestOwnership(manifest, manifestPath)
	if err == nil || !strings.Contains(err.Error(), "non-CRM") {
		t.Fatalf("unsafe manifest accepted: %v", err)
	}
}

func TestProtectedAndUnmarkedRootsAreRejected(t *testing.T) {
	home, _ := os.UserHomeDir()
	if err := validateInstallRoot(home); err == nil {
		t.Fatal("user profile root was accepted")
	}
	if err := validateInstallRoot(t.TempDir()); err == nil {
		t.Fatal("unmarked directory was accepted")
	}
}

func TestPreviewKeepsLoginsByDefault(t *testing.T) {
	output := &bytes.Buffer{}
	printPreview(output, installManifest{InstallationDirectory: `E:\CRM`}, choices{local: true})
	if !strings.Contains(output.String(), "Supabase login: keep") || !strings.Contains(output.String(), "Vercel login: keep") {
		t.Fatalf("default preview: %s", output.String())
	}
}

func TestLocalCleanupLeavesUnrelatedPathsUntouched(t *testing.T) {
	base := t.TempDir()
	configureTestUserDirs(t, base)
	root := createOwnedInstall(t, filepath.Join(base, "installed-crm"))
	roaming, local, _ := crmDataDirs()
	if err := os.MkdirAll(roaming, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(local, 0o700); err != nil {
		t.Fatal(err)
	}
	unrelated := filepath.Join(base, "unrelated", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(unrelated), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unrelated, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	manifest := installManifest{InstallationDirectory: root}
	if err := removeLocalFiles(manifest); err != nil {
		t.Fatal(err)
	}
	if exists(root) || exists(roaming) {
		t.Fatal("owned installation or roaming data remained")
	}
	if !exists(unrelated) {
		t.Fatal("unrelated data was removed")
	}
}

func TestSystemNodeCanNeverBeManifestOwnedRuntime(t *testing.T) {
	base := t.TempDir()
	configureTestUserDirs(t, base)
	root := createOwnedInstall(t, filepath.Join(base, "installed-crm"))
	manifestPath, _ := defaultManifestPath()
	_, local, _ := crmDataDirs()
	manifest := installManifest{
		Version: version, InstallationDirectory: root,
		RuntimeDirectories: []string{filepath.Join(string(os.PathSeparator), "Program Files", "nodejs")},
		NPMCacheDirectory:  filepath.Join(local, "npm-cache"), AppDataDirectories: []string{local},
		CRMInfoPath: filepath.Join(root, "CRM-INFO.txt"), UninstallerPath: filepath.Join(local, "Uninstall Client Interaction CRM.exe"),
	}
	if err := validateManifestOwnership(manifest, manifestPath); err == nil || !strings.Contains(err.Error(), "non-CRM runtime") {
		t.Fatalf("system runtime was accepted: %v", err)
	}
}

func configureTestUserDirs(t *testing.T, base string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Setenv("APPDATA", filepath.Join(base, "roaming"))
		t.Setenv("LOCALAPPDATA", filepath.Join(base, "local"))
		t.Setenv("USERPROFILE", filepath.Join(base, "profile"))
	} else {
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(base, "roaming"))
		t.Setenv("XDG_CACHE_HOME", filepath.Join(base, "local"))
		t.Setenv("HOME", filepath.Join(base, "profile"))
	}
}

func createOwnedInstall(t *testing.T, root string) string {
	t.Helper()
	app := filepath.Join(root, "app")
	if err := os.MkdirAll(app, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, ".crm-version"), []byte(version+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "package.json"), []byte(`{"name":"client-interaction-crm"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".crm-state.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(root)
}

func writeTestManifest(t *testing.T, path string, manifest installManifest) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(manifest)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}
