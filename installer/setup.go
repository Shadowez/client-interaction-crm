package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type project struct {
	ID        string `json:"id"`
	Ref       string `json:"ref"`
	Name      string `json:"name"`
	CreatedAt string `json:"created_at"`
}

type setupState struct {
	Version            string  `json:"version"`
	Supabase           project `json:"supabase_project"`
	SupabaseConfigured bool    `json:"supabase_configured,omitempty"`
	VercelURL          string  `json:"vercel_url,omitempty"`
	DeploymentURL      string  `json:"deployment_url,omitempty"`
	DeploymentID       string  `json:"deployment_id,omitempty"`
	DeploymentReady    bool    `json:"deployment_ready,omitempty"`
}

func (p project) reference() string {
	if p.Ref != "" {
		return p.Ref
	}
	return p.ID
}

func nodePathEnvironment(bin string) string {
	return "PATH=" + bin + string(os.PathListSeparator) + os.Getenv("PATH")
}

func (a *app) brand(_ context.Context) error {
	if a.resuming {
		if existing, ok := readBranding(filepath.Join(a.appDir, "src", "config", "branding.json")); ok {
			label := displayCompany(existing["organizationName"])
			fmt.Fprintf(a.out, "Existing branding found: %s\n", label)
			reuse, err := a.confirm("Reuse this branding?", true)
			if err != nil {
				return err
			}
			if reuse {
				a.company = existing["organizationName"]
				fmt.Fprintln(a.out, "Reusing the existing organization name and logo.")
				return nil
			}
		}
	}
	company, err := a.ask("Organization / company name (optional)", "")
	if err != nil {
		return err
	}
	logo, err := a.ask("Logo file (optional: PNG, JPG, JPEG, WEBP, or SVG)", "")
	if err != nil {
		return err
	}
	a.company = company
	logoPath := ""
	brandingDir := filepath.Join(a.appDir, "public", "branding")
	if err := os.MkdirAll(brandingDir, 0o755); err != nil {
		return err
	}
	for _, ext := range []string{".png", ".jpg", ".jpeg", ".webp", ".svg"} {
		if err := os.Remove(filepath.Join(brandingDir, "logo"+ext)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if logo != "" {
		ext := strings.ToLower(filepath.Ext(logo))
		allowed := map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".svg": true}
		if !allowed[ext] {
			return fmt.Errorf("unsupported logo format %q", ext)
		}
		source, err := os.Open(logo)
		if err != nil {
			return fmt.Errorf("open logo: %w", err)
		}
		defer source.Close()
		destination, err := os.OpenFile(filepath.Join(brandingDir, "logo"+ext), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			return err
		}
		if _, err := ioCopy(destination, source); err != nil {
			destination.Close()
			return err
		}
		if err := destination.Close(); err != nil {
			return err
		}
		logoPath = "branding/logo" + ext
	}
	config := map[string]string{"productName": "Client Interaction CRM", "organizationName": company, "logoPath": logoPath}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(a.appDir, "src", "config", "branding.json"), append(data, '\n'), 0o644)
}

func readBranding(path string) (map[string]string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var config map[string]string
	if json.Unmarshal(data, &config) != nil {
		return nil, false
	}
	_, hasOrganization := config["organizationName"]
	return config, hasOrganization
}

func ioCopy(destination *os.File, source *os.File) (int64, error) {
	return destination.ReadFrom(source)
}

