package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseReadyDeploymentAndCanonicalAlias(t *testing.T) {
	metadata, err := parseDeploymentMetadata(`{"id":"dpl_123","readyState":"READY","aliases":["fix-price.vercel.app","fix-price-abc-owner.vercel.app"]}`)
	if err != nil || !metadata.Ready || metadata.Failed || metadata.ID != "dpl_123" {
		t.Fatalf("metadata %#v, error %v", metadata, err)
	}
	canonical := selectCanonicalURL("https://fix-price-abc-owner.vercel.app", metadata.Aliases)
	if canonical != "https://fix-price.vercel.app" {
		t.Fatalf("canonical URL %q", canonical)
	}
}

func TestParseStructuredDeployResultDoesNotCaptureJSONQuote(t *testing.T) {
	output := "Vercel CLI 59.15.1\nProduction https://example-crm-build.example-team.vercel.app\n" + `{
  "status": "ok",
  "deployment": {
	"id": "dpl_example_fixture_123456",
	"url": "https://example-crm-build.example-team.vercel.app",
    "readyState": "READY"
  },
  "next": [{"command":"vercel inspect https://example-crm-build.example-team.vercel.app"}]
}`
	result, err := parseVercelDeployResult(output)
	if err != nil {
		t.Fatal(err)
	}
	if result.URL != "https://example-crm-build.example-team.vercel.app" || result.ID != "dpl_example_fixture_123456" || !result.Ready {
		t.Fatalf("deploy result %#v", result)
	}
	if strings.Contains(result.URL, `"`) {
		t.Fatalf("deployment URL contains JSON delimiter: %q", result.URL)
	}
}

func TestHumanDeployFallbackStopsAtStructuredDelimiters(t *testing.T) {
	result, err := parseVercelDeployResult(`result "https://fallback.vercel.app"`)
	if err != nil || result.URL != "https://fallback.vercel.app" {
		t.Fatalf("fallback result %#v, error %v", result, err)
	}
}

func TestVercelEnvironmentSuppressesUnrelatedUpdates(t *testing.T) {
	nonInteractive := vercelEnvironment(`C:\runtime\node`, false)
	if !containsString(nonInteractive, "NO_UPDATE_NOTIFIER=1") || !containsString(nonInteractive, "AI_AGENT=client-interaction-crm-installer") {
		t.Fatalf("non-interactive Vercel environment %#v", nonInteractive)
	}
	interactive := vercelEnvironment(`C:\runtime\node`, true)
	if !containsString(interactive, "NO_UPDATE_NOTIFIER=1") || containsString(interactive, "AI_AGENT=client-interaction-crm-installer") {
		t.Fatalf("interactive login environment %#v", interactive)
	}
	if vercelCLI != "59.15.1" {
		t.Fatalf("Vercel CLI pin changed to %q", vercelCLI)
	}
}

func TestCancelledVercelLoginHasReadableReason(t *testing.T) {
	err := friendlyCommandError("npx", []string{"--yes", "vercel@59.15.1", "login"}, "cancelled", errors.New("exit status 1"))
	if !strings.Contains(err.Error(), "Vercel sign-in was cancelled") || !strings.Contains(err.Error(), "safely run") {
		t.Fatalf("login error %q", err)
	}
}

func TestCanonicalURLPrefersVerifiedCustomDomain(t *testing.T) {
	aliases := []string{"project.vercel.app", "crm.example.com", "https://unique.vercel.app"}
	if got := selectCanonicalURL("https://unique.vercel.app", aliases); got != "https://crm.example.com" {
		t.Fatalf("canonical URL %q", got)
	}
}

func TestCanonicalURLFallsBackToVerifiedDeployment(t *testing.T) {
	deployment := "https://unique-deployment.vercel.app"
	if got := selectCanonicalURL(deployment, nil); got != deployment {
		t.Fatalf("fallback URL %q", got)
	}
}

func TestDeploymentMustBeReady(t *testing.T) {
	metadata, err := parseDeploymentMetadata(`{"status":"ERROR","aliases":["project.vercel.app"]}`)
	if err != nil || metadata.Ready || !metadata.Failed {
		t.Fatalf("metadata %#v, error %v", metadata, err)
	}
}

func TestDeploymentInspectWaitArguments(t *testing.T) {
	want := []string{"inspect", "https://deployment.vercel.app", "--wait", "--timeout", "10m", "--json"}
	if got := deploymentInspectArgs("https://deployment.vercel.app"); !reflect.DeepEqual(got, want) {
		t.Fatalf("inspect arguments %#v", got)
	}
}

