// Package main is the entry point for the afon CLI. Build-time metadata is
// injected via ldflags by goreleaser:
//
//	-X 'main.Branch=...'
//	-X 'main.Commit=...'
//	-X 'main.Version=...'
//	-X 'main.BuildDate=...'
//	-X 'main.Architecture=...'
package main

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/n3tuk/afon/internal/apply"
	"github.com/n3tuk/afon/internal/config"
	"github.com/n3tuk/afon/internal/repository"
)

// Build-time variables injected by goreleaser ldflags.
//
//nolint:gochecknoglobals // populated by goreleaser ldflags at build time
var (
	Branch       = "unknown"
	Commit       = "unknown"
	Version      = "unknown"
	BuildDate    = "unknown"
	Architecture = "unknown"
)

func main() {
	err := newRootCommand().Execute()
	if err != nil {
		os.Exit(1)
	}
}

// newRootCommand constructs the root cobra command which serves as the afon
// CLI entry point.
func newRootCommand() *cobra.Command {
	var logLevel string

	// Default to debug when running in GitHub Actions with step debugging on.
	defaultLogLevel := "info"
	if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("ACTIONS_STEP_DEBUG") == "true" {
		defaultLogLevel = "debug"
	}

	root := &cobra.Command{
		Use:   "afon",
		Short: "Apply an upstream template repository to the current repository",
		Long: strings.TrimSpace(`
afon applies files from an upstream template repository (local path or remote
Git URL) to the current directory, rendering Go template files (.tmpl/.t) with
user-supplied variables.
		`),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			return setupLogging(logLevel)
		},
	}

	root.PersistentFlags().StringVar(&logLevel, "log-level", defaultLogLevel,
		"Log level: debug, info, warn, or error")

	root.AddCommand(newApplyCommand())
	root.AddCommand(newVersionCommand())

	return root
}

// setupLogging initialises the default slog logger at the requested level,
// writing structured text output to stderr.
func setupLogging(level string) error {
	var l slog.Level

	err := l.UnmarshalText([]byte(level))
	if err != nil {
		return fmt.Errorf("invalid log level %q: %w", level, err)
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: l})))

	return nil
}

// newApplyCommand constructs the "apply" subcommand.
func newApplyCommand() *cobra.Command {
	v := viper.New()

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply the upstream template to the current repository",
		Long: strings.TrimSpace(`
Apply reads a configuration file (default: .afon.yaml in the current directory)
and applies the upstream template repository to the current directory.

Template files (.tmpl/.t) are rendered using Go's text/template engine with the
sprig function library. A template that renders to an empty or whitespace-only
string causes the corresponding output file to be skipped (if it does not yet
exist) or removed (if it does).
		`),
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runApply(cmd, v)
		},
	}

	cmd.Flags().StringP("config", "c", ".afon.yaml", "Path to the afon configuration file")
	cmd.Flags().StringP("template", "t", "", "Path or URL to the upstream template repository")
	cmd.Flags().StringP("reference", "r", "", "Branch, tag, or commit reference for remote templates")
	cmd.Flags().StringP("output", "o", ".", "Output directory (defaults to current directory)")
	cmd.Flags().StringP("path", "p", "", "Path within the template repository to process")
	cmd.Flags().String("token", "", "Personal access token for private template repositories (overrides GITHUB_TOKEN)")

	err := v.BindPFlags(cmd.Flags())
	if err != nil {
		// BindPFlags only fails on programmer error; panic is appropriate here.
		panic(fmt.Sprintf("afon: binding flags: %v", err))
	}

	return cmd
}

// runApply executes the apply logic: load config, select repository, apply.
func runApply(_ *cobra.Command, v *viper.Viper) error {
	slog.Info("afon starting",
		"version", Version,
		"commit", Commit,
		"build-date", BuildDate,
		"architecture", Architecture,
	)

	cfg, err := config.Load(v.GetString("config"))
	if err != nil {
		return fmt.Errorf("loading configuration: %w", err)
	}

	// Flag overrides take precedence over the config file.
	if t := v.GetString("template"); t != "" {
		cfg.Template.Source = t
	}

	if r := v.GetString("reference"); r != "" {
		cfg.Template.Reference = r
	}

	if tok := v.GetString("token"); tok != "" {
		cfg.Template.Token = tok
	}

	if p := v.GetString("path"); p != "" {
		cfg.Template.Path = p
	}

	slog.Info("applying template repository",
		"source", cfg.Template.Source,
		"path", cfg.Template.Path,
	)

	repo, err := selectRepository(cfg)
	if err != nil {
		return err
	}

	outputDir := v.GetString("output")

	applier, err := apply.New(outputDir)
	if err != nil {
		return fmt.Errorf("initialising applier: %w", err)
	}

	err = applier.Apply(repo, cfg.Template.Path, cfg.Variables)
	if err != nil {
		return fmt.Errorf("applying template: %w", err)
	}

	return nil
}

// selectRepository decides whether to use a local or remote repository based
// on the template source string. A source starting with a URL scheme
// (https://, http://, git://, or ssh://) or containing a '@' (SSH shorthand)
// is treated as remote; everything else is treated as a local path.
//
//nolint:ireturn // returns the applicable repository implementation via the interface type
func selectRepository(cfg *config.Config) (repository.Repository, error) {
	source := cfg.Template.Source

	if isRemoteURL(source) {
		repo, err := repository.NewRemote(source, cfg.Template.Reference, cfg.Template.Token)
		if err != nil {
			return nil, fmt.Errorf("configuring remote repository: %w", err)
		}

		return repo, nil
	}

	repo, err := repository.NewLocal(source)
	if err != nil {
		return nil, fmt.Errorf("configuring local repository: %w", err)
	}

	return repo, nil
}

// isRemoteURL reports whether source looks like a remote Git URL. It uses
// net/url.Parse to detect scheme-based URLs (https, http, git, ssh).
func isRemoteURL(source string) bool {
	u, err := url.Parse(source)
	if err == nil && u.Scheme != "" && u.Host != "" {
		switch u.Scheme {
		case "https", "http", "git", "ssh":
			return true
		}
	}

	return false
}

// newVersionCommand constructs the "version" subcommand.
func newVersionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Fprintf(os.Stdout, "afon %s (%s) built %s [%s@%s]\n",
				Version, Architecture, BuildDate, Branch, Commit,
			)
		},
	}
}
