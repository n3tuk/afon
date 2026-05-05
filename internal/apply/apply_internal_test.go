package apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestOutputPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want string
	}{
		{name: ".tmpl extension", path: "file.txt.tmpl", want: "file.txt"},
		{name: ".t extension", path: "config.yaml.t", want: "config.yaml"},
		{name: "uppercase .TMPL", path: "file.TMPL", want: "file"},
		{name: "uppercase .T", path: "file.T", want: "file"},
		{name: "no template extension", path: "static.txt", want: "static.txt"},
		{name: "nested path tmpl", path: "a/b/c.txt.tmpl", want: "a/b/c.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := outputPath(tt.path); got != tt.want {
				t.Errorf("outputPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestIsTemplate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		want bool
	}{
		{name: ".tmpl is template", path: "file.txt.tmpl", want: true},
		{name: ".t is template", path: "config.yaml.t", want: true},
		{name: "uppercase .TMPL", path: "file.TMPL", want: true},
		{name: "uppercase .T", path: "file.T", want: true},
		{name: ".txt is not template", path: "file.txt", want: false},
		{name: "no extension", path: "Makefile", want: false},
		{name: ".tmplx is not template", path: "file.tmplx", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isTemplate(tt.path); got != tt.want {
				t.Errorf("isTemplate(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestWriteFile_MkdirError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a regular file where MkdirAll would need to create a directory.
	blockingFile := filepath.Join(tmpDir, "blocked")

	err := os.WriteFile(blockingFile, []byte(""), 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	// Try to write to a path where "blocked" is a file, not a directory.
	err = writeFile(filepath.Join(blockingFile, "child.txt"), "content")
	if err == nil {
		t.Fatal("writeFile() expected error when MkdirAll fails, got nil")
	}

	if !strings.Contains(err.Error(), "creating directories") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "creating directories")
	}
}

func TestRemoveIfExists_NonExistentFile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Removing a non-existent file must not return an error.
	err := removeIfExists(filepath.Join(tmpDir, "does-not-exist.txt"))
	if err != nil {
		t.Errorf("removeIfExists() unexpected error for non-existent file: %v", err)
	}
}
