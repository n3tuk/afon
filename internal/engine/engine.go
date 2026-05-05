// Package engine provides a template rendering engine backed by the Go standard
// library's text/template with sprig helper functions. A template file that
// renders to an empty or whitespace-only string signals that the corresponding
// output file should be skipped or removed.
package engine

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
)

// Engine renders Go templates using the sprig function library. A single Engine
// instance may be reused across multiple Render calls.
type Engine struct {
	funcMap template.FuncMap
}

// New creates a new Engine with the full sprig function set available to
// templates.
func New() *Engine {
	return &Engine{
		funcMap: sprig.TxtFuncMap(),
	}
}

// Render parses and executes a Go template from content, injecting vars as the
// template data context (accessible as {{ .key }}). It returns the rendered
// string and any parse or execution error.
//
// If the rendered output is empty after trimming whitespace, an empty string is
// returned. Callers may use IsEmpty to check whether the output should cause
// the corresponding file to be skipped or removed.
func (e *Engine) Render(name, content string, vars map[string]any) (string, error) {
	tmpl, err := template.New(name).Funcs(e.funcMap).Parse(content)
	if err != nil {
		return "", fmt.Errorf("parsing template %q: %w", name, err)
	}

	var buf bytes.Buffer

	err = tmpl.Execute(&buf, vars)
	if err != nil {
		return "", fmt.Errorf("executing template %q: %w", name, err)
	}

	return buf.String(), nil
}

// IsEmpty reports whether the given rendered output should be treated as
// "empty" — i.e. the output contains only whitespace after trimming. An empty
// result signals that the corresponding output file should be skipped or
// removed from the downstream repository.
func IsEmpty(output string) bool {
	return strings.TrimSpace(output) == ""
}
