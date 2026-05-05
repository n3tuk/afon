package apply_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/n3tuk/afon/internal/apply"
)

type (
	// mapRepository is a test double for repository.Repository backed by an
	// fstest.MapFS.
	mapRepository struct {
		fs fstest.MapFS
	}

	// partialErrFS wraps an fstest.MapFS and allows walking, but returns an
	// error when Open is called for a specific target file path. This lets
	// WalkDir discover the file but triggers a read error when the applier
	// tries to open it for processing.
	partialErrFS struct {
		inner    fstest.MapFS
		failFile string
		err      error
	}

	// partialErrRepository wraps a partialErrFS.
	partialErrRepository struct {
		fs *partialErrFS
	}

	// errRepository is a test double whose Open returns a controlled error.
	errRepository struct {
		err error
	}
)

const (
	optionalTmpl = "{{- if .enabled }}content{{- end }}"
	enabledKey   = "enabled"
	readmeMD     = "README.md"
	helloContent = "hello"
)

func (r *mapRepository) Open() (fs.FS, error) {
	return r.fs, nil
}

func newMapRepo(files map[string]string) *mapRepository {
	m := make(fstest.MapFS, len(files))

	for path, content := range files {
		m[path] = &fstest.MapFile{Data: []byte(content)}
	}

	return &mapRepository{fs: m}
}

func (f *partialErrFS) Open(name string) (fs.File, error) {
	if name == f.failFile {
		return nil, f.err
	}

	return f.inner.Open(name)
}

func (r *partialErrRepository) Open() (fs.FS, error) {
	return r.fs, nil
}

func (r *errRepository) Open() (fs.FS, error) {
	return nil, r.err
}

func readOutputFile(t *testing.T, outDir, rel string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(outDir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", rel, err)
	}

	return string(data)
}

// TestNew covers construction error cases.
func TestNew(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "file.txt")

	err := os.WriteFile(tmpFile, []byte(""), 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	tests := []struct {
		name    string
		dir     string
		wantErr string
	}{
		{
			name: "valid directory",
			dir:  tmpDir,
		},
		{
			name:    "empty string",
			dir:     "",
			wantErr: "output directory must not be empty",
		},
		{
			name:    "non-existent directory",
			dir:     filepath.Join(tmpDir, "no-such"),
			wantErr: "output directory does not exist",
		},
		{
			name:    "file not directory",
			dir:     tmpFile,
			wantErr: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := apply.New(tt.dir)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("New(%q) expected error containing %q, got nil", tt.dir, tt.wantErr)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("New(%q) error = %q, want %q", tt.dir, err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("New(%q) unexpected error: %v", tt.dir, err)
			}

			if got == nil {
				t.Fatal("New() returned nil applier")
			}
		})
	}
}

// TestApply_StaticFile verifies that static files are copied verbatim.
func TestApply_StaticFile(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		readmeMD: "# Hello\n",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readOutputFile(t, outDir, readmeMD)
	if got != "# Hello\n" {
		t.Errorf("README.md = %q, want %q", got, "# Hello\n")
	}
}

// TestApply_TemplateFile verifies that .tmpl files are rendered and written.
func TestApply_TemplateFile(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		"greeting.txt.tmpl": "Hello, {{ .name }}!\n",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	vars := map[string]any{"name": "World"}

	err = a.Apply(repo, "", vars)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readOutputFile(t, outDir, "greeting.txt")
	if got != "Hello, World!\n" {
		t.Errorf("greeting.txt = %q, want %q", got, "Hello, World!\n")
	}
}

// TestApply_TplExtension verifies that .t files are also treated as templates.
func TestApply_TplExtension(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		"config.yaml.t": "key: {{ .value }}\n",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", map[string]any{"value": "42"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readOutputFile(t, outDir, "config.yaml")
	if got != "key: 42\n" {
		t.Errorf("config.yaml = %q, want %q", got, "key: 42\n")
	}
}

