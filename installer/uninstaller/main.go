package main

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	productName = "Client Interaction CRM"
	version     = "1.1.0"
	supabaseCLI = "2.117.0"
	vercelCLI   = "59.15.1"
)

type installManifest struct {
	Version               string   `json:"version"`
	InstallationDirectory string   `json:"installation_directory"`
	RuntimeDirectories    []string `json:"launcher_runtime_directories,omitempty"`
	NPMCacheDirectory     string   `json:"npm_cache_directory"`
	AppDataDirectories    []string `json:"appdata_directories"`
	ShortcutPaths         []string `json:"shortcut_paths,omitempty"`
	CRMInfoPath           string   `json:"crm_info_path"`
	PortableNodeInstalled bool     `json:"portable_node_installed_by_crm"`
	UninstallerPath       string   `json:"uninstaller_path"`
	SupabaseProjectRef    string   `json:"supabase_project_ref,omitempty"`
	VercelProject         string   `json:"vercel_project,omitempty"`
}

type choices struct {
	local, supabaseLogout, vercelLogout bool
}

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "\nUninstall could not complete safely.\nReason: %v\nPress Enter to close.", err)
		_, _ = bufio.NewReader(os.Stdin).ReadString('\n')
		os.Exit(1)
	}
}

func run(input io.Reader, output io.Writer) error {
	manifestPath, err := defaultManifestPath()
	if err != nil {
		return err
	}
	return runWithManifest(input, output, manifestPath)
}

func runWithManifest(input io.Reader, output io.Writer, manifestPath string) error {
	reader := bufio.NewReader(input)
	manifest, err := readManifest(manifestPath)
	if err != nil {
		return fmt.Errorf("read CRM ownership manifest: %w", err)
	}
	fmt.Fprintf(output, "%s Uninstaller\n\n", productName)
	fmt.Fprintln(output, "Choose cleanup options. Nothing changes before the final confirmation.")
	selection := choices{}
	if selection.local, err = confirm(reader, output, "Remove the local CRM installation, CRM-owned dependencies/runtime/cache, logs/state, CRM-INFO, shortcuts, and CRM AppData", true); err != nil {
		return err
	}
	fmt.Fprintln(output, "Signing out affects the shared CLI login and may affect other projects on this computer.")
	if selection.supabaseLogout, err = confirm(reader, output, "Sign out of Supabase CLI", false); err != nil {
		return err
	}
	if selection.vercelLogout, err = confirm(reader, output, "Sign out of Vercel CLI", false); err != nil {
		return err
	}
	if !selection.local && !selection.supabaseLogout && !selection.vercelLogout {
		fmt.Fprintln(output, "\nNo cleanup options selected. Nothing was changed.")
		return nil
	}
	if err := validateManifestOwnership(manifest, manifestPath); err != nil {
		return err
	}
	printPreview(output, manifest, selection)
	proceed, err := confirm(reader, output, "Proceed", false)
	if err != nil || !proceed {
		fmt.Fprintln(output, "Uninstall cancelled. Nothing was changed.")
		return err
	}

	if selection.supabaseLogout {
		reportLogout(output, "Supabase", runLogout(manifest, "supabase@"+supabaseCLI, "logout"))
	}
	if selection.vercelLogout {
		reportLogout(output, "Vercel", runLogout(manifest, "vercel@"+vercelCLI, "logout"))
	}
	if selection.local {
		if err := removeLocalFiles(manifest); err != nil {
			return err
		}
		fmt.Fprintln(output, "Local CRM cleanup completed.")
	}
	printRemoteReminder(output, manifest)
	if selection.local {
		return scheduleSelfRemoval(manifest.UninstallerPath)
	}
	return nil
}

func confirm(reader *bufio.Reader, output io.Writer, prompt string, defaultYes bool) (bool, error) {
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}
	fmt.Fprintf(output, "%s [%s]: ", prompt, hint)
	value, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return defaultYes, nil
	}
	return value == "y" || value == "yes", nil
}

func defaultManifestPath() (string, error) {
	dir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ClientInteractionCRM", "install-manifest.json"), nil
}

