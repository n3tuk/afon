package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// LocalRepository reads template files from a local filesystem path.
type LocalRepository struct {
	// Path is the absolute or relative path to the template repository root.
	Path string
}

// Sentinel errors returned by NewLocal.
var (
	ErrLocalPathEmpty    = errors.New("local repository path must not be empty")
	ErrLocalPathNotExist = errors.New("local repository path does not exist")
	ErrLocalPathNotDir   = errors.New("local repository path is not a directory")
)

// NewLocal creates a LocalRepository for the given path, returning an error if
// the path does not exist or is not a directory.
func NewLocal(path string) (*LocalRepository, error) {
	if path == "" {
		return nil, ErrLocalPathEmpty
	}

	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrLocalPathNotExist, path)
	}

	if err != nil {
		return nil, fmt.Errorf("checking local repository path %s: %w", path, err)
	}

	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s", ErrLocalPathNotDir, path)
	}

	return &LocalRepository{Path: path}, nil
}

// Open returns a read-only filesystem rooted at the local repository path.
func (r *LocalRepository) Open() (fs.FS, error) {
	return os.DirFS(r.Path), nil
}