func TestNonInteractiveProgressMessage(t *testing.T) {
	if os.Getenv("CRM_TEST_SLOW_HELPER") == "1" {
		time.Sleep(60 * time.Millisecond)
		fmt.Fprintln(os.Stdout, "done")
		os.Exit(0)
	}
	logFile, err := os.Create(filepath.Join(t.TempDir(), "setup.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	output := &bytes.Buffer{}
	a := &app{out: output, logFile: logFile, appDir: t.TempDir()}
	_, err = a.command(context.Background(), os.Args[0], []string{"-test.run=TestNonInteractiveProgressMessage"}, commandOptions{
		capture: true, env: []string{"CRM_TEST_SLOW_HELPER=1"}, progressMessage: "Still working…", progressAfter: 10 * time.Millisecond, progressEvery: 15 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "Still working") {
		t.Fatalf("progress output %q", output)
	}
}

func TestKnownCommandErrors(t *testing.T) {
	limit := friendlyCommandError("npx", []string{"supabase@2.117.0", "projects", "create"}, "Maximum limits for the number of active free projects reached", errors.New("exit status 1"))
	if !strings.Contains(limit.Error(), "active-project limit") || !strings.Contains(limit.Error(), "safely run") {
		t.Fatalf("project-limit message %q", limit)
	}
	network := friendlyCommandError("npx", []string{"vercel@59.15.1", "whoami"}, "getaddrinfo EAI_AGAIN", errors.New("exit status 1"))
	if !strings.Contains(network.Error(), "internet connection") {
		t.Fatalf("network message %q", network)
	}
	deployment := friendlyCommandError("npx", []string{"vercel@59.15.1", "deploy"}, "unknown build error", errors.New("exit status 1"))
	if !strings.Contains(deployment.Error(), "could not deploy") {
		t.Fatalf("deployment message %q", deployment)
	}
}

func TestDeployWaitsAndUsesCanonicalAlias(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appDir, "supabase"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(appDir, ".env.local"), []byte("VITE_SUPABASE_URL=https://project.supabase.co\nVITE_SUPABASE_PUBLISHABLE_KEY=sb_publishable_test\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	output := &bytes.Buffer{}
	var calls [][]string
	a := &app{
		in: bufio.NewReader(strings.NewReader("\n")), out: output, root: root, appDir: appDir,
		project: project{ID: "supabase-ref"}, browserOpen: func(context.Context, string) error { return nil },
	}
	a.vercelRun = func(_ context.Context, args []string, _ commandOptions) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		if len(args) > 0 && args[0] == "deploy" {
			return "https://project-unique-owner.vercel.app", nil
		}
		if len(args) > 1 && args[0] == "inspect" && args[1] == "https://project-unique-owner.vercel.app" {
			return `{"id":"dpl_ready","readyState":"READY","aliases":["project.vercel.app"]}`, nil
		}
		return "", nil
	}
	a.supabaseRun = func(context.Context, []string, commandOptions) (string, error) { return "", nil }
	if err := a.deploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a.finalURL != "https://project.vercel.app" {
		t.Fatalf("final URL %q", a.finalURL)
	}
	if !containsArgs(calls, deploymentInspectArgs("https://project-unique-owner.vercel.app")) {
		t.Fatalf("readiness inspection missing: %#v", calls)
	}
	config, _ := os.ReadFile(filepath.Join(appDir, "supabase", "config.toml"))
	if !strings.Contains(string(config), `site_url = "https://project.vercel.app"`) {
		t.Fatalf("canonical Auth URL missing: %s", config)
	}
	if !strings.Contains(output.String(), "One final account step") || strings.Contains(output.String(), "Continue after creating") {
		t.Fatalf("first-user UX output %q", output)
	}
}

func TestReadyDeploymentResumeDoesNotDeployAgain(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	if err := os.MkdirAll(filepath.Join(appDir, "supabase"), 0o755); err != nil {
		t.Fatal(err)
	}
	a := &app{
		in: bufio.NewReader(strings.NewReader("\n")), out: &bytes.Buffer{}, root: root, appDir: appDir,
		project: project{ID: "supabase-ref"}, deploymentID: "dpl_existing",
		deploymentURL: "https://perfectco-existing.vercel.app", deploymentReady: true,
		browserOpen: func(context.Context, string) error { return nil },
	}
	var calls [][]string
	a.vercelRun = func(_ context.Context, args []string, _ commandOptions) (string, error) {
		calls = append(calls, append([]string(nil), args...))
		if args[0] == "inspect" {
			return `{"id":"dpl_existing","readyState":"READY","aliases":["perfectco.vercel.app"]}`, nil
		}
		return "", fmt.Errorf("unexpected Vercel mutation: %v", args)
	}
	a.supabaseRun = func(context.Context, []string, commandOptions) (string, error) { return "", nil }
	if err := a.deploy(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if call[0] == "deploy" || call[0] == "link" || call[0] == "env" {
			t.Fatalf("resume repeated Vercel mutation: %#v", calls)
		}
	}
	if a.finalURL != "https://perfectco.vercel.app" {
		t.Fatalf("final URL %q", a.finalURL)
	}
}

func containsArgs(calls [][]string, target []string) bool {
	for _, call := range calls {
		if reflect.DeepEqual(call, target) {
			return true
		}
	}
	return false
}
