// Package app puts gist together: the commands and their flags, the log, and
// what each command is given to run with.
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/cli/go-gh/v2/pkg/term"
	"github.com/spf13/cobra"

	"github.com/babarot/gist/v2/internal/cli"
	"github.com/babarot/gist/v2/internal/config"
	"github.com/babarot/gist/v2/internal/edit"
	"github.com/babarot/gist/v2/internal/github"
	"github.com/babarot/gist/v2/internal/logging"
	"github.com/babarot/gist/v2/internal/model"
	"github.com/babarot/gist/v2/internal/release"
	"github.com/babarot/gist/v2/internal/state"
	"github.com/babarot/gist/v2/internal/tui"
	"github.com/babarot/gist/v2/internal/tui/keymap"
	"github.com/babarot/gist/v2/internal/version"
	"github.com/babarot/gist/v2/internal/xdg"
)

// EnvLogLevel names the log level, below --debug.
const EnvLogLevel = "GIST_LOG_LEVEL"

// The values of --debug. A bare --debug is debugOn.
const (
	debugOn   = "on"
	debugLive = "live"
)

// options are the global flags, and what is opened from them for a command
// to run with.
type options struct {
	config  string
	debug   string
	noColor bool

	log      *slog.Logger
	closeLog func()
	// newClient makes the client; tests give another.
	newClient func(log *slog.Logger, token string) (*github.Client, error)
}

// exitCode is what a command returns once it has said why it failed, so that
// Run exits with the code and prints nothing more. Any other error comes from
// a wrong call.
type exitCode int

func (c exitCode) Error() string { return fmt.Sprintf("exit status %d", int(c)) }

// exited is code as what a command returns.
func exited(code int) error {
	if code == 0 {
		return nil
	}
	return exitCode(code)
}

// Run runs gist with args (without the program name) and returns the exit
// code: 1 when a command failed, 2 when it was called wrongly.
func Run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	return run(&options{newClient: newClient}, args, stdin, stdout, stderr)
}

func newClient(log *slog.Logger, token string) (*github.Client, error) {
	return github.New(log, github.Options{Token: token})
}

func run(o *options, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	o.log, o.closeLog = logging.Discard(), func() {}
	defer func() { o.closeLog() }()
	root := newRootCommand(o, stdin, stdout, stderr)
	root.SetArgs(args)
	cmd, err := root.ExecuteContextC(context.Background())
	if err == nil {
		return 0
	}
	if code, ok := errors.AsType[exitCode](err); ok {
		return int(code)
	}
	fmt.Fprintf(stderr, "gist: %v\n", err)
	fmt.Fprint(stderr, cmd.UsageString())
	return 2
}

// newRootCommand is gist with its commands, writing to stdout and stderr.
func newRootCommand(o *options, stdin io.Reader, stdout, stderr io.Writer) *cobra.Command {
	root := &cobra.Command{
		Short:         "edit your GitHub Gists from the terminal",
		SilenceErrors: true,
		SilenceUsage:  true,
		Use:           "gist [gist]",
		Version:       version.String(),
		Args: func(_ *cobra.Command, args []string) error {
			// On a terminal an argument is a gist to open
			if onTerminal := isTerminal(stdin) && isTerminal(stdout); len(args) > 0 && !onTerminal {
				return fmt.Errorf("unknown command %q", args[0])
			}
			return cobra.MaximumNArgs(1)(nil, args)
		},
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			return o.prepare(cmd, stderr)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if o.debug == debugLive {
				return followLog(stdout, stderr)
			}
			if isTerminal(stdin) && isTerminal(stdout) {
				return o.runTUI(cmd, at(args, 0), stdout, stderr)
			}
			return o.run(cmd, stdin, stdout, stderr, func(ctx context.Context, c *cli.CLI) error {
				return c.List(ctx, cli.ListOptions{})
			})
		},
	}
	root.SetOut(stdout)
	root.SetErr(stderr)
	// The same line as gist version prints
	root.SetVersionTemplate("{{.Version}}\n")

	flags := root.PersistentFlags()
	flags.StringVar(&o.config, "config", filepath.Join(xdg.ConfigDir(), "config.toml"), "the config file")
	flags.StringVar(&o.debug, "debug", "", "log at debug level, or with =live follow the log from another terminal")
	flags.Lookup("debug").NoOptDefVal = debugOn
	flags.BoolVar(&o.noColor, "no-color", false, "print no colors (the NO_COLOR environment variable does the same)")

	root.AddCommand(
		newVersionCommand(stdout, stderr),
		newLogCommand(stdout),
		newConfigCommand(o, stdout),
		newAuthCommand(o, stdout, stderr),
		newListCommand(o, stdin, stdout, stderr),
		newViewCommand(o, stdin, stdout, stderr),
		newNewCommand(o, stdin, stdout, stderr),
		newEditCommand(o, stdin, stdout, stderr),
		newDeleteCommand(o, stdin, stdout, stderr),
		newOpenCommand(o, stdin, stdout, stderr),
		newCloneCommand(o, stdin, stdout, stderr),
	)
	return root
}