// TestApply_EmptyTemplate_SkipsCreation verifies that an empty render does not
// create the output file.
func TestApply_EmptyTemplate_SkipsCreation(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		"optional.txt.tmpl": optionalTmpl,
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", map[string]any{enabledKey: false})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	dest := filepath.Join(outDir, "optional.txt")

	_, err = os.Stat(dest)
	if !os.IsNotExist(err) {
		t.Errorf("optional.txt should not have been created, but it exists")
	}
}

// TestApply_EmptyTemplate_DeletesExisting verifies that an empty render removes
// an already-existing output file.
func TestApply_EmptyTemplate_DeletesExisting(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	dest := filepath.Join(outDir, "optional.txt")

	err := os.WriteFile(dest, []byte("old content"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := newMapRepo(map[string]string{
		"optional.txt.tmpl": optionalTmpl,
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", map[string]any{enabledKey: false})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	_, err = os.Stat(dest)
	if !os.IsNotExist(err) {
		t.Errorf("optional.txt should have been removed, but it still exists")
	}
}

// TestApply_NestedDirectory verifies that nested paths are preserved.
func TestApply_NestedDirectory(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		"a/b/c.txt": "nested content",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := readOutputFile(t, outDir, "a/b/c.txt")
	if got != "nested content" {
		t.Errorf("a/b/c.txt = %q, want %q", got, "nested content")
	}
}

// TestApply_MultipleFiles verifies that a mix of template and static files are
// all handled in a single Apply call.
func TestApply_MultipleFiles(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		"static.txt":       "unchanged",
		"dynamic.txt.tmpl": "Hello, {{ .who }}!",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", map[string]any{"who": "Afon"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	if got := readOutputFile(t, outDir, "static.txt"); got != "unchanged" {
		t.Errorf("static.txt = %q, want %q", got, "unchanged")
	}

	if got := readOutputFile(t, outDir, "dynamic.txt"); got != "Hello, Afon!" {
		t.Errorf("dynamic.txt = %q, want %q", got, "Hello, Afon!")
	}
}

// TestApply_InvalidTemplate verifies that a template parse error is surfaced.
func TestApply_InvalidTemplate(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		"bad.txt.tmpl": "{{ .unclosed",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err == nil {
		t.Fatal("Apply() expected an error for an invalid template, got nil")
	}
}

// TestApply_ReadError_StaticFile verifies that Apply propagates a read error
// for a static file when the underlying FS fails to open it.
func TestApply_ReadError_StaticFile(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	inner := fstest.MapFS{
		"readme.txt": &fstest.MapFile{Data: []byte("hello")},
	}

	repo := &partialErrRepository{
		fs: &partialErrFS{
			inner:    inner,
			failFile: "readme.txt",
			err:      errors.New("disk read error"),
		},
	}

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err == nil {
		t.Fatal("Apply() expected an error from broken FS, got nil")
	}
}

// TestApply_ReadError_TemplateFile verifies that Apply propagates a read error
// for a template file when the underlying FS fails to open it.
func TestApply_ReadError_TemplateFile(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	inner := fstest.MapFS{
		"config.yaml.tmpl": &fstest.MapFile{Data: []byte("key: {{ .value }}")},
	}

	repo := &partialErrRepository{
		fs: &partialErrFS{
			inner:    inner,
			failFile: "config.yaml.tmpl",
			err:      errors.New("disk read error"),
		},
	}

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err == nil {
		t.Fatal("Apply() expected an error from broken FS for template file, got nil")
	}
}

// TestApply_WriteError_ReadOnly verifies that Apply propagates a write error
// when the output file is read-only and cannot be overwritten.
func TestApply_WriteError_ReadOnly(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	// Create a read-only output file that the applier will try to overwrite.
	dest := filepath.Join(outDir, "output.txt")

	err := os.WriteFile(dest, []byte("old"), 0o400)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := newMapRepo(map[string]string{
		"output.txt": "new content",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err == nil {
		t.Fatal("Apply() expected an error writing to a read-only file, got nil")
	}
}

// TestApply_RemoveError_Directory verifies removeIfExists error propagation
// when trying to remove a non-empty directory (which produces a
// non-ErrNotExist error).
func TestApply_RemoveError_Directory(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	// Create a directory (with a child) at the expected output path so that
	// os.Remove fails with "directory not empty" (not ErrNotExist).
	subDir := filepath.Join(outDir, "optional")

	err := os.MkdirAll(filepath.Join(subDir, "child"), 0o750)
	if err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Template renders empty → should try to remove "optional". But "optional"
	// is a non-empty directory, so Remove returns an error.
	repo := newMapRepo(map[string]string{
		"optional.tmpl": optionalTmpl,
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", map[string]any{enabledKey: false})
	if err == nil {
		t.Fatal("Apply() expected an error when Remove fails on a directory, got nil")
	}
}

// TestApply_RepositoryOpenError verifies that Apply propagates an Open error.
func TestApply_RepositoryOpenError(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	repo := &errRepository{err: errors.New("failed to open")}

	err = a.Apply(repo, "", nil)
	if err == nil {
		t.Fatal("Apply() expected an error when repo.Open fails, got nil")
	}

	if !strings.Contains(err.Error(), "opening template repository") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "opening template repository")
	}
}

// TestApply_WriteError verifies that Apply propagates a write error when the
// output path is blocked by an existing file acting as a directory.
func TestApply_WriteError(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	// Create a file that will block mkdir for a/b/.
	err := os.WriteFile(filepath.Join(outDir, "a"), []byte(""), 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo := newMapRepo(map[string]string{
		"a/b/file.txt": "content",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err == nil {
		t.Fatal("Apply() expected a write error, got nil")
	}
}

// TestApply_SkipsGitDirectory verifies that .git directories are never walked
// or written into the output directory.
func TestApply_SkipsGitDirectory(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		".git/config": "[core]\n\trepositoryformatversion = 0",
		".git/HEAD":   "ref: refs/heads/main",
		readmeMD:      helloContent,
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// README.md must be written.
	got := readOutputFile(t, outDir, readmeMD)
	if got != helloContent {
		t.Errorf("README.md = %q, want %q", got, helloContent)
	}

	// Nothing inside .git must be written.
	_, errStat := os.Stat(filepath.Join(outDir, ".git"))
	if !os.IsNotExist(errStat) {
		t.Error(".git directory should not have been created in output")
	}
}

// TestApply_SkipsAfonYaml verifies that the upstream .afon.yaml file is never
// copied or rendered into the output directory.
func TestApply_SkipsAfonYaml(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		".afon.yaml": "template:\n  source: upstream\n",
		readmeMD:     helloContent,
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "", nil)
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// README.md must be written.
	got := readOutputFile(t, outDir, readmeMD)
	if got != helloContent {
		t.Errorf("README.md = %q, want %q", got, helloContent)
	}

	// .afon.yaml must not appear in the output.
	_, errStat := os.Stat(filepath.Join(outDir, ".afon.yaml"))
	if !os.IsNotExist(errStat) {
		t.Error(".afon.yaml should not have been created in output")
	}
}

// TestApply_Subdir verifies that when a template path is set, only files within
// that path are processed and the path prefix is stripped from output paths.
func TestApply_Subdir(t *testing.T) {
	t.Parallel()

	outDir := t.TempDir()

	repo := newMapRepo(map[string]string{
		"template/README.md":          "# from subdir",
		"template/config.yaml.tmpl":   "key: {{ .value }}",
		"should-not-appear/other.txt": "ignored",
	})

	a, err := apply.New(outDir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	err = a.Apply(repo, "template", map[string]any{"value": "ok"})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// Files from the subdir must appear at the root of output (no "template/" prefix).
	if got := readOutputFile(t, outDir, "README.md"); got != "# from subdir" {
		t.Errorf("README.md = %q, want %q", got, "# from subdir")
	}

	if got := readOutputFile(t, outDir, "config.yaml"); got != "key: ok" {
		t.Errorf("config.yaml = %q, want %q", got, "key: ok")
	}

	// Files outside the subdir must not appear.
	_, errStat := os.Stat(filepath.Join(outDir, "should-not-appear"))
	if !os.IsNotExist(errStat) {
		t.Error("should-not-appear/ must not be created in output")
	}
}
