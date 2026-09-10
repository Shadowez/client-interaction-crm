package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func safeTarget(base, name string) (string, error) {
	target := filepath.Join(base, filepath.FromSlash(name))
	rel, err := filepath.Rel(base, target)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return "", fmt.Errorf("unsafe archive path %q", name)
	}
	return target, nil
}

func extractZip(source, destination string) error {
	r, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target, err := safeTarget(destination, f.Name)
		if err != nil {
			return err
		}
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		in, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, f.Mode().Perm())
		if err != nil {
			in.Close()
			return err
		}
		_, copyErr := io.Copy(out, in)
		closeErr := out.Close()
		in.Close()
		if copyErr != nil {
			return copyErr
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}

func extractTarGz(source, destination string) error {
	f, err := os.Open(source)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target, err := safeTarget(destination, h.Name)
		if err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(h.Mode).Perm())
			if err != nil {
				return err
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
}
