// Package config provides the configuration structures and loading logic for
// afon. Configuration is loaded from a YAML file (default: .afon.yaml) using
// viper, with values overridable via CLI flags.
package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

type (
	// Template holds the upstream template repository settings.
	Template struct {
		// Source is a local filesystem path or a remote Git URL pointing to the
		// upstream template repository.
		Source string `mapstructure:"source"`

		// Reference is the branch, tag, or commit SHA to check out when fetching a
		// remote template repository. Ignored for local paths.
		Reference string `mapstructure:"reference"`

		// Path is an optional path within the template repository that holds the
		// template files. When set, only files within this path are processed;
		// the path prefix is stripped from all output paths.
		Path string `mapstructure:"path"`

		// Token is a personal access token for authenticating against private
		// remote template repositories. If empty, the GITHUB_TOKEN environment
		// variable is used as a fallback when cloning.
		Token string `mapstructure:"token"`
	}

	// Config is the top-level configuration for an afon run.
	Config struct {
		// Template defines the upstream template repository to apply.
		Template Template `mapstructure:"template"`

		// Variables is a free-form map of values passed to the template engine as
		// the rendering context. Template authors access these as {{ .key }}.
		Variables map[string]any `mapstructure:"variables"`
	}
)

// Sentinel errors returned by Load and validate.
var (
	ErrConfigPathEmpty    = errors.New("configuration file path must not be empty")
	ErrConfigFileNotFound = errors.New("configuration file not found")
	ErrTemplateSrcEmpty   = errors.New("template.source must not be empty")
)

// Load reads and validates the configuration from the given YAML file path.
// It returns a non-nil *Config on success, or a descriptive error.
func Load(path string) (*Config, error) {
	if path == "" {
		return nil, ErrConfigPathEmpty
	}

	_, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrConfigFileNotFound, path)
	}

	v := viper.New()
	v.SetConfigFile(path)
	v.SetConfigType("yaml")

	err = v.ReadInConfig()
	if err != nil {
		return nil, fmt.Errorf("reading configuration file %s: %w", path, err)
	}

	cfg := &Config{}

	err = v.Unmarshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing configuration file %s: %w", path, err)
	}

	err = validate(cfg)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration in %s: %w", path, err)
	}

	return cfg, nil
}

// validate checks that the loaded Config contains the required fields.
func validate(cfg *Config) error {
	if cfg.Template.Source == "" {
		return ErrTemplateSrcEmpty
	}

	return nil
}
