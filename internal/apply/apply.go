// Package apply implements the core orchestration logic for afon: walking a
// template repository, rendering .tmpl/.t files with the template engine, and
// writing or removing output files in the target directory.
package apply

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/n3tuk/afon/internal/engine"
	"github.com/n3tuk/afon/internal/repository"
)

// Applier orchestrates applying a template repository to an output directory.
type Applier struct {
	// Output is the target directory where rendered files are written (and
	// optional empty files are removed). Defaults to the current directory.
	Output string

	engine *engine.Engine
}

const (
	// tmplExt is the primary template file extension.
	tmplExt = ".tmpl"

	// tExt is the alternative template file extension.
	tExt = ".t"

	// afonConfig is the upstream configuration file that must never be copied
	// or rendered into the downstream repository.
	afonConfig = ".afon.yaml"

	// gitDir is the git metadata directory that must always be skipped to
	// avoid overwriting the downstream repository's git history.
	gitDir = ".git"
)

// Sentinel errors returned by New.
var (
	ErrOutputDirEmpty    = errors.New("output directory must not be empty")
	ErrOutputDirNotExist = errors.New("output directory does not exist")
	ErrOutputDirNotDir   = errors.New("output path is not a directory")
)

// New creates an Applier that writes output to the given directory.
func New(outputDir string) (*Applier, error) {
	if outputDir == "" {
		return nil, ErrOutputDirEmpty
	}

	info, err := os.Stat(outputDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrOutputDirNotExist, outputDir)
	}

	if err != nil {
		return nil, fmt.Errorf("checking output directory %s: %w", outputDir, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrOutputDirNotDir, outputDir)
	}

	return &Applier{
		Output: outputDir,
		engine: engine.New(),
	}, nil
}

// Apply walks the template repository, processes each file, and writes the
// results to the output directory. Template files (.tmpl/.t) are rendered
// with the given variables; static files are copied verbatim. If a template
// renders to an empty/whitespace-only string, the output file is removed (if it
// exists) and not recreated.
//
// When path is non-empty, only files within that path are processed and the
// path prefix is stripped from all output paths.
//
//nolint:funlen // We're only just over the limit
func (a *Applier) Apply(repo repository.Repository, path string, vars map[string]any) error {
	fsys, err := repo.Open()
	if err != nil {
		return fmt.Errorf("opening template repository: %w", err)
	}

	root := "."
	if path != "" {
		root = path
	}

	slog.Debug("walking template repository", "path", root, "output", a.Output)

	err = fs.WalkDir(fsys, root, func(fsPath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			// Skip the .git directory to avoid overwriting the downstream
			// repository's git history.
			if d.Name() == gitDir {
				slog.Debug("skipping git directory", "path", fsPath)

				return fs.SkipDir
			}

			return nil
		}

		// Skip the upstream .afon.yaml to prevent it from being copied or
		// rendered into the downstream repository.
		if d.Name() == afonConfig {
			slog.Debug("skipping afon configuration file", "path", fsPath)

			return nil
		}

		// Compute the output-relative path by stripping the template path
		// prefix so that its directory structure is not replicated in the output.
		relPath := fsPath
		if path != "" {
			if stripped, ok := strings.CutPrefix(fsPath, path+"/"); ok {
				relPath = stripped
			}
		}

		var errP error

		if isTemplate(relPath) {
			slog.Info("processing template", "path", fsPath, "output", outputPath(relPath))
			errP = a.processTemplate(fsys, fsPath, relPath, vars)
		} else {
			slog.Info("processing file", "path", fsPath, "output", outputPath(relPath))
			errP = a.processFile(fsys, fsPath, relPath, vars)
		}

		if errP != nil {
			slog.Error("error processing", "path", fsPath, "error", errP)
		}

		return errP
	})
	if err != nil {
		return fmt.Errorf("walking template repository: %w", err)
	}

	return nil
}

// processFile handles a single file from the template repository. Template
// files are rendered; static files are copied.
func (a *Applier) processFile(fsys fs.FS, fsPath, relPath string, vars map[string]any) error {
	if isTemplate(relPath) {
		return a.processTemplate(fsys, fsPath, relPath, vars)
	}

	return a.copyFile(fsys, fsPath, relPath)
}

// isTemplate reports whether the given path is a template file.
func isTemplate(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))

	return ext == tmplExt || ext == tExt
}

// outputPath returns the output path for a given source path, stripping any
// .tmpl/.t extension.
func outputPath(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	if ext == tmplExt || ext == tExt {
		return path[:len(path)-len(ext)]
	}

	return path
}

// processTemplate renders a template file and writes the result to the output
// directory. If the rendered output is empty, the output file is removed (if
// it exists) and not recreated.
func (a *Applier) processTemplate(fsys fs.FS, fsPath, relPath string, vars map[string]any) error {
	content, err := readFile(fsys, fsPath)
	if err != nil {
		return err
	}

	rendered, err := a.engine.Render(fsPath, content, vars)
	if err != nil {
		return fmt.Errorf("rendering template %s: %w", fsPath, err)
	}

	dest := filepath.Join(a.Output, outputPath(relPath))

	if engine.IsEmpty(rendered) {
		return removeIfExists(dest)
	}

	return writeFile(dest, rendered)
}

// copyFile copies a static (non-template) file verbatim to the output directory.
func (a *Applier) copyFile(fsys fs.FS, fsPath, relPath string) error {
	content, err := readFile(fsys, fsPath)
	if err != nil {
		return err
	}

	dest := filepath.Join(a.Output, relPath)

	return writeFile(dest, content)
}

// readFile reads the complete content of path from fsys, returning it as a
// string.
func readFile(fsys fs.FS, path string) (string, error) {
	f, err := fsys.Open(path)
	if err != nil {
		return "", fmt.Errorf("opening %s: %w", path, err)
	}

	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}

	return string(data), nil
}

// writeFile writes content to path, creating any necessary parent directories.
func writeFile(path, content string) error {
	err := os.MkdirAll(filepath.Dir(path), 0o750)
	if err != nil {
		return fmt.Errorf("creating directories for %s: %w", path, err)
	}

	err = os.WriteFile(path, []byte(content), 0o600)
	if err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// removeIfExists removes the file at path if it exists; it is not an error if
// the file does not exist.
func removeIfExists(path string) error {
	err := os.Remove(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("removing %s: %w", path, err)
	}

	return nil
}
