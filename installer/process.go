package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

type commandOptions struct {
	interactive bool
	capture     bool
	sensitive   bool
	env         []string
	stdin       string
}

func (a *app) command(ctx context.Context, executable string, args []string, options commandOptions) (string, error) {
	// Arguments are always passed directly; user input is never evaluated by a shell.
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir = a.appDir
	cmd.Env = append(os.Environ(), options.env...)
	var captured bytes.Buffer
	logWriter := io.Writer(a.logFile)
	if a.verbose {
		logWriter = io.MultiWriter(a.logFile, a.out)
	}
	if options.capture {
		cmd.Stdout = &captured
		if !options.sensitive {
			cmd.Stdout = io.MultiWriter(&captured, a.logFile)
		}
	} else {
		cmd.Stdout = logWriter
	}
	cmd.Stderr = logWriter
	if options.interactive {
		cmd.Stdin = os.Stdin
		if !a.verbose {
			cmd.Stdout = io.MultiWriter(a.out, a.logFile)
			cmd.Stderr = io.MultiWriter(a.out, a.logFile)
		}
	}
	if options.stdin != "" {
		cmd.Stdin = strings.NewReader(options.stdin)
	}
	log.Printf("run executable=%q args=%q", executable, redactArgs(args))
	if err := cmd.Run(); err != nil {
		return captured.String(), fmt.Errorf("command failed (%s): %w", filepathBase(executable), err)
	}
	return strings.TrimSpace(captured.String()), nil
}

func redactArgs(args []string) []string {
	result := append([]string(nil), args...)
	redactNext := false
	for i, value := range result {
		if redactNext {
			result[i] = "<redacted>"
			redactNext = false
			continue
		}
		upper := strings.ToUpper(value)
		if strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "TOKEN") || strings.Contains(upper, "KEY=") {
			if before, _, ok := strings.Cut(value, "="); ok {
				result[i] = before + "=<redacted>"
			} else {
				redactNext = true
			}
		}
	}
	return result
}

func filepathBase(path string) string {
	parts := strings.FieldsFunc(path, func(r rune) bool { return r == '/' || r == '\\' })
	if len(parts) == 0 {
		return path
	}
	return parts[len(parts)-1]
}

func openBrowser(ctx context.Context, url string) error {
	var command string
	var args []string
	switch runtime.GOOS {
	case "windows":
		command, args = "rundll32", []string{"url.dll,FileProtocolHandler", url}
	case "darwin":
		command, args = "open", []string{url}
	default:
		command, args = "xdg-open", []string{url}
	}
	return exec.CommandContext(ctx, command, args...).Start()
}
