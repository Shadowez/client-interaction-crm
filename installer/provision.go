package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type organization struct {
	ID   string `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

func (o organization) reference() string {
	if o.ID != "" {
		return o.ID
	}
	return o.Slug
}

type regionOption struct {
	Name string
	Code string
}

var supportedRegions = []regionOption{
	{Name: "West US — North California", Code: "us-west-1"},
	{Name: "West US — Oregon", Code: "us-west-2"},
	{Name: "East US — North Virginia", Code: "us-east-1"},
	{Name: "East US — Ohio", Code: "us-east-2"},
	{Name: "Canada — Central", Code: "ca-central-1"},
	{Name: "West EU — Ireland", Code: "eu-west-1"},
	{Name: "West Europe — London", Code: "eu-west-2"},
	{Name: "West EU — Paris", Code: "eu-west-3"},
	{Name: "Central EU — Frankfurt", Code: "eu-central-1"},
	{Name: "Central Europe — Zurich", Code: "eu-central-2"},
	{Name: "North EU — Stockholm", Code: "eu-north-1"},
	{Name: "South Asia — Mumbai", Code: "ap-south-1"},
	{Name: "Southeast Asia — Singapore", Code: "ap-southeast-1"},
	{Name: "Northeast Asia — Tokyo", Code: "ap-northeast-1"},
	{Name: "Northeast Asia — Seoul", Code: "ap-northeast-2"},
	{Name: "Oceania — Sydney", Code: "ap-southeast-2"},
	{Name: "South America — São Paulo", Code: "sa-east-1"},
}

func (a *app) runSupabase(ctx context.Context, args []string, options commandOptions) (string, error) {
	if a.supabaseRun != nil {
		return a.supabaseRun(ctx, args, options)
	}
	all := append([]string{"--yes", "supabase@" + supabaseCLI}, args...)
	options.env = append(options.env, nodePathEnvironment(filepath.Dir(a.node)))
	return a.command(ctx, a.npx, all, options)
}

func (a *app) openURL(ctx context.Context, url string) error {
	if a.browserOpen != nil {
		return a.browserOpen(ctx, url)
	}
	return openBrowser(ctx, url)
}

func parseOrganizations(output string) ([]organization, error) {
	start := strings.IndexAny(output, "[{")
	if start < 0 {
		return nil, fmt.Errorf("Supabase returned no organization JSON")
	}
	var raw any
	if err := json.Unmarshal([]byte(output[start:]), &raw); err != nil {
		return nil, fmt.Errorf("parse Supabase organizations: %w", err)
	}
	var organizations []organization
	switch value := raw.(type) {
	case []any:
		data, _ := json.Marshal(value)
		_ = json.Unmarshal(data, &organizations)
	case map[string]any:
		data, _ := json.Marshal(value["organizations"])
		_ = json.Unmarshal(data, &organizations)
	}
	usable := organizations[:0]
	for _, org := range organizations {
		if org.reference() != "" {
			usable = append(usable, org)
		}
	}
	return usable, nil
}

func (a *app) organizations(ctx context.Context) ([]organization, error) {
	out, err := a.runSupabase(ctx, []string{"orgs", "list", "--output", "json"}, commandOptions{capture: true, progressMessage: "Still checking your Supabase account…"})
	if err != nil {
		return nil, err
	}
	return parseOrganizations(out)
}

func (a *app) selectOrganization(ctx context.Context) (organization, error) {
	organizations, err := a.organizations(ctx)
	if err != nil {
		return organization{}, err
	}
	if len(organizations) == 0 {
		const organizationsURL = "https://supabase.com/dashboard/organizations"
		fmt.Fprintln(a.out, "No Supabase organization is available yet.")
		fmt.Fprintln(a.out, "Create a free organization in the Supabase Dashboard, then return here.")
		fmt.Fprintf(a.out, "Dashboard: %s\n", organizationsURL)
		_ = a.openURL(ctx, organizationsURL)
		ready, askErr := a.confirm("Continue after creating an organization", true)
		if askErr != nil {
			return organization{}, askErr
		}
		if !ready {
			return organization{}, errorsNew("a Supabase organization is required")
		}
		organizations, err = a.organizations(ctx)
		if err != nil {
			return organization{}, err
		}
		if len(organizations) == 0 {
			return organization{}, errorsNew("no Supabase organization was found after refresh")
		}
	}
	if len(organizations) == 1 {
		fmt.Fprintf(a.out, "Using Supabase organization: %s\n", organizationLabel(organizations[0]))
		return organizations[0], nil
	}
	fmt.Fprintln(a.out, "Choose the Supabase organization that will own this CRM:")
	for i, org := range organizations {
		fmt.Fprintf(a.out, "  %d. %s\n", i+1, organizationLabel(org))
	}
	choice, err := a.selectNumber("Organization", len(organizations), 1)
	if err != nil {
		return organization{}, err
	}
	return organizations[choice-1], nil
}

func organizationLabel(org organization) string {
	if org.Name != "" {
		return org.Name
	}
	return org.Slug
}

func (a *app) selectRegion() (regionOption, error) {
	fmt.Fprintln(a.out, "Choose the database location closest to your team:")
	for i, region := range supportedRegions {
		fmt.Fprintf(a.out, "  %d. %s\n", i+1, region.Name)
	}
	choice, err := a.selectNumber("Region", len(supportedRegions), 9)
	if err != nil {
		return regionOption{}, err
	}
	return supportedRegions[choice-1], nil
}

func (a *app) selectNumber(prompt string, maximum, fallback int) (int, error) {
	for {
		value, err := a.ask(prompt, fmt.Sprintf("%d", fallback))
		if err != nil {
			return 0, err
		}
		var choice int
		if _, err := fmt.Sscanf(value, "%d", &choice); err == nil && choice >= 1 && choice <= maximum {
			return choice, nil
		}
		fmt.Fprintf(a.out, "Enter a number from 1 to %d.\n", maximum)
	}
}

func generateDatabasePassword() (string, error) {
	const randomCharacters = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789!@#$%*-_+"
	password := []byte("Aa1!")
	randomBytes := make([]byte, 28)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("generate database password: %w", err)
	}
	for _, value := range randomBytes {
		password = append(password, randomCharacters[int(value)%len(randomCharacters)])
	}
	return string(password), nil
}

func validateDatabasePassword(password string) error {
	if len(password) < 12 {
		return errorsNew("database password is absent or too short")
	}
	if strings.ContainsAny(password, "\r\n\x00") {
		return errorsNew("database password contains invalid control characters")
	}
	return nil
}

func (a *app) createDedicatedProject(ctx context.Context, name string, org organization, region regionOption, password string) (project, error) {
	if org.reference() == "" {
		return project{}, errorsNew("Supabase organization is required")
	}
	if region.Code == "" {
		return project{}, errorsNew("Supabase region is required")
	}
	if err := validateDatabasePassword(password); err != nil {
		return project{}, err
	}
	args := []string{"projects", "create", name, "--org-id", org.reference(), "--region", region.Code, "--db-password", password, "--output", "json", "--yes"}
	fmt.Fprintln(a.out, "Creating your dedicated Supabase project. This can take a few minutes.")
	out, err := a.runSupabase(ctx, args, commandOptions{capture: true, sensitive: true, progressMessage: "Still creating the Supabase project…", secretValues: []string{password}})
	if err != nil {
		return project{}, err
	}
	if created := parseCreatedProject(out); created.reference() != "" {
		if created.Name == "" {
			created.Name = name
		}
		return created, nil
	}
	for attempt := 0; attempt < 5; attempt++ {
		projects, listErr := a.projects(ctx)
		if listErr == nil {
			sort.SliceStable(projects, func(i, j int) bool { return projects[i].CreatedAt > projects[j].CreatedAt })
			for _, candidate := range projects {
				if candidate.Name == name && candidate.reference() != "" {
					return candidate, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return project{}, ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	return project{}, fmt.Errorf("Supabase created the project but no project reference was returned or found")
}

func parseCreatedProject(output string) project {
	start := strings.IndexAny(output, "[{")
	if start < 0 {
		return project{}
	}
	var direct project
	if json.Unmarshal([]byte(output[start:]), &direct) == nil && direct.reference() != "" {
		return direct
	}
	var wrapper struct {
		Project project `json:"project"`
	}
	if json.Unmarshal([]byte(output[start:]), &wrapper) == nil {
		return wrapper.Project
	}
	return project{}
}

func (a *app) provisionNewProject(ctx context.Context, name string) error {
	org, err := a.selectOrganization(ctx)
	if err != nil {
		return err
	}
	region, err := a.selectRegion()
	if err != nil {
		return err
	}
	password, err := generateDatabasePassword()
	if err != nil {
		return err
	}
	fmt.Fprintln(a.out, "This password protects the CRM database.")
	fmt.Fprintln(a.out, "You normally will not need it to use the CRM, but save it in your password manager in case you need to administer the database later.")
	fmt.Fprintln(a.out, "It is shown once and is not written to CRM files or logs:")
	fmt.Fprintf(a.out, "\n%s\n\n", password)
	confirmed, err := a.confirm("I saved the database password", false)
	if err != nil {
		return err
	}
	if !confirmed {
		return errorsNew("save the database password before creating the project")
	}
	created, err := a.createDedicatedProject(ctx, name, org, region, password)
	if err != nil {
		return err
	}
	a.databasePassword = password
	a.project = created
	return a.saveState()
}
