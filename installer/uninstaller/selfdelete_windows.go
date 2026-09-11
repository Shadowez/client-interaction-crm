//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func scheduleSelfRemoval(uninstallerPath string) error {
	directory := filepath.Dir(uninstallerPath)
	_, local, err := crmDataDirs()
	if err != nil {
		return err
	}
	if !samePath(directory, local) || strings.ContainsAny(directory, `"%&|<>^!`) {
		return fmt.Errorf("refusing unsafe self-removal path: %s", directory)
	}
	helper := filepath.Join(os.TempDir(), fmt.Sprintf("client-interaction-crm-uninstall-%d.cmd", os.Getpid()))
	content := "@echo off\r\n" +
		"ping 127.0.0.1 -n 3 >nul\r\n" +
		"rmdir /s /q \"" + directory + "\"\r\n" +
		"del /f /q \"%~f0\"\r\n"
	if err := os.WriteFile(helper, []byte(content), 0o600); err != nil {
		return err
	}
	cmd := exec.Command("cmd.exe", "/d", "/c", helper)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x00000008}
	return cmd.Start()
}
