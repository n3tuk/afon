package engine_test

import (
	"strings"
	"testing"

	"github.com/n3tuk/afon/internal/engine"
)

func TestEngine_Render(t *testing.T) {
	t.Parallel()

	e := engine.New()

	tests := []struct {
		name    string
		tmpl    string
		content string
		vars    map[string]any
		want    string
		wantErr string
	}{
		{
			name:    "simple substitution",
			tmpl:    "hello",
			content: "Hello, {{ .name }}!",
			vars:    map[string]any{"name": "World"},
			want:    "Hello, World!",
		},
		{
			name:    "no variables",
			tmpl:    "static",
			content: "no variables here",
			vars:    nil,
			want:    "no variables here",
		},
		{
			name:    "empty template",
			tmpl:    "empty",
			content: "",
			vars:    nil,
			want:    "",
		},
		{
			name:    "whitespace only",
			tmpl:    "whitespace",
			content: "   \n\t  ",
			vars:    nil,
			want:    "   \n\t  ",
		},
		{
			name:    "conditional empty output",
			tmpl:    "conditional",
			content: "{{- if .enabled }}content{{- end }}",
			vars:    map[string]any{"enabled": false},
			want:    "",
		},
		{
			name:    "conditional with content",
			tmpl:    "conditional-true",
			content: "{{- if .enabled }}content{{- end }}",
			vars:    map[string]any{"enabled": true},
			want:    "content",
		},
		{
			name:    "sprig function",
			tmpl:    "sprig-upper",
			content: `{{ .name | upper }}`,
			vars:    map[string]any{"name": "hello"},
			want:    "HELLO",
		},
		{
			name:    "sprig default function",
			tmpl:    "sprig-default",
			content: `{{ .missing | default "fallback" }}`,
			vars:    map[string]any{},
			want:    "fallback",
		},
		{
			name:    "multiline output",
			tmpl:    "multiline",
			content: "line1\nline2\nline3",
			vars:    nil,
			want:    "line1\nline2\nline3",
		},
		{
			name:    "invalid template syntax",
			tmpl:    "bad",
			content: "{{ .name",
			wantErr: "parsing template",
		},
		{
			name:    "template execution error",
			tmpl:    "exec-err",
			content: `{{ call .fn }}`,
			vars:    map[string]any{"fn": "not-a-func"},
			wantErr: "executing template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := e.Render(tt.tmpl, tt.content, tt.vars)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Render() expected error containing %q, got nil", tt.wantErr)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("Render() error = %q, want to contain %q", err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("Render() unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("Render() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsEmpty(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{name: "empty string", output: "", want: true},
		{name: "whitespace only", output: "   ", want: true},
		{name: "newlines only", output: "\n\n", want: true},
		{name: "tab only", output: "\t", want: true},
		{name: "mixed whitespace", output: "  \n\t  ", want: true},
		{name: "non-empty", output: "content", want: false},
		{name: "content with whitespace", output: "  content  ", want: false},
		{name: "newline with content", output: "\ncontent\n", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := engine.IsEmpty(tt.output); got != tt.want {
				t.Errorf("IsEmpty(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}
