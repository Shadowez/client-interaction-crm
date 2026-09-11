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
	in                 *bufio.Reader
	out                io.Writer
	verbose            bool
	logFile            *os.File
	root               string
	appDir             string
	runtime            string
	node               string
	npm                string
	npx                string
	npmCache           string
	portableNodeOwned  bool
	company            string
	project            project
	databasePassword   string
	finalURL           string
	deploymentURL      string
	deploymentID       string
	deploymentReady    bool
	supabaseRun        func(context.Context, []string, commandOptions) (string, error)
	vercelRun          func(context.Context, []string, commandOptions) (string, error)
	browserOpen        func(context.Context, string) error
	resuming           bool
	supabaseConfigured bool
	requestedRoot      string
}

func main() {
	verbose := flag.Bool("verbose", false, "show detailed command output")
	showVersion := flag.Bool("version", false, "print launcher version")
	installDir := flag.String("install-dir", "", "use a specific installation directory")
	flag.Parse()
	if *showVersion {
		fmt.Printf("Client Interaction CRM Setup %s\n", version)
		return
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	a := &app{in: bufio.NewReader(os.Stdin), out: os.Stdout, verbose: *verbose, requestedRoot: *installDir}
	if err := a.run(ctx); err != nil {
		logPath := ""
		if a.logFile != nil {
			logPath = a.logFile.Name()
		}
		reportSetupFailure(os.Stderr, a.in, runtime.GOOS, err, logPath)
		os.Exit(1)
	}
}

func reportSetupFailure(out io.Writer, in *bufio.Reader, goos string, err error, logPath string) {
	fmt.Fprintf(out, "\nSetup could not continue.\nReason: %v\n", err)
	if logPath != "" {
		fmt.Fprintf(out, "Troubleshooting log: %s\n", logPath)
	}
	fmt.Fprintln(out, "You can safely run setup again to resume.")
	if goos == "windows" {
		fmt.Fprint(out, "Press Enter to close.")
		if in != nil {
			_, _ = in.ReadString('\n')
		}
	}
}

func (a *app) run(ctx context.Context) error {
	fmt.Fprintln(a.out, "Client Interaction CRM Setup")
	fmt.Fprintln(a.out, "This launcher installs local setup files and deploys a private CRM for one trusted team.")

	defaultRoot, err := defaultInstallDir()
	if err != nil {
		return err
	}
	chosen := ""
	if strings.TrimSpace(a.requestedRoot) != "" {
		chosen = a.requestedRoot
		if validInstallDir(chosen) {
			a.resuming = true
			fmt.Fprintf(a.out, "\nPrepared Client Interaction CRM setup found: %s\n", chosen)
			fmt.Fprintln(a.out, "Reusing the prepared application and saved non-secret setup state.")
		}
	} else if previous, previousErr := loadInstallPointer(); previousErr == nil && validInstallDir(previous) {
		fmt.Fprintf(a.out, "\nPrevious Client Interaction CRM setup found: %s\n", previous)
		resume, confirmErr := a.confirm("Resume this setup?", true)
		if confirmErr != nil {
			return confirmErr
		}
		if resume {
			chosen = previous
			a.resuming = true
			fmt.Fprintf(a.out, "Reusing the prepared application and saved non-secret setup state in %s.\n", previous)
		}
	}
	if chosen == "" {
		chosen, err = a.ask("Installation directory", defaultRoot)
	}
	if err != nil {
		return err
	}
	a.root, err = filepath.Abs(chosen)
	if err != nil {
		return fmt.Errorf("resolve installation directory: %w", err)
	}
	a.appDir = filepath.Join(a.root, "app")
	a.runtime = filepath.Join(a.root, ".crm-runtime")
	a.npmCache, err = crmNPMCacheDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(a.root, "logs"), 0o700); err != nil {
		return fmt.Errorf("create installation directory: %w", err)
	}
	a.logFile, err = openSetupLog(filepath.Join(a.root, "logs", "setup.log"))
	if err != nil {
		return fmt.Errorf("create setup log: %w", err)
	}
	defer a.logFile.Close()
	log.SetOutput(a.logFile)
	log.Printf("launcher=%s os=%s arch=%s root=%q", version, runtime.GOOS, runtime.GOARCH, a.root)

	if err := a.step(ctx, 1, "Preparing application", a.prepare); err != nil {
		return err
	}
	if err := saveInstallPointer(a.root); err != nil {
		log.Printf("could not save last installation pointer: %v", err)
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
	if err := a.finalizeLifecycleFiles(); err != nil {
		return fmt.Errorf("write local uninstall information: %w", err)
	}

	fmt.Fprintf(a.out, "\nSetup complete.\n\nWeb address:\n%s\n\nCompany:\n%s\n\nLocal files:\n%s\n\nOpen your CRM and sign in using the user you just created or invited.\n", a.finalURL, displayCompany(a.company), a.root)
	return nil
}

func openSetupLog(path string) (*os.File, error) {
	// Keep diagnostics for this run only. This also removes sensitive material
	// written by older launcher versions as soon as the corrected launcher runs.
	return os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
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
