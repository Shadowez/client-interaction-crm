package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallManifestContainsOwnershipWithoutSecrets(t *testing.T) {
	manifest := installManifest{
		Version: version, InstallationDirectory: `E:\CRM`,
		RuntimeDirectories:    []string{`E:\CRM\.crm-runtime\node`},
		NPMCacheDirectory:     `C:\Users\test\AppData\Local\ClientInteractionCRM\npm-cache`,
		AppDataDirectories:    []string{`C:\Users\test\AppData\Roaming\ClientInteractionCRM`, `C:\Users\test\AppData\Local\ClientInteractionCRM`},
		PortableNodeInstalled: true,
	}
	data, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	text := strings.ToLower(string(data))
	for _, forbidden := range []string{"password", "access_token", "refresh_token", "service_role", "oidc_token", "login_code"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("manifest contains secret field %q: %s", forbidden, text)
		}
	}
}

func TestCRMDataDirectoriesAreProductScoped(t *testing.T) {
	roaming, local, err := crmDataDirs()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(roaming) != "ClientInteractionCRM" || filepath.Base(local) != "ClientInteractionCRM" {
		t.Fatalf("unexpected CRM data directories: %q %q", roaming, local)
	}
	cache, err := crmNPMCacheDir()
	if err != nil || filepath.Base(cache) != "npm-cache" || filepath.Base(filepath.Dir(cache)) != "ClientInteractionCRM" {
		t.Fatalf("unexpected CRM npm cache %q: %v", cache, err)
	}
}

func TestUninstallerReleaseAndInstalledNamesAreExplicit(t *testing.T) {
	if uninstallerAsset != "Uninstall Client Interaction CRM.exe" {
		t.Fatalf("installed uninstaller name %q", uninstallerAsset)
	}
	if uninstallerReleaseAsset != "Uninstall.Client.Interaction.CRM.exe" {
		t.Fatalf("GitHub-normalized asset name %q", uninstallerReleaseAsset)
	}
}

func TestLocalUninstallerOverrideWithSidecarChecksum(t *testing.T) {
	source := filepath.Join(t.TempDir(), "Uninstall Client Interaction CRM.exe")
	contents := []byte("actions-built-uninstaller")
	if err := os.WriteFile(source, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(contents))
	if err := os.WriteFile(source+".sha256", []byte(hash+"  "+filepath.Base(source)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRM_UNINSTALLER_FILE", source)
	t.Setenv("CRM_UNINSTALLER_SHA256", "")
	destination := filepath.Join(t.TempDir(), "installed.exe")
	if err := (&app{}).acquireUninstaller(context.Background(), destination); err != nil {
		t.Fatal(err)
	}
	installed, err := os.ReadFile(destination)
	if err != nil || string(installed) != string(contents) {
		t.Fatalf("installed uninstaller mismatch: %v", err)
	}
}

func TestLocalUninstallerOverrideExplicitChecksum(t *testing.T) {
	source := filepath.Join(t.TempDir(), "uninstaller.exe")
	contents := []byte("release-uninstaller")
	if err := os.WriteFile(source, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRM_UNINSTALLER_FILE", source)
	t.Setenv("CRM_UNINSTALLER_SHA256", fmt.Sprintf("%x", sha256.Sum256(contents)))
	if err := (&app{}).acquireUninstaller(context.Background(), filepath.Join(t.TempDir(), "installed.exe")); err != nil {
		t.Fatal(err)
	}
}

func TestLocalUninstallerOverrideRejectsMismatchAndMissingFile(t *testing.T) {
	source := filepath.Join(t.TempDir(), "uninstaller.exe")
	if err := os.WriteFile(source, []byte("wrong"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRM_UNINSTALLER_FILE", source)
	t.Setenv("CRM_UNINSTALLER_SHA256", strings.Repeat("0", 64))
	if err := (&app{}).acquireUninstaller(context.Background(), filepath.Join(t.TempDir(), "installed.exe")); err == nil || !strings.Contains(err.Error(), "checksum verification failed") {
		t.Fatalf("expected checksum failure, got %v", err)
	}
	t.Setenv("CRM_UNINSTALLER_FILE", filepath.Join(t.TempDir(), "missing.exe"))
	if err := (&app{}).acquireUninstaller(context.Background(), filepath.Join(t.TempDir(), "installed.exe")); err == nil || !strings.Contains(err.Error(), "CRM_UNINSTALLER_FILE") {
		t.Fatalf("expected missing override failure, got %v", err)
	}
}

func TestPublicUninstallerDownloadURLsRemainVersionedAndUnauthenticated(t *testing.T) {
	asset, checksum := releaseUninstallerURLs("1.1.0")
	want := "https://github.com/Shadowez/client-interaction-crm/releases/download/v1.1.0/Uninstall.Client.Interaction.CRM.exe"
	if asset != want || checksum != want+".sha256" {
		t.Fatalf("unexpected public release URLs: %q %q", asset, checksum)
	}
	if strings.Contains(strings.ToLower(asset), "token") {
		t.Fatalf("release URL must not contain authentication: %q", asset)
	}
}

func TestPayloadAndUninstallerOverridesCanCoexist(t *testing.T) {
	env := map[string]string{
		"CRM_PAYLOAD_FILE":     `C:\acceptance\payload.zip`,
		"CRM_UNINSTALLER_FILE": `C:\acceptance\Uninstall Client Interaction CRM.exe`,
	}
	getenv := func(key string) string { return env[key] }
	if _, enabled := localPayloadOverride(getenv); !enabled {
		t.Fatal("payload override was not enabled")
	}
	if _, enabled := localUninstallerOverride(getenv); !enabled {
		t.Fatal("uninstaller override was not enabled")
	}
}
