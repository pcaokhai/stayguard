package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/pcaokhai/stayguard/api/internal/adapter/clock"
	"github.com/pcaokhai/stayguard/api/internal/adapter/postgres"
	"github.com/pcaokhai/stayguard/api/internal/app"
	"github.com/pcaokhai/stayguard/api/internal/platform/config"
)

const installerUsage = `usage:
  stayguard tenant import --file <file>.json
  stayguard sepay webhook --tenant <code> [--base-url https://domain]
  stayguard sepay set-secret --tenant <code> [--account <id>]
  stayguard sepay status --tenant <code>`

// runInstallerCLI is the installer's side of docs/runbooks/sepay-handover.md. It runs on the server with the
// application database role and never prints a secret.
func runInstallerCLI(ctx context.Context, args []string) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	pool, err := postgres.NewPool(ctx, postgres.PoolConfig{URL: cfg.DatabaseURL, AllowPrivileged: cfg.AllowPrivilegedDB})
	if err != nil {
		return fmt.Errorf("database: %w", err)
	}
	defer pool.Close()
	inst, err := newInstaller(cfg, pool, clock.System{})
	if err != nil {
		return err
	}
	return execInstaller(ctx, inst, args, os.Stdout, promptSecret)
}

// promptSecret reads the secret from the terminal without echo. A pipe or file is refused, so the secret
// can never come from an argument, an environment variable or a script.
func promptSecret() (string, error) {
	fd := int(os.Stdin.Fd()) //nolint:gosec // a file descriptor number
	if !term.IsTerminal(fd) {
		return "", errors.New("the secret is read from a terminal only")
	}
	fmt.Fprint(os.Stderr, "SePay webhook secret (hidden): ")
	b, err := term.ReadPassword(fd)
	fmt.Fprintln(os.Stderr)
	return strings.TrimSpace(string(b)), err
}

func execInstaller(ctx context.Context, inst *app.Installer, args []string, out io.Writer, readSecret func() (string, error)) error {
	if len(args) < 2 {
		return errors.New(installerUsage)
	}
	switch args[0] + " " + args[1] {
	case "tenant import":
		return cliImport(ctx, inst, args[2:], out)
	case "sepay webhook":
		return cliWebhook(ctx, inst, args[2:], out)
	case "sepay set-secret":
		return cliSetSecret(ctx, inst, args[2:], out, readSecret)
	case "sepay status":
		return cliStatus(ctx, inst, args[2:], out)
	}
	return errors.New(installerUsage)
}

// say writes one line of CLI output; a failed write to the terminal is not worth stopping for.
func say(w io.Writer, format string, args ...any) { _, _ = fmt.Fprintf(w, format, args...) }

func flags(name string, args []string, set func(*flag.FlagSet)) error {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	set(fs)
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("%s: %w\n%s", name, err, installerUsage)
	}
	return nil
}

func cliImport(ctx context.Context, inst *app.Installer, args []string, out io.Writer) error {
	var file string
	if err := flags("tenant import", args, func(fs *flag.FlagSet) { fs.StringVar(&file, "file", "", "import file") }); err != nil || file == "" {
		return errors.Join(err, errors.New("--file is required"))
	}
	raw, err := os.ReadFile(file) //nolint:gosec // the installer names the file
	if err != nil {
		return fmt.Errorf("read import file: %w", err)
	}
	res, err := inst.Import(ctx, raw)
	if err != nil {
		var ve *app.ValidationError
		if errors.As(err, &ve) { // field and reason only, never the file's values
			return fmt.Errorf("import file: %s: %s", ve.Field, ve.Reason)
		}
		return err
	}
	say(out, "tenant %s created, guesthouse code %s\n", res.TenantID, res.GuesthouseCode)
	say(out, "webhook path %s\n", res.HookPath)
	say(out, "%s\n", "one-time PINs, shown once, valid 24 hours (the person sets their own at first sign-in):")
	for _, p := range res.Pins {
		say(out, "  %-20s %s  until %s\n", p.Username, p.Pin, p.ExpiresAt.Local().Format("2006-01-02 15:04"))
	}
	say(out, "%s\n", "delete the import file now; keep it out of git.")
	return nil
}

func tenantFlag(name string, args []string, extra func(*flag.FlagSet)) (string, error) {
	var tenant string
	err := flags(name, args, func(fs *flag.FlagSet) {
		fs.StringVar(&tenant, "tenant", "", "guesthouse code")
		if extra != nil {
			extra(fs)
		}
	})
	if err == nil && tenant == "" {
		err = errors.New("--tenant is required")
	}
	return strings.ToLower(tenant), err
}

func cliWebhook(ctx context.Context, inst *app.Installer, args []string, out io.Writer) error {
	base := os.Getenv("PUBLIC_BASE_URL")
	tenant, err := tenantFlag("sepay webhook", args, func(fs *flag.FlagSet) { fs.StringVar(&base, "base-url", base, "public https address") })
	if err != nil {
		return err
	}
	path, err := inst.WebhookPath(ctx, tenant)
	if err != nil {
		return err
	}
	if base == "" {
		base = "https://[DOMAIN]"
	}
	say(out, "%s\n", strings.TrimRight(base, "/")+path)
	return nil
}

func cliSetSecret(ctx context.Context, inst *app.Installer, args []string, out io.Writer, readSecret func() (string, error)) error {
	var account string
	tenant, err := tenantFlag("sepay set-secret", args, func(fs *flag.FlagSet) { fs.StringVar(&account, "account", "", "bank account id") })
	if err != nil {
		return err
	}
	secret, err := readSecret()
	if err != nil {
		return err
	}
	if err = inst.SetSecret(ctx, tenant, secret, account); err != nil {
		return err
	}
	say(out, "%s\n", "secret stored (write-only). The owner sees: Installer updated SePay connection.")
	return nil
}

func cliStatus(ctx context.Context, inst *app.Installer, args []string, out io.Writer) error {
	tenant, err := tenantFlag("sepay status", args, nil)
	if err != nil {
		return err
	}
	st, err := inst.Status(ctx, tenant)
	if err != nil {
		return err
	}
	conn := "NOT_CONNECTED"
	if st.DefaultConnected {
		conn = "CONNECTED"
	}
	last, sig := "never", "no webhook yet"
	if st.LastWebhookAt != nil {
		last = st.LastWebhookAt.Local().Format("2006-01-02 15:04:05")
	}
	if st.SignatureOK != nil {
		sig = map[bool]string{true: "valid", false: "FAILED"}[*st.SignatureOK]
	}
	say(out, "connection: %s\nsecret stored: %t\nlast webhook: %s\nsignature check: %s\n", conn, st.HasSecret, last, sig)
	for _, a := range st.Accounts {
		say(out, "account %s %s %s default=%t %s\n", a.ID, a.BankName, a.AccountNoMasked, a.IsDefault, a.SepayStatus)
	}
	return nil
}