func (a *app) connectSupabase(ctx context.Context) error {
	fmt.Fprintln(a.out, "Client Interaction CRM uses Supabase for secure authentication and database storage.")
	fmt.Fprintln(a.out, "Already have a Supabase account? Sign in in the browser.")
	fmt.Fprintln(a.out, "New to Supabase? Create an account in the browser, then continue.")
	fmt.Fprintln(a.out, "This installer never asks for your Supabase account password.")
	fmt.Fprintln(a.out, "\nPreparing Supabase connection tools. First-time setup may take several minutes.")
	projects, err := a.projects(ctx)
	if err != nil {
		fmt.Fprintln(a.out, "\nOpening Supabase sign-in…")
		fmt.Fprintln(a.out, "If the browser does not open automatically, use the sign-in link shown below.")
		if _, loginErr := a.runSupabase(ctx, []string{"login"}, commandOptions{interactive: true}); loginErr != nil {
			return loginErr
		}
		projects, err = a.projects(ctx)
		if err != nil {
			return fmt.Errorf("verify Supabase sign-in: %w", err)
		}
	}
	if saved, stateErr := a.loadState(); stateErr == nil && saved.Version == version && saved.Supabase.reference() != "" {
		for _, candidate := range projects {
			if candidate.reference() == saved.Supabase.reference() {
				fmt.Fprintf(a.out, "Resuming with the previously selected dedicated Supabase project %s.\n", candidate.Name)
				a.project = candidate
				a.supabaseConfigured = saved.SupabaseConfigured || a.hasConfiguredSupabase(candidate.reference())
				a.finalURL = saved.VercelURL
				a.deploymentURL = normalizeWebURL(saved.DeploymentURL)
				a.deploymentID = saved.DeploymentID
				a.deploymentReady = saved.DeploymentReady && a.deploymentURL != ""
				return nil
			}
		}
	}

	useExisting := false
	if len(projects) > 0 {
		useExisting, err = a.confirm("Advanced: use an existing dedicated Supabase project instead of creating a new one?", false)
		if err != nil {
			return err
		}
	}
	if useExisting {
		for i, p := range projects {
			fmt.Fprintf(a.out, "  %d. %s (%s)\n", i+1, p.Name, p.reference())
		}
		choiceText, err := a.ask("Project number", "")
		if err != nil {
			return err
		}
		var choice int
		if _, err := fmt.Sscanf(choiceText, "%d", &choice); err != nil || choice < 1 || choice > len(projects) {
			return fmt.Errorf("invalid project selection")
		}
		confirmed, err := a.confirm("This applies CRM migrations and Auth settings. Confirm it is a dedicated project safe to modify", false)
		if err != nil || !confirmed {
			return errorsNew("existing project was not confirmed")
		}
		a.project = projects[choice-1]
		return a.saveState()
	}

	name := displayCompany(a.company)
	if a.company != "" {
		name += " CRM"
	}
	name, err = a.ask("New dedicated Supabase project name", name)
	if err != nil {
		return err
	}
	for _, candidate := range projects {
		if candidate.Name == name && candidate.reference() != "" {
			fmt.Fprintf(a.out, "A Supabase project named %q already exists. To prevent a duplicate, setup will not create another silently.\n", name)
			useMatch, confirmErr := a.confirm("Use this existing project after confirming it is dedicated to this CRM", false)
			if confirmErr != nil {
				return confirmErr
			}
			if !useMatch {
				return errorsNew("choose a different project name or explicitly select the existing dedicated project")
			}
			a.project = candidate
			return a.saveState()
		}
	}
	return a.provisionNewProject(ctx, name)
}

func (a *app) projects(ctx context.Context) ([]project, error) {
	out, err := a.runSupabase(ctx, []string{"projects", "list", "--output", "json"}, commandOptions{capture: true, progressMessage: "Still preparing Supabase tools…"})
	if err != nil {
		return nil, err
	}
	start := strings.Index(out, "[")
	if start < 0 {
		return nil, fmt.Errorf("Supabase returned no project list")
	}
	var projects []project
	if out[start] == '[' {
		if err := json.Unmarshal([]byte(out[start:]), &projects); err != nil {
			return nil, fmt.Errorf("parse Supabase projects: %w", err)
		}
	} else {
		var wrapper struct {
			Projects []project `json:"projects"`
		}
		if err := json.Unmarshal([]byte(out[start:]), &wrapper); err != nil {
			return nil, fmt.Errorf("parse Supabase projects: %w", err)
		}
		projects = wrapper.Projects
	}
	return projects, nil
}

