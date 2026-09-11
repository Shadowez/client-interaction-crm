//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func rejectReparseTree(root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to recursively remove symbolic link: %s", path)
		}
		return nil
	})
}
