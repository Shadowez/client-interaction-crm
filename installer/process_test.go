package main

import (
	"bytes"
	"context"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestWindowsNpxUsesNodeEntrypointWithoutShell(t *testing.T) {
	executable := `C:\Program Files\nodejs\npx.cmd`
	password := `Aa1!dollar$_amp&caret^percent%_quote"safe`
	args := []string{"--yes", "supabase@2.117.0", "projects", "create", "NYDP CRM", "--db-password", password}
	gotExecutable, gotArgs := executableArgs("windows", executable, args)
	wantExecutable := `C:\Program Files\nodejs\node.exe`
	wantArgs := append([]string{`C:\Program Files\nodejs\node_modules\npm\bin\npx-cli.js`}, args...)
	if gotExecutable != wantExecutable || !reflect.DeepEqual(gotArgs, wantArgs) {
		t.Fatalf("got executable %q args %#v; want %q %#v", gotExecutable, gotArgs, wantExecutable, wantArgs)
	}
	if strings.Contains(strings.ToLower(gotExecutable), "cmd.exe") {
		t.Fatal("Windows node shim must not cross a command shell")
	}
}

func TestNonWindowsCommandRemainsStructured(t *testing.T) {
	args := []string{"projects", "create", "Name With Spaces"}
	executable, gotArgs := executableArgs("linux", "/opt/Node Runtime/bin/npx", args)
	if executable != "/opt/Node Runtime/bin/npx" || !reflect.DeepEqual(gotArgs, args) {
		t.Fatalf("unexpected command %q %#v", executable, gotArgs)
	}
}

func TestAuthenticationMaterialRedaction(t *testing.T) {
	input := strings.Join([]string{
		"open https://supabase.com/dashboard/cli/login?session_id=session-123&token_name=launcher&public_key=pk-123",
		"verification code: ABCD-1234",
		"access_token=access-123",
		"client_secret: oauth-secret",
		"service_role_key=sb_secret_abcdefghijklmnop",
		"oidc_token=eyJabcdefghijklmnop.qrstuv",
	}, "\n")
	redacted := redactText(input, nil)
	for _, secret := range []string{"session-123", "pk-123", "ABCD-1234", "access-123", "oauth-secret", "sb_secret_abcdefghijklmnop", "eyJabcdefghijklmnop.qrstuv"} {
		if strings.Contains(redacted, secret) {
			t.Errorf("secret %q remained in %q", secret, redacted)
		}
	}
}

func TestSetupLogStartsFresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "setup.log")
	if err := os.WriteFile(path, []byte("old session_id=secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := openSetupLog(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(contents) != 0 {
		t.Fatalf("old log contents survived: %q", contents)
	}
}

func TestInteractiveOutputIsVisibleButNotPersisted(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "setup.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	previousWriter := log.Writer()
	log.SetOutput(logFile)
	t.Cleanup(func() { log.SetOutput(previousWriter) })
	visible := &bytes.Buffer{}
	a := &app{appDir: t.TempDir(), logFile: logFile, out: visible}
	secret := "session_id=must-only-be-visible"
	_, err = a.command(context.Background(), os.Args[0], []string{"-test.run=TestInteractiveLogHelper", "--", secret}, commandOptions{interactive: true, env: []string{"CRM_INTERACTIVE_HELPER=1"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = logFile.Close()
	persisted, _ := os.ReadFile(logPath)
	if !strings.Contains(visible.String(), secret) {
		t.Fatal("interactive authentication material was not shown live")
	}
	if strings.Contains(string(persisted), secret) {
		t.Fatal("interactive authentication material was persisted")
	}
}

func TestInteractiveLogHelper(t *testing.T) {
	if os.Getenv("CRM_INTERACTIVE_HELPER") != "1" {
		return
	}
	os.Stdout.WriteString(strings.Join(os.Args, " "))
	os.Exit(0)
}

func TestNativeWindowsNpxShimExecution(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("native Windows regression test")
	}
	npx, err := exec.LookPath("npx.cmd")
	if err != nil {
		t.Skipf("npx.cmd is not installed: %v", err)
	}
	workDir := filepath.Join(t.TempDir(), "CRM ünicode install with spaces")
	if err := os.MkdirAll(workDir, 0o755); err != nil {
		t.Fatal(err)
	}
	logFile, err := os.Create(filepath.Join(workDir, "setup.log"))
	if err != nil {
		t.Fatal(err)
	}
	defer logFile.Close()
	a := &app{appDir: workDir, logFile: logFile, out: &bytes.Buffer{}}
	output, err := a.command(context.Background(), npx, []string{"--version"}, commandOptions{capture: true})
	if err != nil || strings.TrimSpace(output) == "" {
		t.Fatalf("npx.cmd through node entrypoint failed: output %q, error %v", output, err)
	}
}