func (a *app) configureSupabase(ctx context.Context) error {
	ref := a.project.reference()
	if ref == "" {
		return fmt.Errorf("missing Supabase project reference")
	}
	if a.supabaseConfigured || a.hasConfiguredSupabase(ref) {
		a.supabaseConfigured = true
		if err := a.saveState(); err != nil {
			return err
		}
		fmt.Fprintln(a.out, "Supabase database, Auth settings, and browser-safe API configuration are already complete; resuming at Vercel.")
		return nil
	}
	commands := [][]string{
		{"link", "--project-ref", ref, "--yes"},
		{"db", "push", "--linked", "--skip-vault", "--yes"},
		{"config", "push", "--project-ref", ref, "--yes"},
	}
	messages := []string{
		"Connecting the new CRM database…",
		"Creating the CRM database tables and security rules…",
		"Applying secure sign-in settings…",
	}
	databaseEnv := []string{}
	if a.databasePassword != "" {
		databaseEnv = append(databaseEnv, "SUPABASE_DB_PASSWORD="+a.databasePassword)
	}
	for index, args := range commands {
		fmt.Fprintln(a.out, messages[index])
		if _, err := a.runSupabase(ctx, args, commandOptions{env: databaseEnv, progressMessage: "Still working with Supabase…"}); err != nil {
			return err
		}
	}
	out, err := a.runSupabase(ctx, []string{"projects", "api-keys", "--project-ref", ref, "--output", "json"}, commandOptions{capture: true, sensitive: true})
	if err != nil {
		return err
	}
	key := findPublishableKey(out)
	if key == "" {
		return fmt.Errorf("Supabase did not return a browser-safe publishable key")
	}
	content := fmt.Sprintf("# Generated by Client Interaction CRM Setup\nVITE_SUPABASE_URL=https://%s.supabase.co\nVITE_SUPABASE_PUBLISHABLE_KEY=%s\n", ref, key)
	if err := os.WriteFile(filepath.Join(a.appDir, ".env.local"), []byte(content), 0o600); err != nil {
		return err
	}
	a.supabaseConfigured = true
	return a.saveState()
}

func (a *app) hasConfiguredSupabase(ref string) bool {
	data, err := os.ReadFile(filepath.Join(a.appDir, ".env.local"))
	if err != nil {
		return false
	}
	values := parseEnv(string(data))
	return values["VITE_SUPABASE_URL"] == "https://"+ref+".supabase.co" && strings.HasPrefix(values["VITE_SUPABASE_PUBLISHABLE_KEY"], "sb_publishable_")
}

func findPublishableKey(payload string) string {
	var value any
	start := strings.IndexAny(payload, "[{")
	if start < 0 || json.Unmarshal([]byte(payload[start:]), &value) != nil {
		return ""
	}
	return walkForKey(value)
}

func walkForKey(value any) string {
	switch item := value.(type) {
	case []any:
		for _, child := range item {
			if found := walkForKey(child); found != "" {
				return found
			}
		}
	case map[string]any:
		for _, key := range []string{"api_key", "key", "value"} {
			if text, ok := item[key].(string); ok && strings.HasPrefix(text, "sb_publishable_") {
				return text
			}
		}
		for _, child := range item {
			if found := walkForKey(child); found != "" {
				return found
			}
		}
	case string:
		if strings.HasPrefix(item, "sb_publishable_") {
			return item
		}
	}
	return ""
}

func (a *app) connectVercel(ctx context.Context) error {
	fmt.Fprintln(a.out, "Client Interaction CRM uses Vercel to host the web interface.")
	fmt.Fprintln(a.out, "Already have a Vercel account? Sign in in the browser.")
	fmt.Fprintln(a.out, "New to Vercel? Create an account in the browser, then continue.")
	fmt.Fprintln(a.out, "This installer never asks for your Vercel password.")
	fmt.Fprintln(a.out, "\nPreparing Vercel deployment tools…")
	fmt.Fprintln(a.out, "First-time setup may take several minutes. This is normal.")
	if _, err := a.runVercel(ctx, []string{"whoami"}, commandOptions{capture: true, progressMessage: "Still preparing Vercel tools…"}); err != nil {
		fmt.Fprintln(a.out, "\nOpening Vercel sign-in…")
		fmt.Fprintln(a.out, "If the browser does not open automatically, use the sign-in link or code shown below.")
		log.Printf("vercel login: entering interactive device flow")
		if _, loginErr := a.runVercel(ctx, []string{"login"}, commandOptions{interactive: true}); loginErr != nil {
			log.Printf("vercel login: exited without authenticated session: %v", loginErr)
			return loginErr
		}
		log.Printf("vercel login: command completed; verifying authentication")
		if _, verifyErr := a.runVercel(ctx, []string{"whoami"}, commandOptions{capture: true}); verifyErr != nil {
			log.Printf("vercel login: authentication verification failed: %v", verifyErr)
			return fmt.Errorf("verify Vercel sign-in: %w", verifyErr)
		}
		log.Printf("vercel login: authentication succeeded; next stage=Vercel project deployment")
	} else {
		log.Printf("vercel authentication: existing session confirmed; next stage=Vercel project deployment")
	}
	return nil
}

