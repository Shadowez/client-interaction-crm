//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

func rejectReparseTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return rejectPathReparse(path)
	})
}

func hasReparsePoint(path string) (bool, error) {
	pointer, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false, err
	}
	attributes, err := syscall.GetFileAttributes(pointer)
	if err != nil {
		return false, err
	}
	return attributes&syscall.FILE_ATTRIBUTE_REPARSE_POINT != 0, nil
}

func rejectPathReparse(path string) error {
	reparse, err := hasReparsePoint(path)
	if err != nil {
		return err
	}
	if reparse {
		return fmt.Errorf("refusing to recursively remove reparse point: %s", path)
	}
	return nil
}
