package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const uninstallerAsset = "Uninstall Client Interaction CRM.exe"
const uninstallerReleaseAsset = "Uninstall.Client.Interaction.CRM.exe"

type installManifest struct {
	Version               string   `json:"version"`
	InstallationDirectory string   `json:"installation_directory"`
	RuntimeDirectories    []string `json:"launcher_runtime_directories,omitempty"`
	NPMCacheDirectory     string   `json:"npm_cache_directory"`
	AppDataDirectories    []string `json:"appdata_directories"`
	ShortcutPaths         []string `json:"shortcut_paths,omitempty"`
	CRMInfoPath           string   `json:"crm_info_path"`
	PortableNodeInstalled bool     `json:"portable_node_installed_by_crm"`
	UninstallerPath       string   `json:"uninstaller_path,omitempty"`
	SupabaseProjectRef    string   `json:"supabase_project_ref,omitempty"`
	VercelProject         string   `json:"vercel_project,omitempty"`
}

func crmDataDirs() (roaming, local string, err error) {
	config, err := os.UserConfigDir()
	if err != nil {
		return "", "", fmt.Errorf("find per-user config directory: %w", err)
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return "", "", fmt.Errorf("find per-user cache directory: %w", err)
	}
	return filepath.Join(config, "ClientInteractionCRM"), filepath.Join(cache, "ClientInteractionCRM"), nil
}

func crmNPMCacheDir() (string, error) {
	_, local, err := crmDataDirs()
	if err != nil {
		return "", err
	}
	return filepath.Join(local, "npm-cache"), nil
}

func installManifestPath() (string, error) {
	_, local, err := crmDataDirs()
	if err != nil {
		return "", err
	}
	return filepath.Join(local, "install-manifest.json"), nil
}

func (a *app) currentManifest() (installManifest, error) {
	roaming, local, err := crmDataDirs()
	if err != nil {
		return installManifest{}, err
	}
	runtimes := []string{}
	if a.portableNodeOwned {
		runtimes = append(runtimes, filepath.Join(a.runtime, "node"))
	}
	manifest := installManifest{
		Version: version, InstallationDirectory: a.root, RuntimeDirectories: runtimes,
		NPMCacheDirectory: a.npmCache, AppDataDirectories: []string{roaming, local},
		CRMInfoPath: filepath.Join(a.root, "CRM-INFO.txt"), PortableNodeInstalled: a.portableNodeOwned,
		SupabaseProjectRef: a.project.reference(),
		VercelProject:      vercelProjectName(displayCompany(a.company)),
	}
	if runtime.GOOS == "windows" {
		manifest.UninstallerPath = filepath.Join(local, uninstallerAsset)
		if path, pathErr := installManifestPath(); pathErr == nil {
			if data, readErr := os.ReadFile(path); readErr == nil {
				var previous installManifest
				if json.Unmarshal(data, &previous) == nil && filepath.Clean(previous.InstallationDirectory) == filepath.Clean(a.root) {
					manifest.ShortcutPaths = append(manifest.ShortcutPaths, previous.ShortcutPaths...)
				}
			}
		}
	}
	return manifest, nil
}

func (a *app) installLifecycleFiles(ctx context.Context) error {
	if err := os.MkdirAll(a.npmCache, 0o700); err != nil {
		return fmt.Errorf("create CRM-owned npm cache: %w", err)
	}
	manifest, err := a.currentManifest()
	if err != nil {
		return err
	}
	if runtime.GOOS == "windows" {
		if err := a.acquireUninstaller(ctx, manifest.UninstallerPath); err != nil {
			return err
		}
	}
	return writeInstallManifest(manifest)
}