func (a *app) deploy(ctx context.Context) error {
	projectName := vercelProjectName(displayCompany(a.company))
	if a.deploymentReady && a.deploymentURL != "" {
		fmt.Fprintf(a.out, "Resuming verification of the successful Vercel production deployment %s.\n", a.deploymentURL)
		log.Printf("vercel deployment: reusing saved ready deployment id=%q url=%q", a.deploymentID, a.deploymentURL)
	} else {
		fmt.Fprintln(a.out, "Creating or reusing the Vercel project for this CRM…")
		if _, err := a.runVercel(ctx, []string{"link", "--yes", "--project", projectName}, commandOptions{progressMessage: "Still preparing the Vercel project…"}); err != nil {
			return err
		}
		envData, err := os.ReadFile(filepath.Join(a.appDir, ".env.local"))
		if err != nil {
			return err
		}
		values := parseEnv(string(envData))
		for _, name := range []string{"VITE_SUPABASE_URL", "VITE_SUPABASE_PUBLISHABLE_KEY"} {
			value := values[name]
			if value == "" {
				return fmt.Errorf("missing %s", name)
			}
			if _, err := a.runVercel(ctx, []string{"env", "add", name, "production", "--force"}, commandOptions{stdin: value + "\n"}); err != nil {
				return err
			}
		}
		fmt.Fprintln(a.out, "Deploying the CRM web interface. This can take several minutes.")
		out, err := a.runVercel(ctx, []string{"deploy", "--prod", "--yes"}, commandOptions{capture: true, progressMessage: "Still deploying the CRM web interface…"})
		if err != nil {
			return err
		}
		result, parseErr := parseVercelDeployResult(out)
		if parseErr != nil {
			return parseErr
		}
		a.deploymentURL = result.URL
		a.deploymentID = result.ID
		a.deploymentReady = result.Ready
		// Persist the successful deployment before the separate inspection step,
		// so an interrupted verification resumes without another deployment.
		if err := a.saveState(); err != nil {
			return err
		}
	}
	fmt.Fprintln(a.out, "Verifying that the production deployment is ready…")
	inspectOutput, err := a.runVercel(ctx, deploymentInspectArgs(a.deploymentURL), commandOptions{capture: true, progressMessage: "Still waiting for Vercel to finish the deployment…"})
	if err != nil {
		return err
	}
	metadata, err := parseDeploymentMetadata(inspectOutput)
	if err != nil {
		return err
	}
	if metadata.Failed {
		return errorsNew("Vercel reported that the production deployment failed. See the troubleshooting log, then safely run the installer again using the same installation folder")
	}
	if !metadata.Ready {
		return errorsNew("Vercel did not confirm that the production deployment is ready. See the troubleshooting log, then safely run the installer again using the same installation folder")
	}
	aliases := metadata.Aliases
	if len(aliases) == 0 {
		if projectOutput, projectErr := a.runVercel(ctx, []string{"project", "inspect", projectName, "--json", "--yes"}, commandOptions{capture: true}); projectErr == nil {
			if projectMetadata, parseErr := parseDeploymentMetadata(projectOutput); parseErr == nil {
				candidate := selectCanonicalURL(a.deploymentURL, projectMetadata.Aliases)
				if candidate != a.deploymentURL {
					if aliasOutput, aliasErr := a.runVercel(ctx, []string{"inspect", candidate, "--json"}, commandOptions{capture: true}); aliasErr == nil {
						if aliasMetadata, aliasParseErr := parseDeploymentMetadata(aliasOutput); aliasParseErr == nil && aliasMetadata.Ready && metadata.ID != "" && aliasMetadata.ID == metadata.ID {
							aliases = []string{candidate}
						}
					}
				}
			}
		}
	}
	a.finalURL = selectCanonicalURL(a.deploymentURL, aliases)
	if a.finalURL == "" {
		return errorsNew("Vercel did not return a usable production URL")
	}
	if err := a.saveState(); err != nil {
		return err
	}
	if err := writeAuthConfig(filepath.Join(a.appDir, "supabase", "config.toml"), a.finalURL); err != nil {
		return err
	}
	if _, err := a.runSupabase(ctx, []string{"config", "push", "--project-ref", a.project.reference(), "--yes"}, commandOptions{progressMessage: "Still updating secure sign-in settings…"}); err != nil {
		fmt.Fprintf(a.out, "Automatic Auth URL update failed. Set Site URL to %s and add %s/** in Supabase Dashboard.\n", a.finalURL, strings.TrimRight(a.finalURL, "/"))
		_ = a.openURL(ctx, "https://supabase.com/dashboard/project/"+a.project.reference()+"/auth/url-configuration")
		if ok, askErr := a.confirm("Continue after saving the Auth URL settings", true); askErr != nil || !ok {
			return fmt.Errorf("Supabase Auth URL configuration remains incomplete")
		}
	}
	dashboardURL := "https://supabase.com/dashboard/project/" + a.project.reference() + "/auth/users"
	fmt.Fprintln(a.out, "\nYour CRM infrastructure is ready.")
	fmt.Fprintln(a.out, "One final account step is required.")
	fmt.Fprintln(a.out, "Open Supabase Dashboard → Authentication → Users and create or invite the first CRM user.")
	fmt.Fprintf(a.out, "Dashboard: %s\n", dashboardURL)
	_ = a.openURL(ctx, dashboardURL)
	if _, err := a.ask("Press Enter after you have completed this step", ""); err != nil {
		return err
	}
	return nil
}

