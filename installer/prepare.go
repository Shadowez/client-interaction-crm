package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

func (a *app) prepare(ctx context.Context) error {
	if err := a.preparePayload(ctx); err != nil {
		return err
	}
	if err := a.prepareNode(ctx); err != nil {
		return err
	}
	_, err := a.command(ctx, a.npm, []string{"ci", "--no-audit", "--no-fund"}, commandOptions{})
	return err
}

func (a *app) preparePayload(ctx context.Context) error {
	packageJSON := filepath.Join(a.appDir, "package.json")
	marker := filepath.Join(a.appDir, ".crm-version")
	if installedVersion, err := os.ReadFile(marker); err == nil && strings.TrimSpace(string(installedVersion)) == version {
		fmt.Fprintln(a.out, "Existing application files found; safely resuming setup.")
		return nil
	}
	if _, err := os.Stat(packageJSON); err == nil {
		return errorsNew("app directory belongs to an incomplete or different CRM release; choose a new installation directory")
	}
	if entries, err := os.ReadDir(a.appDir); err == nil && len(entries) > 0 {
		return errorsNew("app directory is not empty; choose a new installation directory")
	}
	temporary, err := os.MkdirTemp(a.root, ".payload-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	asset := fmt.Sprintf("client-interaction-crm-app-v%s.zip", version)
	archivePath := filepath.Join(temporary, asset)
	if err := a.acquirePayload(ctx, asset, archivePath); err != nil {
		return err
	}
	staging := filepath.Join(temporary, "extract")
	if err := extractZip(archivePath, staging); err != nil {
		return err
	}
	stagedApp := filepath.Join(staging, "app")
	if _, err := os.Stat(filepath.Join(stagedApp, "package.json")); err != nil {
		return fmt.Errorf("release payload has no app/package.json")
	}
	if err := os.RemoveAll(a.appDir); err != nil {
		return err
	}
	if err := os.Rename(stagedApp, a.appDir); err != nil {
		return err
	}
	return os.WriteFile(marker, []byte(version+"\n"), 0o644)
}

func releasePayloadURLs(releaseVersion, asset string) (string, string) {
	base := fmt.Sprintf("https://github.com/Shadowez/client-interaction-crm/releases/download/v%s/", releaseVersion)
	return base + asset, base + asset + ".sha256"
}

func localPayloadOverride(getenv func(string) string) (string, bool) {
	path := strings.TrimSpace(getenv("CRM_PAYLOAD_FILE"))
	return path, path != ""
}

func (a *app) acquirePayload(ctx context.Context, asset, destination string) error {
	if localPath, enabled := localPayloadOverride(os.Getenv); enabled {
		fmt.Fprintln(a.out, "Maintainer testing: using the explicit local payload override.")
		log.Print("maintainer local payload override enabled")
		if !filepath.IsAbs(localPath) {
			return errorsNew("CRM_PAYLOAD_FILE must be an absolute local file path")
		}
		info, err := os.Stat(localPath)
		if err != nil {
			return fmt.Errorf("open CRM_PAYLOAD_FILE: %w", err)
		}
		if !info.Mode().IsRegular() {
			return errorsNew("CRM_PAYLOAD_FILE must identify a regular file")
		}
		if err := copyLocalFile(localPath, destination); err != nil {
			return fmt.Errorf("copy local CRM payload: %w", err)
		}
		expected := strings.TrimSpace(os.Getenv("CRM_PAYLOAD_SHA256"))
		if expected == "" {
			checksumPath := localPath + ".sha256"
			data, err := os.ReadFile(checksumPath)
			if err != nil {
				return fmt.Errorf("read local payload checksum %s: %w", checksumPath, err)
			}
			expected, err = expectedChecksum(string(data), filepath.Base(localPath))
			if err != nil {
				return err
			}
		} else if !validSHA256(expected) {
			return errorsNew("CRM_PAYLOAD_SHA256 must be exactly 64 hexadecimal characters")
		}
		if err := verifyChecksum(destination, expected); err != nil {
			return fmt.Errorf("verify local CRM payload: %w", err)
		}
		return nil
	}

	payloadURL, checksumURL := releasePayloadURLs(version, asset)
	checksumPath := destination + ".sha256"
	if err := download(ctx, payloadURL, destination); err != nil {
		return fmt.Errorf("download versioned CRM payload: %w", err)
	}
	if err := download(ctx, checksumURL, checksumPath); err != nil {
		return fmt.Errorf("download CRM checksum: %w", err)
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}
	expected, err := expectedChecksum(string(data), asset)
	if err != nil {
		return err
	}
	return verifyChecksum(destination, expected)
}

func (a *app) prepareNode(ctx context.Context) error {
	if path, err := exec.LookPath("node"); err == nil {
		output, runErr := exec.CommandContext(ctx, path, "--version").Output()
		bin := filepath.Dir(path)
		if runErr == nil && compatibleNode(string(output)) && commandExists(filepath.Join(bin, npmName())) && commandExists(filepath.Join(bin, npxName())) {
			a.setNodePaths(bin)
			return nil
		}
	}
	nodeHome := filepath.Join(a.runtime, "node")
	if existing := localNodeBinary(nodeHome); existing != "" {
		a.setNodePaths(filepath.Dir(existing))
		return nil
	}
	if runtime.GOARCH != "amd64" || (runtime.GOOS != "windows" && runtime.GOOS != "linux") {
		return fmt.Errorf("portable Node is supported on Windows x64 and Linux x64, got %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	if err := os.MkdirAll(a.runtime, 0o755); err != nil {
		return err
	}
	temporary, err := os.MkdirTemp(a.runtime, ".node-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	platform, extension := "linux-x64", "tar.gz"
	if runtime.GOOS == "windows" {
		platform, extension = "win-x64", "zip"
	}
	filename := fmt.Sprintf("node-v%s-%s.%s", nodeVersion, platform, extension)
	base := fmt.Sprintf("https://nodejs.org/download/release/v%s/", nodeVersion)
	archivePath := filepath.Join(temporary, filename)
	checksumPath := filepath.Join(temporary, "SHASUMS256.txt")
	if err := download(ctx, base+"SHASUMS256.txt", checksumPath); err != nil {
		return err
	}
	if err := download(ctx, base+filename, archivePath); err != nil {
		return err
	}
	data, err := os.ReadFile(checksumPath)
	if err != nil {
		return err
	}
	expected, err := expectedChecksum(string(data), filename)
	if err != nil {
		return err
	}
	if err := verifyChecksum(archivePath, expected); err != nil {
		return err
	}
	staging := filepath.Join(temporary, "extract")
	if extension == "zip" {
		err = extractZip(archivePath, staging)
	} else {
		err = extractTarGz(archivePath, staging)
	}
	if err != nil {
		return err
	}
	extracted := filepath.Join(staging, strings.TrimSuffix(filename, "."+extension))
	if err := os.Rename(extracted, nodeHome); err != nil {
		if removeErr := os.RemoveAll(nodeHome); removeErr != nil {
			return removeErr
		}
		if renameErr := os.Rename(extracted, nodeHome); renameErr != nil {
			return renameErr
		}
	} else {
		// Renamed successfully on the first attempt.
	}
	a.setNodePaths(nodeBinaryDir(nodeHome))
	return nil
}

func compatibleNode(value string) bool {
	value = strings.TrimSpace(strings.TrimPrefix(value, "v"))
	majorText, _, _ := strings.Cut(value, ".")
	major, err := strconv.Atoi(majorText)
	return err == nil && major >= 20 && major <= 24
}

func nodeBinaryDir(home string) string {
	if runtime.GOOS == "windows" {
		return home
	}
	return filepath.Join(home, "bin")
}

func localNodeBinary(home string) string {
	name := "node"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(nodeBinaryDir(home), name)
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func (a *app) setNodePaths(bin string) {
	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".cmd"
	}
	a.node = filepath.Join(bin, "node"+map[bool]string{true: ".exe", false: ""}[runtime.GOOS == "windows"])
	a.npm = filepath.Join(bin, "npm"+ext)
	a.npx = filepath.Join(bin, "npx"+ext)
}

func npmName() string {
	if runtime.GOOS == "windows" {
		return "npm.cmd"
	}
	return "npm"
}
func npxName() string {
	if runtime.GOOS == "windows" {
		return "npx.cmd"
	}
	return "npx"
}
func commandExists(path string) bool { _, err := os.Stat(path); return err == nil }

func errorsNew(message string) error { return fmt.Errorf("%s", message) }
