package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseOrganizations(t *testing.T) {
	organizations, err := parseOrganizations(`noise [{"id":"org-one","name":"One"},{"slug":"two","name":"Two"},{"name":"Unusable"}]`)
	if err != nil {
		t.Fatal(err)
	}
	if len(organizations) != 2 || organizations[0].reference() != "org-one" || organizations[1].reference() != "two" {
		t.Fatalf("unexpected organizations: %#v", organizations)
	}
}

func TestSelectOneOrganizationAutomatically(t *testing.T) {
	a := provisioningTestApp(t, "")
	a.supabaseRun = staticSupabase(`[ {"id":"org-one","name":"Only Organization"} ]`, nil)
	selected, err := a.selectOrganization(context.Background())
	if err != nil || selected.ID != "org-one" {
		t.Fatalf("selected %#v, error %v", selected, err)
	}
}

func TestSelectMultipleOrganizations(t *testing.T) {
	a := provisioningTestApp(t, "2\n")
	a.supabaseRun = staticSupabase(`[{"id":"one","name":"One"},{"id":"two","name":"Two"}]`, nil)
	selected, err := a.selectOrganization(context.Background())
	if err != nil || selected.ID != "two" {
		t.Fatalf("selected %#v, error %v", selected, err)
	}
}

func TestNoOrganizationsGuidesAndRefreshes(t *testing.T) {
	a := provisioningTestApp(t, "\n")
	calls := 0
	a.supabaseRun = func(_ context.Context, _ []string, _ commandOptions) (string, error) {
		calls++
		if calls == 1 {
			return `[]`, nil
		}
		return `[{"id":"created-org","name":"Created Organization"}]`, nil
	}
	opened := ""
	a.browserOpen = func(_ context.Context, url string) error { opened = url; return nil }
	selected, err := a.selectOrganization(context.Background())
	if err != nil || selected.ID != "created-org" || !strings.Contains(opened, "organizations") {
		t.Fatalf("selected %#v, opened %q, error %v", selected, opened, err)
	}
}

func TestRegionSelectionAndInvalidSelection(t *testing.T) {
	a := provisioningTestApp(t, "99\ninvalid\n2\n")
	selected, err := a.selectRegion()
	if err != nil {
		t.Fatal(err)
	}
	if selected.Code != "us-west-2" {
		t.Fatalf("selected %#v", selected)
	}
	if !strings.Contains(a.out.(*bytes.Buffer).String(), "Enter a number") {
		t.Fatal("invalid selection was not explained")
	}
}

func TestProjectCreateRequiredArgumentsAndSuccess(t *testing.T) {
	a := provisioningTestApp(t, "")
	var gotArgs []string
	a.supabaseRun = func(_ context.Context, args []string, _ commandOptions) (string, error) {
		gotArgs = append([]string(nil), args...)
		return `{"id":"new-project-ref","name":"New CRM"}`, nil
	}
	password := "Aa1!secure-database-password"
	created, err := a.createDedicatedProject(context.Background(), "New CRM", organization{ID: "org-id"}, regionOption{Code: "eu-central-1"}, password)
	if err != nil || created.reference() != "new-project-ref" {
		t.Fatalf("created %#v, error %v", created, err)
	}
	want := []string{"projects", "create", "New CRM", "--org-id", "org-id", "--region", "eu-central-1", "--db-password", password, "--output", "json"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("arguments %#v, want %#v", gotArgs, want)
	}
}

