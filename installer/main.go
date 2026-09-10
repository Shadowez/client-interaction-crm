package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	defaultVersion = "1.1.0"
	nodeVersion    = "22.23.2"
	supabaseCLI    = "2.117.0"
	vercelCLI      = "59.15.1"
)

var version = defaultVersion

type app struct {
	in       *bufio.Reader
	out      io.Writer
	verbose  bool
	logFile  *os.File
	root     string
	appDir   string
	runtime  string
	node     string
	npm      string
	npx      string
	company  string
	project  project
	finalURL string
}

func main() {
	verbose := flag.Bool("verbose", false, "show detailed command output")
	showVersion := flag.Bool("version", false, "print launcher version")
	flag.Parse()
	if *showVersion {
		fmt.Printf("Client Interaction CRM Setup %s\n", version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	a := &app{in: bufio.NewReader(os.Stdin), out: os.Stdout, verbose: *verbose}
	if err := a.run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\nSetup stopped: %v\n", err)
		if a.logFile != nil {
			fmt.Fprintf(os.Stderr, "Troubleshooting log: %s\n", a.logFile.Name())
		}
		os.Exit(1)
	}
}

func (a *app) run(ctx context.Context) error {
	fmt.Fprintln(a.out, "Client Interaction CRM Setup")
	fmt.Fprintln(a.out, "This launcher installs local setup files and deploys a private CRM for one trusted team.")

	defaultRoot, err := defaultInstallDir()
	if err != nil {
		return err
	}
	chosen, err := a.ask("Installation directory", defaultRoot)
	if err != nil {
		return err
	}
	a.root, err = filepath.Abs(chosen)
	if err != nil {
		return fmt.Errorf("resolve installation directory: %w", err)
	}
	a.appDir = filepath.Join(a.root, "app")
	a.runtime = filepath.Join(a.root, ".crm-runtime")
	if err := os.MkdirAll(filepath.Join(a.root, "logs"), 0o700); err != nil {
		return fmt.Errorf("create installation directory: %w", err)
	}
	a.logFile, err = os.OpenFile(filepath.Join(a.root, "logs", "setup.log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return fmt.Errorf("create setup log: %w", err)
	}
	defer a.logFile.Close()
	log.SetOutput(a.logFile)
	log.Printf("launcher=%s os=%s arch=%s root=%q", version, runtime.GOOS, runtime.GOARCH, a.root)

	if err := a.step(ctx, 1, "Preparing application", a.prepare); err != nil {
		return err
	}
	if err := a.step(ctx, 2, "Company branding", a.brand); err != nil {
		return err
	}
	if err := a.step(ctx, 3, "Connecting Supabase", a.connectSupabase); err != nil {
		return err
	}
	if err := a.step(ctx, 4, "Creating CRM database", a.configureSupabase); err != nil {
		return err
	}
	if err := a.step(ctx, 5, "Connecting Vercel", a.connectVercel); err != nil {
		return err
	}
	if err := a.step(ctx, 6, "Deploying web application", a.deploy); err != nil {
		return err
	}

	fmt.Fprintf(a.out, "\nClient Interaction CRM is ready.\n\nWeb address:\n%s\n\nCompany:\n%s\n\nLocal files:\n%s\n\nNext:\nCreate or invite the first user in Supabase Dashboard, then sign in to the CRM.\n", a.finalURL, displayCompany(a.company), a.root)
	return nil
}

func (a *app) step(ctx context.Context, number int, title string, fn func(context.Context) error) error {
	fmt.Fprintf(a.out, "\n[%d/6] %s…\n", number, title)
	if err := fn(ctx); err != nil {
		if errors.Is(err, context.Canceled) {
			return errors.New("cancelled")
		}
		return fmt.Errorf("%s: %w", strings.ToLower(title), err)
	}
	return nil
}

func (a *app) ask(prompt, fallback string) (string, error) {
	if fallback != "" {
		fmt.Fprintf(a.out, "%s [%s]: ", prompt, fallback)
	} else {
		fmt.Fprintf(a.out, "%s: ", prompt)
	}
	value, err := a.in.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback, nil
	}
	return value, nil
}

func (a *app) confirm(prompt string, defaultYes bool) (bool, error) {
	hint := "y/N"
	if defaultYes {
		hint = "Y/n"
	}
	value, err := a.ask(prompt+" ("+hint+")", "")
	if err != nil || value == "" {
		return defaultYes, err
	}
	value = strings.ToLower(value)
	return value == "y" || value == "yes", nil
}

func defaultInstallDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find user home: %w", err)
	}
	return filepath.Join(home, "ClientInteractionCRM"), nil
}

func displayCompany(company string) string {
	if company == "" {
		return "Client Interaction CRM"
	}
	return company
}