func newVersionCommand(stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "print the version and revision",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if _, err := fmt.Fprintln(stdout, version.String()); err != nil {
				return err
			}
			// Only what was found before: this reads nothing
			seen := state.Open(xdg.StateDir()).ReleaseSeen()
			if version.IsRelease() && release.Newer(seen.Latest, version.Version) {
				fmt.Fprintf(stderr, "gist %s is out · %s\n", seen.Latest, updateHow())
			}
			return nil
		},
	}
}

func newLogCommand(stdout io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "print where the log file is",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintln(stdout, logging.DefaultPath())
			return err
		},
	}
}

func newConfigCommand(o *options, stdout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "look at the config file",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "check [path]",
		Short: "read the config file and print its mistakes, without starting; exits 1 when it has some",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			path := o.config
			if len(args) == 1 {
				path = args[0]
			}
			return exited(checkConfig(path, stdout))
		},
	})
	return cmd
}

func newAuthCommand(o *options, stdout, stderr io.Writer) *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "print where the token came from and what it can do",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, source, err := o.client()
			if err != nil {
				fmt.Fprintf(stderr, "gist: %v\n", err)
				return exitCode(1)
			}
			return exited(cli.Auth(cmd.Context(), stdout, stderr, source, client))
		},
	}
}

// client makes the client with the token found, and says where it came from.
func (o *options) client() (*github.Client, string, error) {
	token, source := github.Token()
	if token == "" {
		return nil, "", github.ErrNoToken
	}
	client, err := o.newClient(o.log, token)
	return client, source, err
}

func (o *options) newCLI(client *github.Client, stdin io.Reader, stdout, stderr io.Writer) *cli.CLI {
	return &cli.CLI{
		Stdin: stdin, Stdout: stdout, Stderr: stderr,
		StdinTTY: isTerminal(stdin), StdoutTTY: isTerminal(stdout),
		API:      client,
		Editor:   &edit.Editor{API: client, State: state.Open(xdg.StateDir())},
		Log:      o.log,
		Open:     func(url string) error { return github.Browse(url, stdout, stderr) },
		Run:      runProgram,
		CacheDir: xdg.CacheDir(),
		Now:      time.Now,
	}
}

// run runs f with what a command needs, and turns what it returns into the
// exit code: 130 when the user said no, 1 when it failed.
func (o *options) run(cmd *cobra.Command, stdin io.Reader, stdout, stderr io.Writer, f func(context.Context, *cli.CLI) error) error {
	client, _, err := o.client()
	if err == nil {
		err = f(cmd.Context(), o.newCLI(client, stdin, stdout, stderr))
	}
	switch {
	case err == nil:
		return nil
	case errors.Is(err, cli.ErrCanceled):
		return exitCode(130)
	}
	fmt.Fprintf(stderr, "gist: %v\n", err)
	return exitCode(1)
}

// readConfig reads the config file, writing a commented one on first run. Not
// being able to write it stops nothing.
func (o *options) readConfig() (config.File, error) {
	if err := config.WriteTemplate(o.config); err != nil {
		o.log.Warn("the config file cannot be written", "err", err.Error())
	}
	return loadConfig(o.config)
}

// loadConfig reads the file at path, its keys too, and shows all that is wrong
// in it, where it is.
func loadConfig(path string) (config.File, error) {
	cfg, err := config.Load(path)
	var ps config.Problems
	if err != nil && !errors.As(err, &ps) {
		return cfg, err
	}
	if _, err := keymap.Default().With(cfg.Keys); err != nil {
		var keys keymap.Problems
		if !errors.As(err, &keys) {
			return cfg, err
		}
		for _, p := range keys {
			ps = append(ps, config.Problem{Key: []string{"keys", p.Action}, Index: p.Index, AtKey: p.AtKey, Message: p.Message})
		}
	}
	if len(ps) > 0 {
		return cfg, errors.New(config.Report(path, ps))
	}
	return cfg, nil
}

