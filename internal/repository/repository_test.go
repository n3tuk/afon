package repository_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"

	"github.com/n3tuk/afon/internal/repository"

	gogit "github.com/go-git/go-git/v5"
)

func TestNewLocal(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr string
	}{
		{
			name: "valid directory",
			path: tmpDir,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: "local repository path must not be empty",
		},
		{
			name:    "non-existent path",
			path:    filepath.Join(tmpDir, "no-such-dir"),
			wantErr: "local repository path does not exist",
		},
		{
			name:    "file not directory",
			path:    createTempFile(t, tmpDir, "notadir.txt"),
			wantErr: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := repository.NewLocal(tt.path)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("NewLocal(%q) expected error containing %q, got nil", tt.path, tt.wantErr)
				}

				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("NewLocal(%q) error = %q, want to contain %q", tt.path, err.Error(), tt.wantErr)
				}

				return
			}

			if err != nil {
				t.Fatalf("NewLocal(%q) unexpected error: %v", tt.path, err)
			}

			if got == nil {
				t.Fatal("NewLocal() returned nil repository")
			}
		})
	}
}

func TestLocalRepository_Open(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	err := os.WriteFile(filepath.Join(tmpDir, "hello.txt"), []byte("hello"), 0o600)
	if err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	repo, err := repository.NewLocal(tmpDir)
	if err != nil {
		t.Fatalf("NewLocal: %v", err)
	}

	fsys, err := repo.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	data, err := fs.ReadFile(fsys, "hello.txt")
	if err != nil {
		t.Fatalf("ReadFile(hello.txt): %v", err)
	}

	if string(data) != "hello" {
		t.Errorf("ReadFile(hello.txt) = %q, want %q", string(data), "hello")
	}
}

func TestNewRemote_EmptyURL(t *testing.T) {
	t.Parallel()

	_, err := repository.NewRemote("", "", "")
	if err == nil {
		t.Fatal("NewRemote(\"\") expected error, got nil")
	}

	if !strings.Contains(err.Error(), "remote repository URL must not be empty") {
		t.Fatalf("error %q does not contain expected message", err.Error())
	}
}

// initLocalGitRepo creates a minimal git repository in dir with a single
// committed file, returning the file:// URL for use with RemoteRepository.
func initLocalGitRepo(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()

	repo, err := gogit.PlainInit(dir, false)
	if err != nil {
		t.Fatalf("git init: %v", err)
	}

	wt, err := repo.Worktree()
	if err != nil {
		t.Fatalf("worktree: %v", err)
	}

	for name, content := range files {
		p := filepath.Join(dir, filepath.FromSlash(name))

		err = os.MkdirAll(filepath.Dir(p), 0o750)
		if err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		err = os.WriteFile(p, []byte(content), 0o600)
		if err != nil {
			t.Fatalf("WriteFile %s: %v", name, err)
		}

		_, err = wt.Add(name)
		if err != nil {
			t.Fatalf("git add %s: %v", name, err)
		}
	}

	sig := &object.Signature{Name: "Test", Email: "test@example.com", When: time.Now()}

	_, err = wt.Commit("initial commit", &gogit.CommitOptions{Author: sig})
	if err != nil {
		t.Fatalf("git commit: %v", err)
	}

	return "file://" + dir
}

// TestRemoteRepository_Open_LocalURL verifies that Open clones a local
// file:// repository and returns an accessible filesystem.
func TestRemoteRepository_Open_LocalURL(t *testing.T) {
	t.Parallel()

	url := initLocalGitRepo(t, map[string]string{
		"hello.txt": "hello from remote",
	})

	repo, err := repository.NewRemote(url, "", "")
	if err != nil {
		t.Fatalf("NewRemote: %v", err)
	}

	fsys, err := repo.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	data, err := fs.ReadFile(fsys, "hello.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	if string(data) != "hello from remote" {
		t.Errorf("hello.txt = %q, want %q", string(data), "hello from remote")
	}
}

// TestRemoteRepository_Open_InvalidURL verifies that Open returns an error for
// an unreachable URL.
func TestRemoteRepository_Open_InvalidURL(t *testing.T) {
	t.Parallel()

	repo, err := repository.NewRemote("https://invalid.example.test/no-such-repo.git", "", "")
	if err != nil {
		t.Fatalf("NewRemote: %v", err)
	}

	_, err = repo.Open()
	if err == nil {
		t.Fatal("Open() expected error for unreachable URL, got nil")
	}

	if !strings.Contains(err.Error(), "cloning remote repository") {
		t.Errorf("error = %q, want to contain %q", err.Error(), "cloning remote repository")
	}
}

func TestNewRemote_ValidURL(t *testing.T) {
	t.Parallel()

	repo, err := repository.NewRemote("https://github.com/n3tuk/afon", "", "")
	if err != nil {
		t.Fatalf("NewRemote() unexpected error: %v", err)
	}

	if repo == nil {
		t.Fatal("NewRemote() returned nil")
	}
}

func createTempFile(t *testing.T, dir, name string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	err := os.WriteFile(path, []byte(""), 0o600)
	if err != nil {
		t.Fatalf("createTempFile: %v", err)
	}

	return path
}