func readManifest(path string) (installManifest, error) {
	var manifest installManifest
	data, err := os.ReadFile(path)
	if err != nil {
		return manifest, err
	}
	if err := json.Unmarshal(data, &manifest); err != nil {
		return manifest, err
	}
	if manifest.Version != version {
		return manifest, fmt.Errorf("manifest version %q does not match %s", manifest.Version, version)
	}
	return manifest, nil
}

func printPreview(output io.Writer, manifest installManifest, selected choices) {
	fmt.Fprintln(output, "\nThe following items will be removed:")
	if selected.local {
		fmt.Fprintln(output, manifest.InstallationDirectory)
		for _, path := range manifest.RuntimeDirectories {
			fmt.Fprintln(output, path)
		}
		fmt.Fprintln(output, manifest.CRMInfoPath)
		fmt.Fprintln(output, manifest.NPMCacheDirectory)
		for _, path := range manifest.AppDataDirectories {
			fmt.Fprintln(output, path)
		}
		for _, path := range manifest.ShortcutPaths {
			fmt.Fprintln(output, path)
		}
		fmt.Fprintln(output, manifest.UninstallerPath)
	}
	fmt.Fprintf(output, "\nSupabase login: %s\nVercel login: %s\n", keepOrRemove(selected.supabaseLogout), keepOrRemove(selected.vercelLogout))
	fmt.Fprintln(output, "Online Supabase/Vercel projects will NOT be deleted.")
}

func keepOrRemove(remove bool) string {
	if remove {
		return "sign out"
	}
	return "keep"
}

func validateManifestOwnership(manifest installManifest, manifestPath string) error {
	root, err := filepath.Abs(manifest.InstallationDirectory)
	if err != nil || root != filepath.Clean(manifest.InstallationDirectory) {
		return errors.New("manifest installation path is malformed")
	}
	if err := validateInstallRoot(root); err != nil {
		return err
	}
	_, local, err := crmDataDirs()
	if err != nil {
		return err
	}
	if !samePath(manifestPath, filepath.Join(local, "install-manifest.json")) || !samePath(manifest.NPMCacheDirectory, filepath.Join(local, "npm-cache")) || !samePath(manifest.UninstallerPath, filepath.Join(local, "Uninstall Client Interaction CRM.exe")) {
		return errors.New("manifest contains a non-CRM cache, metadata, or uninstaller path")
	}
	roaming, expectedLocal, _ := crmDataDirs()
	for _, path := range manifest.AppDataDirectories {
		if !samePath(path, roaming) && !samePath(path, expectedLocal) {
			return fmt.Errorf("manifest contains a non-CRM AppData path: %s", path)
		}
	}
	for _, path := range manifest.RuntimeDirectories {
		if !samePath(path, filepath.Join(root, ".crm-runtime", "node")) {
			return fmt.Errorf("manifest contains a non-CRM runtime path: %s", path)
		}
	}
	if !samePath(manifest.CRMInfoPath, filepath.Join(root, "CRM-INFO.txt")) {
		return errors.New("manifest contains a non-CRM information path")
	}
	home, _ := os.UserHomeDir()
	for _, path := range manifest.ShortcutPaths {
		if !samePath(path, filepath.Join(home, "Desktop", "Client Interaction CRM.url")) {
			return fmt.Errorf("manifest contains a non-CRM shortcut path: %s", path)
		}
		if exists(path) {
			info, statErr := os.Lstat(path)
			if statErr != nil || !info.Mode().IsRegular() {
				return fmt.Errorf("refusing unsafe shortcut path: %s", path)
			}
		}
	}
	for _, path := range []string{roaming, expectedLocal} {
		if exists(path) {
			if err := rejectReparseTree(path); err != nil {
				return err
			}
		}
	}
	return rejectReparseTree(root)
}

