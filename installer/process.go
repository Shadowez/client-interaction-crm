package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

type commandOptions struct {
	interactive     bool
	capture         bool
	sensitive       bool
	env             []string
	stdin           string
	progressMessage string
	progressAfter   time.Duration
	progressEvery   time.Duration
	secretValues    []string
}

func (a *app) command(ctx context.Context, executable string, args []string, options commandOptions) (string, error) {
	// Node's Windows npm/npx launchers are batch files. Execute their JavaScript
	// entry points with node.exe so paths and user input never cross cmd.exe.
	actualExecutable, actualArgs := executableArgs(runtime.GOOS, executable, args)
	cmd := exec.CommandContext(ctx, actualExecutable, actualArgs...)
	cmd.Dir = a.appDir
	cmd.Env = append(os.Environ(), a.commandEnvironment(executable, options.env)...)
	var captured bytes.Buffer
	var diagnostic bytes.Buffer
	if options.capture {
		cmd.Stdout = &captured
	} else {
		cmd.Stdout = &diagnostic
	}
	cmd.Stderr = &diagnostic
	if options.interactive {
		cmd.Stdin = os.Stdin
		// Authentication links and device codes must be visible live, but are
		// deliberately never copied to the persistent troubleshooting log.
		cmd.Stdout = a.out
		cmd.Stderr = a.out
	}
	if options.stdin != "" {
		cmd.Stdin = strings.NewReader(options.stdin)
	}
	log.Printf("run executable=%q args=%q", actualExecutable, redactArgs(actualArgs))
	var err error
	if options.progressMessage == "" || options.interactive {
		err = cmd.Run()
	} else {
		err = a.runWithProgress(ctx, cmd, options)
	}
	if !options.interactive && diagnostic.Len() > 0 {
		sanitized := redactText(diagnostic.String(), options.secretValues)
		_, _ = io.WriteString(a.logFile, sanitized)
		if a.verbose {
			_, _ = io.WriteString(a.out, sanitized)
		}
	}
	if options.capture && !options.sensitive && captured.Len() > 0 {
		_, _ = io.WriteString(a.logFile, redactText(captured.String(), options.secretValues))
	}
	if err != nil {
		return captured.String(), friendlyCommandError(executable, args, diagnostic.String()+"\n"+captured.String(), err)
	}
	return strings.TrimSpace(captured.String()), nil
}

func (a *app) commandEnvironment(executable string, existing []string) []string {
	env := append([]string(nil), existing...)
	if a.npmCache != "" && isNPMExecutable(executable) {
		env = append(env, "npm_config_cache="+a.npmCache)
	}
	return env
}

func isNPMExecutable(executable string) bool {
	name := strings.ToLower(strings.TrimSuffix(filepath.Base(executable), filepath.Ext(executable)))
	return name == "npm" || name == "npx"
}

func redactText(value string, secrets []string) string {
	for _, secret := range secrets {
		if secret != "" {
			value = strings.ReplaceAll(value, secret, "<redacted>")
		}
	}
	return redactAuthenticationMaterial(value)
}

var sensitiveLogPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(https?://[^\s]*(?:session_id|token_name|public_key|code|token|secret)=[^\s&]+[^\s]*)`),
	regexp.MustCompile(`(?i)((?:session_id|token_name|public_key|verification[_ -]?code|device[_ -]?code|access[_ -]?token|refresh[_ -]?token|management[_ -]?api[_ -]?token|oidc[_ -]?token|service[_ -]?role[_ -]?key|client[_ -]?secret)\s*[:=]\s*)[^\s,;]+`),
	regexp.MustCompile(`\b(?:sb_secret_|sb_service_role_|eyJ)[A-Za-z0-9._~-]{12,}\b`),
}

func redactAuthenticationMaterial(value string) string {
	for index, pattern := range sensitiveLogPatterns {
		if index == 0 {
			value = pattern.ReplaceAllString(value, "<redacted-auth-url>")
		} else if index == 1 {
			value = pattern.ReplaceAllString(value, "$1<redacted>")
		} else {
			value = pattern.ReplaceAllString(value, "<redacted>")
		}
	}
	return value
}

func executableArgs(goos, executable string, args []string) (string, []string) {
	if goos != "windows" || !strings.EqualFold(filepath.Ext(executable), ".cmd") {
		return executable, append([]string(nil), args...)
	}
	separator := strings.LastIndexAny(executable, `/\`)
	base := executable[separator+1:]
	extension := filepath.Ext(base)
	name := strings.ToLower(strings.TrimSuffix(base, extension))
	if name != "npm" && name != "npx" {
		return executable, append([]string(nil), args...)
	}
	bin := "."
	if separator >= 0 {
		bin = executable[:separator]
	}
	join := func(parts ...string) string { return strings.Join(parts, `\`) }
	node := join(bin, "node.exe")
	entrypoint := join(bin, "node_modules", "npm", "bin", name+"-cli.js")
	return node, append([]string{entrypoint}, args...)
}

func (a *app) runWithProgress(ctx context.Context, cmd *exec.Cmd, options commandOptions) error {
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	after := options.progressAfter
	if after <= 0 {
		after = 45 * time.Second
	}
	every := options.progressEvery
	if every <= 0 {
		every = 45 * time.Second
	}
	timer := time.NewTimer(after)
	defer timer.Stop()
	for {
		select {
		case err := <-done:
			return err
		case <-ctx.Done():
			return <-done
		case <-timer.C:
			fmt.Fprintln(a.out, options.progressMessage)
			timer.Reset(every)
		}
	}
}

func friendlyCommandError(executable string, args []string, details string, err error) error {
	lower := strings.ToLower(details)
	isSupabase := argsContain(args, "supabase@")
	isVercel := argsContain(args, "vercel@")
	if isSupabase && argsContain(args, "projects") && argsContain(args, "create") &&
		((strings.Contains(lower, "maximum") && strings.Contains(lower, "active free project")) || strings.Contains(lower, "project limit")) {
		return errorsNew("Supabase cannot create another project because your account has reached its active-project limit.\n\nDelete or pause an unused Supabase project, or upgrade your Supabase plan, then run this installer again.\n\nYour CRM setup files are safe. You can safely run the installer again using the same installation folder")
	}
	if strings.Contains(lower, "eai_again") || strings.Contains(lower, "getaddrinfo") || strings.Contains(lower, "network is unreachable") || strings.Contains(lower, "connection timed out") {
		return errorsNew("A network connection could not be completed. Check your internet connection, then safely run the installer again using the same installation folder")
	}
	if isSupabase && argsContain(args, "login") {
		return errorsNew("Supabase sign-in was cancelled or did not complete. You can safely run the installer again using the same installation folder")
	}
	if isVercel && argsContain(args, "login") {
		return errorsNew("Vercel sign-in was cancelled or did not complete. You can safely run the installer again using the same installation folder")
	}
	if isVercel && argsContain(args, "deploy") {
		return errorsNew("Vercel could not deploy the web application. See the troubleshooting log for details, then safely run the installer again using the same installation folder")
	}
	if isVercel && argsContain(args, "inspect") {
		return errorsNew("Vercel could not confirm that the production deployment is ready. See the troubleshooting log, then safely run the installer again using the same installation folder")
	}
	return fmt.Errorf("command failed (%s): %w", filepathBase(executable), err)
}

func argsContain(args []string, fragment string) bool {
	for _, arg := range args {
		if strings.Contains(strings.ToLower(arg), strings.ToLower(fragment)) {
			return true
		}
	}
	return false
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
		result[i] = redactAuthenticationMaterial(result[i])
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
