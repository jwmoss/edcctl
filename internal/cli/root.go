package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"

	"github.com/jwmoss/edcctl/internal/api"
	"github.com/jwmoss/edcctl/internal/config"
	"github.com/jwmoss/edcctl/internal/output"
)

const (
	exitOK    = 0
	exitErr   = 1
	exitUsage = 2
)

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

type globals struct {
	configPath  string
	baseURL     string
	accountID   string
	asJSON      bool
	plain       bool
	quiet       bool
	noColor     bool
	showVersion bool
	timeout     time.Duration
	traceHTTP   bool
	dryRun      bool
	noInput     bool
}

type runtime struct {
	ctx    context.Context
	stdin  io.Reader
	stdout io.Writer
	stderr io.Writer
	g      *globals
	cfg    *config.Config
	out    *output.Formatter
	client *api.Client
	app    *api.AppClient
}

func Execute(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	rc := &runtime{
		ctx:    ctx,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		g:      &globals{timeout: 30 * time.Second},
	}

	cmd := newRootCommand(rc)
	cmd.SetArgs(args)
	cmd.SetIn(stdin)
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)

	if err := cmd.ExecuteContext(ctx); err != nil {
		if !errors.Is(err, errSilent) {
			_, _ = fmt.Fprintf(stderr, "error: %v\n", err)
		}
		if errors.Is(err, errUsage) {
			return exitUsage
		}
		return exitErr
	}
	return exitOK
}

func newRootCommand(rc *runtime) *cobra.Command {
	root := &cobra.Command{
		Use:           "edcctl",
		Short:         "Command-line client for the Evolution Dance Complex Studio Pro parent portal",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			if rc.g.showVersion {
				return rc.writeVersion()
			}
			_ = cmd.Help()
			return errUsage
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if rc.g.showVersion || commandSkipsClient(cmd) {
				return rc.initOutput()
			}
			return rc.initClient()
		},
	}
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return fmt.Errorf("%w: %v", errUsage, err)
	})
	root.Flags().BoolVar(&rc.g.showVersion, "version", false, "print version and exit")

	flags := root.PersistentFlags()
	flags.StringVar(&rc.g.configPath, "config", "", "config file path")
	flags.StringVar(&rc.g.baseURL, "base-url", "", "portal base URL override")
	flags.StringVar(&rc.g.accountID, "account-id", "", "Studio Pro account ID override")
	flags.BoolVar(&rc.g.asJSON, "json", false, "emit JSON to stdout")
	flags.BoolVar(&rc.g.plain, "plain", false, "emit stable plain text where available")
	flags.BoolVarP(&rc.g.quiet, "quiet", "q", false, "suppress non-essential output")
	flags.BoolVar(&rc.g.noColor, "no-color", false, "disable color")
	flags.DurationVar(&rc.g.timeout, "timeout", 30*time.Second, "HTTP timeout")
	flags.BoolVar(&rc.g.traceHTTP, "trace-http", false, "log HTTP requests to stderr without secrets")
	flags.BoolVar(&rc.g.dryRun, "dry-run", false, "refuse non-GET HTTP requests")
	flags.BoolVar(&rc.g.noInput, "no-input", false, "disable interactive prompts")

	root.AddCommand(newVersionCommand(rc))
	root.AddCommand(newConfigCommand(rc))
	root.AddCommand(newLoginCommand(rc))
	root.AddCommand(newDoctorCommand(rc))
	root.AddCommand(newScheduleCommand(rc))
	root.AddCommand(newBalanceCommand(rc))
	root.AddCommand(newHistoryCommand(rc))
	root.AddCommand(newAppCommand(rc))
	root.AddCommand(newCompletionCommand(root))

	return root
}

func (rc *runtime) initOutput() error {
	if rc.g.asJSON && rc.g.plain {
		return fmt.Errorf("%w: choose only one of --json or --plain", errUsage)
	}
	rc.out = output.New(rc.stdout, rc.stderr, rc.g.asJSON, rc.g.plain, rc.g.quiet, rc.g.noColor)
	return nil
}

func (rc *runtime) initClient() error {
	if err := rc.initOutput(); err != nil {
		return err
	}
	cfg, err := rc.loadConfig()
	if err != nil {
		return err
	}
	rc.cfg = cfg
	v, _, _ := currentVersion()
	options := []api.Option{
		api.WithTimeout(rc.g.timeout),
		api.WithDryRun(rc.g.dryRun),
		api.WithUserAgent("edcctl/" + v),
	}
	if rc.g.traceHTTP {
		options = append(options, api.WithTrace(func(method, path string, status int, duration time.Duration) {
			_, _ = fmt.Fprintf(rc.stderr, "[http] %s %s -> %d (%s)\n", method, path, status, duration)
		}))
	}
	rc.client = api.New(cfg.BaseURL, cfg.AccountID, options...)
	rc.app = api.NewApp(options...)
	return rc.client.Login(rc.ctx, cfg.Email, cfg.Password)
}

func (rc *runtime) loadConfig() (*config.Config, error) {
	cfg, err := config.Load(rc.g.configPath)
	if err != nil {
		return nil, err
	}
	if rc.g.baseURL != "" {
		cfg.BaseURL = rc.g.baseURL
	}
	if rc.g.accountID != "" {
		cfg.AccountID = rc.g.accountID
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func commandSkipsClient(cmd *cobra.Command) bool {
	for cmd != nil {
		switch cmd.Name() {
		case "completion", "config", "help", "version":
			return true
		}
		cmd = cmd.Parent()
	}
	return false
}

func newVersionCommand(rc *runtime) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  usageArgs(cobra.NoArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			return rc.writeVersion()
		},
	}
}

func currentVersion() (string, string, string) {
	v, c, d := version, commit, date
	if info, ok := debug.ReadBuildInfo(); ok {
		if v == "dev" && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		}
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if c == "unknown" {
					c = setting.Value
				}
			case "vcs.time":
				if d == "unknown" {
					d = setting.Value
				}
			}
		}
	}
	return v, c, d
}

func (rc *runtime) writeVersion() error {
	v, c, d := currentVersion()
	payload := map[string]string{
		"version": v,
		"commit":  c,
		"date":    d,
	}
	if rc.out.IsJSON() {
		return rc.out.JSON(payload)
	}
	if rc.out.IsPlain() {
		rc.out.Printf("%s\n", v)
		return nil
	}
	rc.out.Printf("edcctl version %s\n", v)
	rc.out.Printf("commit: %s\n", c)
	rc.out.Printf("built:  %s\n", d)
	return nil
}

func newCompletionCommand(root *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Args:  usageArgs(cobra.ExactArgs(1)),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletion(cmd.OutOrStdout())
			default:
				return fmt.Errorf("%w: unsupported shell %q", errUsage, args[0])
			}
		},
	}
	return cmd
}

var (
	errUsage  = errors.New("invalid usage")
	errSilent = errors.New("silent")
)

func usageArgs(fn cobra.PositionalArgs) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := fn(cmd, args); err != nil {
			return fmt.Errorf("%w: %v", errUsage, err)
		}
		return nil
	}
}

func apiExitCode(err error) int {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		if apiErr.Status == http.StatusUnauthorized || apiErr.Status == http.StatusForbidden {
			return exitErr
		}
	}
	return exitErr
}

func init() {
	cobra.EnableCommandSorting = false
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
