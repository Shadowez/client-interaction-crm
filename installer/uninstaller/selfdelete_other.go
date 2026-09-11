//go:build !windows

package main

import "os"

func scheduleSelfRemoval(uninstallerPath string) error {
	return os.RemoveAll(uninstallerPath)
}