func (a *app) statePath() string { return filepath.Join(a.root, ".crm-state.json") }

func (a *app) loadState() (setupState, error) {
	var state setupState
	data, err := os.ReadFile(a.statePath())
	if err != nil {
		return state, err
	}
	err = json.Unmarshal(data, &state)
	return state, err
}

func (a *app) saveState() error {
	state := setupState{
		Version: version, Supabase: a.project, SupabaseConfigured: a.supabaseConfigured,
		VercelURL: a.finalURL, DeploymentURL: a.deploymentURL, DeploymentID: a.deploymentID, DeploymentReady: a.deploymentReady,
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(a.statePath(), append(data, '\n'), 0o600)
}

func parseEnv(content string) map[string]string {
	result := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		if key, value, ok := strings.Cut(line, "="); ok {
			result[strings.TrimSpace(key)] = strings.TrimSpace(value)
		}
	}
	return result
}

func vercelProjectName(value string) string {
	value = strings.ToLower(value)
	value = regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		return "client-interaction-crm"
	}
	if len(value) > 80 {
		value = strings.Trim(value[:80], "-")
	}
	return value
}

func lastHTTPSURL(value string) string {
	re := regexp.MustCompile(`https://[^\s"'<>\\]+`)
	matches := re.FindAllString(value, -1)
	if len(matches) == 0 {
		return ""
	}
	return normalizeWebURL(strings.TrimRight(matches[len(matches)-1], ").,;]}"))
}

func writeAuthConfig(path, siteURL string) error {
	base := strings.TrimRight(siteURL, "/")
	content := fmt.Sprintf("project_id = \"client-interaction-crm\"\n\n[auth]\nenabled = true\nsite_url = %q\nadditional_redirect_urls = [\n  \"http://localhost:5173/**\",\n  \"http://127.0.0.1:5173/**\",\n  %q\n]\nenable_signup = false\n\n[auth.email]\nenable_signup = true\ndouble_confirm_changes = true\nenable_confirmations = false\nsecure_password_change = true\n", base, base+"/**")
	return os.WriteFile(path, []byte(content), 0o644)
}
