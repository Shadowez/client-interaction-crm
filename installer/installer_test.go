package main

import (
	"archive/zip"
	"bufio"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCompatibleNode(t *testing.T) {
	cases := map[string]bool{"v20.0.0": true, "v22.23.2\n": true, "v24.1.0": true, "v19.9.0": false, "v25.0.0": false, "garbage": false}
	for input, want := range cases {
		if got := compatibleNode(input); got != want {
			t.Errorf("compatibleNode(%q)=%v want %v", input, got, want)
		}
	}
}

func TestExpectedChecksum(t *testing.T) {
	hash := strings.Repeat("a", 64)
	got, err := expectedChecksum(hash+"  node.zip\n", "node.zip")
	if err != nil || got != hash {
		t.Fatalf("got %q, %v", got, err)
	}
	if _, err := expectedChecksum(hash+"  other.zip\n", "node.zip"); err == nil {
		t.Fatal("expected missing checksum error")
	}
}

func TestVerifyChecksum(t *testing.T) {
	path := filepath.Join(t.TempDir(), "payload")
	if err := os.WriteFile(path, []byte("known"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(path, "7117fff2d0fd294462b3c802b7cb8753579f23f3946b99cf55f38e873f013f10"); err != nil {
		t.Fatal(err)
	}
	if err := verifyChecksum(path, strings.Repeat("0", 64)); err == nil {
		t.Fatal("expected mismatch")
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	archive := filepath.Join(t.TempDir(), "bad.zip")
	f, err := os.Create(archive)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	entry, err := w.Create("../outside")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = entry.Write([]byte("bad"))
	_ = w.Close()
	_ = f.Close()
	if err := extractZip(archive, t.TempDir()); err == nil {
		t.Fatal("expected traversal rejection")
	}
}

func TestLocalPayloadValid(t *testing.T) {
	payload, checksum := testPayload(t, map[string]string{"app/package.json": `{}`})
	if err := os.WriteFile(payload+".sha256", []byte(checksum+"  "+filepath.Base(payload)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRM_PAYLOAD_FILE", payload)
	t.Setenv("CRM_PAYLOAD_SHA256", "")
	a := testPayloadApp(t)
	if err := a.preparePayload(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(a.appDir, "package.json")); err != nil {
		t.Fatal(err)
	}
}

func TestLocalPayloadMissingFile(t *testing.T) {
	t.Setenv("CRM_PAYLOAD_FILE", filepath.Join(t.TempDir(), "missing.zip"))
	t.Setenv("CRM_PAYLOAD_SHA256", strings.Repeat("0", 64))
	if err := testPayloadApp(t).preparePayload(context.Background()); err == nil || !strings.Contains(err.Error(), "CRM_PAYLOAD_FILE") {
		t.Fatalf("expected missing local payload error, got %v", err)
	}
}

func TestLocalPayloadChecksumMismatch(t *testing.T) {
	payload, _ := testPayload(t, map[string]string{"app/package.json": `{}`})
	t.Setenv("CRM_PAYLOAD_FILE", payload)
	t.Setenv("CRM_PAYLOAD_SHA256", strings.Repeat("0", 64))
	if err := testPayloadApp(t).preparePayload(context.Background()); err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("expected checksum mismatch, got %v", err)
	}
}

func TestLocalPayloadMalformedArchive(t *testing.T) {
	payload := filepath.Join(t.TempDir(), "client-interaction-crm-app-v1.1.0.zip")
	if err := os.WriteFile(payload, []byte("not a ZIP"), 0o600); err != nil {
		t.Fatal(err)
	}
	checksum, err := sha256File(payload)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CRM_PAYLOAD_FILE", payload)
	t.Setenv("CRM_PAYLOAD_SHA256", checksum)
	if err := testPayloadApp(t).preparePayload(context.Background()); err == nil {
		t.Fatal("expected malformed archive error")
	}
}

func TestLocalPayloadTraversalAttempt(t *testing.T) {
	payload, checksum := testPayload(t, map[string]string{"app/package.json": `{}`, "../outside": "bad"})
	t.Setenv("CRM_PAYLOAD_FILE", payload)
	t.Setenv("CRM_PAYLOAD_SHA256", checksum)
	if err := testPayloadApp(t).preparePayload(context.Background()); err == nil || !strings.Contains(err.Error(), "unsafe archive path") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
}

func TestGitHubReleasePayloadIsDefault(t *testing.T) {
	t.Setenv("CRM_PAYLOAD_FILE", "")
	t.Setenv("CRM_PAYLOAD_SHA256", strings.Repeat("f", 64))
	if _, enabled := localPayloadOverride(os.Getenv); enabled {
		t.Fatal("local override activated without CRM_PAYLOAD_FILE")
	}
	asset := "client-interaction-crm-app-v1.1.0.zip"
	payloadURL, checksumURL := releasePayloadURLs("1.1.0", asset)
	want := "https://github.com/Shadowez/client-interaction-crm/releases/download/v1.1.0/" + asset
	if payloadURL != want || checksumURL != want+".sha256" {
		t.Fatalf("immutable release URLs changed: %q %q", payloadURL, checksumURL)
	}
}

func TestBrandingWithoutAndWithLogo(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appDir, "src", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	logo := filepath.Join(root, "My Logo.svg")
	if err := os.WriteFile(logo, []byte("<svg/>"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := "Acme Company\n" + logo + "\n"
	a := &app{in: bufioNew(input), out: ioDiscard{}, appDir: appDir}
	if err := a.brand(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(appDir, "public", "branding", "logo.svg")); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(appDir, "src", "config", "branding.json"))
	if !strings.Contains(string(data), "Acme Company") {
		t.Fatalf("branding missing company: %s", data)
	}
}

func TestBrandingDefaultsToMonogram(t *testing.T) {
	appDir := filepath.Join(t.TempDir(), "app")
	if err := os.MkdirAll(filepath.Join(appDir, "src", "config"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := &app{in: bufioNew("\n\n"), out: ioDiscard{}, appDir: appDir}
	if err := a.brand(nil); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(filepath.Join(appDir, "src", "config", "branding.json"))
	if !strings.Contains(string(data), `"logoPath": ""`) || !strings.Contains(string(data), `"organizationName": ""`) {
		t.Fatalf("expected default monogram configuration: %s", data)
	}
}

func TestHelpers(t *testing.T) {
	if got := vercelProjectName("My Café CRM!"); got != "my-caf-crm" {
		t.Errorf("project name %q", got)
	}
	if got := lastHTTPSURL("noise https://one.example\nhttps://two.example."); got != "https://two.example" {
		t.Errorf("url %q", got)
	}
	key := "sb_publishable_test"
	if got := findPublishableKey(`[{"name":"publishable","api_key":"` + key + `"}]`); got != key {
		t.Errorf("key %q", got)
	}
	if got := displayCompany(""); got != "Client Interaction CRM" {
		t.Errorf("company %q", got)
	}
	if runtime.GOOS == "windows" && filepath.Separator != '\\' {
		t.Fatal("unexpected Windows separator")
	}
}

func TestSetupStateRoundTrip(t *testing.T) {
	a := &app{root: t.TempDir(), project: project{Ref: "test-ref", Name: "Disposable CRM"}, finalURL: "https://example.vercel.app"}
	if err := a.saveState(); err != nil {
		t.Fatal(err)
	}
	state, err := a.loadState()
	if err != nil {
		t.Fatal(err)
	}
	if state.Version != version || state.Supabase.Ref != "test-ref" || state.VercelURL != a.finalURL {
		t.Fatalf("unexpected state: %#v", state)
	}
}

// Tiny test adapters keep production prompts tied to standard interfaces.
func bufioNew(value string) *bufio.Reader { return bufio.NewReader(strings.NewReader(value)) }

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }

func testPayloadApp(t *testing.T) *app {
	t.Helper()
	root := t.TempDir()
	return &app{out: ioDiscard{}, root: root, appDir: filepath.Join(root, "app")}
}

func testPayload(t *testing.T, entries map[string]string) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "client-interaction-crm-app-v1.1.0.zip")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for name, content := range entries {
		entry, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	checksum, err := sha256File(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, checksum
}