// checkConfig prints what is wrong in the file at path, and gives the exit code.
func checkConfig(path string, stdout io.Writer) int {
	shown := config.TildePath(path)
	if _, err := os.Stat(path); errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintf(stdout, "%s is not there; the defaults are used\n", shown)
		return 0
	}
	if _, err := loadConfig(path); err != nil {
		fmt.Fprintln(stdout, err)
		return 1
	}
	fmt.Fprintf(stdout, "%s: no mistakes\n", shown)
	return 0
}

// runTUI runs the TUI, on the gist arg names when it names one.
func (o *options) runTUI(cmd *cobra.Command, arg string, stdout, stderr io.Writer) error {
	fail := func(err error) error {
		fmt.Fprintf(stderr, "gist: %v\n", err)
		return exitCode(1)
	}
	cfg, err := o.readConfig()
	if err != nil {
		return fail(err)
	}
	keys, err := keymap.Default().With(cfg.Keys)
	if err != nil {
		return fail(err)
	}
	store := state.Open(xdg.StateDir())
	opt := tui.Options{
		Host: github.Host, CacheDir: xdg.CacheDir(), Layout: store, Config: cfg, Keys: keys, Log: o.log,
		Browse: func(url string) error { return github.Browse(url, io.Discard, io.Discard) },
	}
	client, source, err := o.client()
	if err == nil {
		opt.Release = releaseOptions(&releaseSource{client, store}, os.Getenv(EnvNoUpdateCheck) == "", time.Now())
	}
	switch {
	case errors.Is(err, github.ErrNoToken):
		opt.NoToken = true
	case err != nil:
		return fail(err)
	default:
		opt.Source, opt.Via = client, github.Source(source)
		opt.Editor = &edit.Editor{API: client, State: store}
	}
	if arg != "" && client != nil {
		c := o.newCLI(client, nil, stdout, stderr)
		if opt.Select, err = c.Resolve(cmd.Context(), arg); err != nil {
			return fail(err)
		}
	}
	if _, err := tui.Run(opt); err != nil {
		return fail(err)
	}
	return nil
}

// runForm runs the form of a new gist alone, and gives the gist it made.
func (o *options) runForm(c *cli.CLI, cfg config.File, n model.NewGist) (*model.Gist, error) {
	keys, err := keymap.Default().With(cfg.Keys)
	if err != nil {
		return nil, err
	}
	res, err := tui.Run(tui.Options{
		Source: c.API, FormOnly: true, Host: github.Host, CacheDir: xdg.CacheDir(),
		Config: cfg, Keys: keys, Log: o.log, NewGist: &n,
	})
	return res.Created, err
}

// isTerminal says v is a terminal.
func isTerminal(v any) bool {
	f, ok := v.(*os.File)
	return ok && term.IsTerminal(f)
}

// runProgram runs a program on the user's terminal.
func runProgram(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// prepare checks the global flags and opens the log. A log that cannot be
// opened does not stop the command, which does not depend on it.
func (o *options) prepare(cmd *cobra.Command, stderr io.Writer) error {
	switch o.debug {
	case "", debugOn:
	case debugLive:
		if cmd.HasParent() {
			return errors.New("--debug=live takes no command")
		}
		return nil
	default:
		return fmt.Errorf("invalid --debug=%s: use --debug or --debug=live", o.debug)
	}
	level, err := o.logLevel()
	if err != nil {
		fmt.Fprintf(stderr, "gist: %s: %v\n", EnvLogLevel, err)
		return exitCode(1)
	}
	log, closer, err := logging.Open(logging.Options{Level: level})
	if err != nil {
		fmt.Fprintf(stderr, "gist: writing no log: %v\n", err)
		return nil
	}
	started := time.Now()
	log = log.With(logging.KeyRun, logging.NewID())
	log.Info("start", logging.KeyVersion, version.String(), "command", cmd.CommandPath(),
		"config", o.config, "state", xdg.StateDir(), "cache", xdg.CacheDir())
	o.log = log
	o.closeLog = func() {
		log.Info("exit", logging.KeyDurationMS, time.Since(started).Milliseconds())
		if err := closer.Close(); err != nil {
			fmt.Fprintf(stderr, "gist: closing the log: %v\n", err)
		}
	}
	return nil
}

// logLevel is the level --debug or the environment asks for.
func (o *options) logLevel() (slog.Level, error) {
	if o.debug == debugOn {
		return slog.LevelDebug, nil
	}
	return logging.ParseLevel(os.Getenv(EnvLogLevel))
}

// followLog prints what is added to the log until the user interrupts it.
func followLog(stdout, stderr io.Writer) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := logging.Follow(ctx, stdout, logging.DefaultPath()); err != nil {
		fmt.Fprintf(stderr, "gist: following the log: %v\n", err)
		return exitCode(1)
	}
	return nil
}