func (a *app) acquireUninstaller(ctx context.Context, destination string) error {
	if local, enabled := localUninstallerOverride(os.Getenv); enabled {
		if !filepath.IsAbs(local) {
			return errorsNew("CRM_UNINSTALLER_FILE must be an absolute local file path")
		}
		info, err := os.Stat(local)
		if err != nil {
			return fmt.Errorf("open CRM_UNINSTALLER_FILE: %w", err)
		}
		if !info.Mode().IsRegular() {
			return errorsNew("CRM_UNINSTALLER_FILE must identify a regular file")
		}
		expected := strings.TrimSpace(os.Getenv("CRM_UNINSTALLER_SHA256"))
		if expected == "" {
			checksumPath := local + ".sha256"
			checksumData, readErr := os.ReadFile(checksumPath)
			if readErr != nil {
				return fmt.Errorf("read local CRM uninstaller checksum %s: %w", checksumPath, readErr)
			}
			expected, err = expectedChecksum(string(checksumData), filepath.Base(local))
			if err != nil {
				return err
			}
		} else if !validSHA256(expected) {
			return errorsNew("CRM_UNINSTALLER_SHA256 must be exactly 64 hexadecimal characters")
		}
		if err := verifyChecksum(local, expected); err != nil {
			return fmt.Errorf("local CRM uninstaller checksum verification failed: %w", err)
		}
		if err := copyLocalFile(local, destination); err != nil {
			return fmt.Errorf("copy local CRM uninstaller: %w", err)
		}
		log.Print("maintainer local uninstaller override enabled")
		return nil
	}
	temporary, err := os.MkdirTemp("", "crm-uninstaller-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	file := filepath.Join(temporary, uninstallerAsset)
	checksum := file + ".sha256"
	assetURL, checksumURL := releaseUninstallerURLs(version)
	if err := download(ctx, assetURL, file); err != nil {
		return fmt.Errorf("download CRM uninstaller: %w", err)
	}
	if err := download(ctx, checksumURL, checksum); err != nil {
		return fmt.Errorf("download CRM uninstaller checksum: %w", err)
	}
	data, err := os.ReadFile(checksum)
	if err != nil {
		return err
	}
	expected, err := expectedChecksum(string(data), uninstallerAsset)
	if err != nil {
		return err
	}
	if err := verifyChecksum(file, expected); err != nil {
		return fmt.Errorf("CRM uninstaller checksum verification failed: %w", err)
	}
	return copyLocalFile(file, destination)
}

func localUninstallerOverride(getenv func(string) string) (string, bool) {
	path := strings.TrimSpace(getenv("CRM_UNINSTALLER_FILE"))
	return path, path != ""
}

func releaseUninstallerURLs(releaseVersion string) (string, string) {
	base := fmt.Sprintf("https://github.com/Shadowez/client-interaction-crm/releases/download/v%s/", releaseVersion)
	return base + uninstallerReleaseAsset, base + uninstallerReleaseAsset + ".sha256"
}

func writeInstallManifest(manifest installManifest) error {
	path, err := installManifestPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}

func (a *app) finalizeLifecycleFiles() error {
	manifest, err := a.currentManifest()
	if err != nil {
		return err
	}
	manifest.VercelProject = vercelProjectName(displayCompany(a.company))
	info := fmt.Sprintf("Client Interaction CRM %s\n\nInstallation: %s\nCRM: %s\nSupabase dashboard: https://supabase.com/dashboard/project/%s\nVercel dashboard: https://vercel.com/dashboard/%s\n\nOnline projects are not removed by the local uninstaller.\n", version, a.root, a.finalURL, a.project.reference(), manifest.VercelProject)
	if err := os.WriteFile(manifest.CRMInfoPath, []byte(info), 0o600); err != nil {
		return err
	}
	if runtime.GOOS == "windows" && a.finalURL != "" {
		if home, homeErr := os.UserHomeDir(); homeErr == nil {
			desktop := filepath.Join(home, "Desktop")
			if info, statErr := os.Stat(desktop); statErr == nil && info.IsDir() {
				shortcut := filepath.Join(desktop, "Client Interaction CRM.url")
				content := fmt.Sprintf("[InternetShortcut]\r\nURL=%s\r\n", a.finalURL)
				if writeErr := os.WriteFile(shortcut, []byte(content), 0o600); writeErr == nil {
					manifest.ShortcutPaths = []string{shortcut}
				}
			}
		}
	}
	return writeInstallManifest(manifest)
}