func TestDatabasePasswordPassedByEnvironmentAfterCreation(t *testing.T) {
	a := provisioningTestApp(t, "")
	if err := os.MkdirAll(a.appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	a.project = project{ID: "project-ref"}
	a.databasePassword = "Aa1!secure-database-password"
	var databaseCommands int
	a.supabaseRun = func(_ context.Context, args []string, options commandOptions) (string, error) {
		if len(args) >= 2 && args[0] == "projects" && args[1] == "api-keys" {
			return `[{"api_key":"sb_publishable_test"}]`, nil
		}
		databaseCommands++
		if !containsString(options.env, "SUPABASE_DB_PASSWORD="+a.databasePassword) {
			t.Fatalf("database password environment missing for %#v", args)
		}
		return "", nil
	}
	if err := a.configureSupabase(context.Background()); err != nil {
		t.Fatal(err)
	}
	if databaseCommands != 3 {
		t.Fatalf("expected three configuration commands, got %d", databaseCommands)
	}
}

func TestProjectCreationFailureDoesNotRecordState(t *testing.T) {
	a := provisioningTestApp(t, "\ny\n")
	a.supabaseRun = func(_ context.Context, args []string, _ commandOptions) (string, error) {
		if reflect.DeepEqual(args, []string{"orgs", "list", "--output", "json"}) {
			return `[{"id":"org-id","name":"Organization"}]`, nil
		}
		return "", errors.New("non-interactive project creation failed")
	}
	err := a.provisionNewProject(context.Background(), "Failed CRM")
	if err == nil {
		t.Fatal("expected project creation failure")
	}
	if _, statErr := os.Stat(a.statePath()); !os.IsNotExist(statErr) {
		t.Fatalf("state must not exist after failure: %v", statErr)
	}
	if a.project.reference() != "" {
		t.Fatalf("project was recorded after failure: %#v", a.project)
	}
}

func TestRerunAfterPreCreationAcceptanceFailure(t *testing.T) {
	a := provisioningTestApp(t, "n\nRetry CRM\n\ny\n")
	a.supabaseRun = func(_ context.Context, args []string, _ commandOptions) (string, error) {
		switch strings.Join(args, " ") {
		case "projects list --output json":
			return `[{"id":"unrelated","name":"Unrelated"}]`, nil
		case "orgs list --output json":
			return `[{"id":"org-id","name":"Personal Organization"}]`, nil
		default:
			if len(args) >= 3 && args[0] == "projects" && args[1] == "create" {
				return `{"id":"retry-project","name":"Retry CRM"}`, nil
			}
			return "", errors.New("unexpected command")
		}
	}
	if err := a.connectSupabase(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err := a.loadState()
	if err != nil || state.Supabase.reference() != "retry-project" {
		t.Fatalf("state %#v, error %v", state, err)
	}
}

func TestExistingProjectAdvancedPathStillWorks(t *testing.T) {
	a := provisioningTestApp(t, "y\n1\ny\n")
	a.supabaseRun = staticSupabase(`[{"id":"dedicated","name":"Dedicated CRM"}]`, nil)
	if err := a.connectSupabase(context.Background()); err != nil {
		t.Fatal(err)
	}
	if a.project.reference() != "dedicated" {
		t.Fatalf("unexpected project %#v", a.project)
	}
}

func TestDatabasePasswordValidationAndNoPersistenceOrLogging(t *testing.T) {
	for _, password := range []string{"", "short", "validlengthbut\ninvalid"} {
		if err := validateDatabasePassword(password); err == nil {
			t.Fatalf("password %q should be rejected", password)
		}
	}
	password, err := generateDatabasePassword()
	if err != nil || validateDatabasePassword(password) != nil {
		t.Fatalf("generated password is invalid: %v", err)
	}
	a := provisioningTestApp(t, "")
	a.project = project{ID: "safe-project", Name: "Safe CRM"}
	a.databasePassword = password
	if err := a.saveState(); err != nil {
		t.Fatal(err)
	}
	state, _ := os.ReadFile(a.statePath())
	if strings.Contains(string(state), password) {
		t.Fatal("database password persisted in state")
	}
	loggedArgs := strings.Join(redactArgs([]string{"projects", "create", "CRM", "--db-password", password}), " ")
	if strings.Contains(loggedArgs, password) {
		t.Fatal("database password remained in logged arguments")
	}
	logPath := filepath.Join(t.TempDir(), "setup.log")
	logFile, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	previousWriter := log.Writer()
	log.SetOutput(logFile)
	t.Cleanup(func() { log.SetOutput(previousWriter) })
	commandApp := &app{appDir: t.TempDir(), logFile: logFile, out: &bytes.Buffer{}}
	_, err = commandApp.command(context.Background(), os.Args[0], []string{"-test.run=TestPasswordLogHelper", "--", "--db-password", password}, commandOptions{capture: true, sensitive: true, env: []string{"CRM_TEST_PASSWORD_HELPER=1"}})
	if err != nil {
		t.Fatal(err)
	}
	if err := logFile.Close(); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(logPath)
	if strings.Contains(string(contents), password) {
		t.Fatal("database password appeared in launcher log")
	}
}

func TestPasswordLogHelper(t *testing.T) {
	if os.Getenv("CRM_TEST_PASSWORD_HELPER") != "1" {
		return
	}
	fmt.Fprintln(os.Stdout, strings.Join(os.Args, " "))
	os.Exit(0)
}

func TestParseCreatedProject(t *testing.T) {
	if got := parseCreatedProject(`status\n{"project":{"ref":"wrapped-ref","name":"CRM"}}`); got.reference() != "wrapped-ref" {
		t.Fatalf("unexpected project %#v", got)
	}
}

func provisioningTestApp(t *testing.T, input string) *app {
	t.Helper()
	root := t.TempDir()
	return &app{
		in:     bufio.NewReader(strings.NewReader(input)),
		out:    &bytes.Buffer{},
		root:   root,
		appDir: filepath.Join(root, "app"),
	}
}

func staticSupabase(output string, err error) func(context.Context, []string, commandOptions) (string, error) {
	return func(context.Context, []string, commandOptions) (string, error) { return output, err }
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
