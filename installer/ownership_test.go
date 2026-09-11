package main

import (
	"encoding/json"
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