func validateInstallRoot(root string) error {
	volume := filepath.VolumeName(root)
	if !filepath.IsAbs(root) || samePath(root, string(os.PathSeparator)) || (volume != "" && samePath(root, volume+string(os.PathSeparator))) {
		return errors.New("refusing to remove a drive or filesystem root")
	}
	home, _ := os.UserHomeDir()
	protected := []string{home, filepath.Join(home, "Desktop"), filepath.Join(home, "Documents")}
	for _, path := range protected {
		if path != "" && samePath(root, path) {
			return fmt.Errorf("refusing to remove protected user directory: %s", root)
		}
	}
	if exists(filepath.Join(root, ".git")) || exists(filepath.Join(root, "app", ".git")) {
		return errors.New("refusing to remove a repository/development checkout")
	}
	marker, err := os.ReadFile(filepath.Join(root, "app", ".crm-version"))
	if err != nil || strings.TrimSpace(string(marker)) != version {
		return errors.New("installation marker is missing or does not match")
	}
	var identity struct {
		Name string `json:"name"`
	}
	data, err := os.ReadFile(filepath.Join(root, "app", "package.json"))
	if err != nil || json.Unmarshal(data, &identity) != nil || identity.Name != "client-interaction-crm" {
		return errors.New("application package identity is missing or does not match")
	}
	if !exists(filepath.Join(root, ".crm-state.json")) {
		return errors.New("CRM state marker is missing")
	}
	return nil
}

func crmDataDirs() (string, string, error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", "", err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", "", err
	}
	return filepath.Join(config, "ClientInteractionCRM"), filepath.Join(cache, "ClientInteractionCRM"), nil
}

func removeLocalFiles(manifest installManifest) error {
	for _, shortcut := range manifest.ShortcutPaths {
		if err := os.Remove(shortcut); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	if err := os.RemoveAll(manifest.InstallationDirectory); err != nil {
		return err
	}
	roaming, local, _ := crmDataDirs()
	if err := os.RemoveAll(roaming); err != nil {
		return err
	}
	// The running executable and manifest are below Local AppData. Windows
	// removes that final CRM-owned directory through the deferred helper.
	if runtime.GOOS != "windows" {
		return os.RemoveAll(local)
	}
	return nil
}

func runLogout(manifest installManifest, pkg, command string) error {
	node, npxJS := findNodeAndNPX(manifest)
	if node == "" || npxJS == "" {
		return errors.New("Node/npm runtime is unavailable; login was kept")
	}
	cmd := exec.Command(node, npxJS, "--yes", pkg, command)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "npm_config_cache="+manifest.NPMCacheDirectory, "NO_UPDATE_NOTIFIER=1")
	return cmd.Run()
}

func findNodeAndNPX(manifest installManifest) (string, string) {
	candidates := []string{}
	if manifest.PortableNodeInstalled {
		candidates = append(candidates, filepath.Join(manifest.InstallationDirectory, ".crm-runtime", "node"))
	}
	if node, err := exec.LookPath("node"); err == nil {
		candidates = append(candidates, filepath.Dir(node))
	}
	for _, bin := range candidates {
		node := filepath.Join(bin, "node")
		if runtime.GOOS == "windows" {
			node += ".exe"
		}
		npx := filepath.Join(bin, "node_modules", "npm", "bin", "npx-cli.js")
		if exists(node) && exists(npx) {
			return node, npx
		}
	}
	return "", ""
}

func reportLogout(output io.Writer, name string, err error) {
	if err != nil {
		fmt.Fprintf(output, "%s CLI sign-out: failed (%v). Local cleanup may continue safely.\n", name, err)
	} else {
		fmt.Fprintf(output, "%s CLI sign-out: completed.\n", name)
	}
}

func printRemoteReminder(output io.Writer, manifest installManifest) {
	fmt.Fprintln(output, "\nYour online CRM infrastructure still exists.")
	if manifest.SupabaseProjectRef != "" {
		fmt.Fprintf(output, "Supabase: https://supabase.com/dashboard/project/%s\n", manifest.SupabaseProjectRef)
	}
	if manifest.VercelProject != "" {
		fmt.Fprintf(output, "Vercel project %s: https://vercel.com/dashboard\n", manifest.VercelProject)
	}
	fmt.Fprintln(output, "Delete these manually only if you also want to permanently delete the CRM data/service.")
}

func samePath(a, b string) bool { return strings.EqualFold(filepath.Clean(a), filepath.Clean(b)) }
func exists(path string) bool   { _, err := os.Lstat(path); return err == nil }
