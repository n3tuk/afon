package config_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/n3tuk/afon/internal/config"
)

func TestLoad(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		file    string
		want    *config.Config
		wantErr string
	}{
		{
			name: "valid minimal config",
			file: "testdata/minimal.yaml",
			want: &config.Config{
				Template: config.Template{
					Source: "https://github.com/org/template",
				},
				Variables: nil,
			},
		},
		{
			name: "valid full config",
			file: "testdata/full.yaml",
			want: &config.Config{
				Template: config.Template{
					Source:    "https://github.com/org/template",
					Reference: "main",
				},
				Variables: map[string]any{
					"project_name": "my-project",
					"go_version":   "1.24",
				},
			},
		},
		{
			name: "valid local path config",
			file: "testdata/local.yaml",
			want: &config.Config{
				Template: config.Template{
					Source: "/path/to/template",
				},
				Variables: nil,
			},
		},
		{
			name:    "missing source",
			file:    "testdata/missing_source.yaml",
			wantErr: "template.source must not be empty",
		},
		{
			name:    "empty path",
			file:    "",
			wantErr: "configuration file path must not be empty",
		},
		{
			name:    "file not found",
			file:    "testdata/nonexistent.yaml",
			wantErr: "configuration file not found",
		},
		{
			name:    "invalid yaml",
			file:    "testdata/invalid.yaml",
			wantErr: "reading configuration file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := config.Load(tt.file)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Load(%q) expected error containing %q, got nil", tt.file, tt.wantErr)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Load(%q) error = %q, want to contain %q", tt.file, err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("Load(%q) unexpected error: %v", tt.file, err)
			}

			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("Load(%q) mismatch (-want +got):\n%s", filepath.Base(tt.file), diff)
			}
		})
	}
}
