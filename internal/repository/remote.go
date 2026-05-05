package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/go-git/go-billy/v5/helper/iofs"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"

	githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)

// RemoteRepository clones a template from a remote Git URL into memory using
// go-git. An optional Ref (branch name, tag, or full reference string) may be
// specified; if empty, the default branch (HEAD) is used.
type RemoteRepository struct {
	// URL is the remote Git repository URL (HTTPS or SSH).
	URL string

	// Reference is the branch name, tag, or full reference to check out. If
	// empty, the remote HEAD is used.
	Reference string

	// Token is a personal access token for authenticating against private
	// repositories. If empty, the GITHUB_TOKEN environment variable is used
	// as a fallback.
	Token string
}

// ErrRemoteURLEmpty is returned by NewRemote when the URL is empty.
var ErrRemoteURLEmpty = errors.New("remote repository URL must not be empty")

// NewRemote creates a RemoteRepository for the given URL, reference, and
// optional authentication token.
func NewRemote(url, reference, token string) (*RemoteRepository, error) {
	if url == "" {
		return nil, ErrRemoteURLEmpty
	}

	return &RemoteRepository{
		URL:       url,
		Reference: reference,
		Token:     token,
	}, nil
}

// Open clones the remote repository into memory and returns a read-only
// filesystem rooted at the repository root.
func (r *RemoteRepository) Open() (fs.FS, error) {
	worktree := memfs.New()

	opts := &git.CloneOptions{
		URL:          r.URL,
		SingleBranch: true,
		Depth:        1,
	}

	if r.Reference != "" {
		opts.ReferenceName = resolveRef(r.Reference)
	}

	// Authenticate using the provided token, falling back to GITHUB_TOKEN.
	token := r.Token
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}

	if token != "" {
		opts.Auth = &githttp.BasicAuth{
			Username: "x-token-auth",
			Password: token,
		}
	}

	slog.Info("cloning remote repository",
		"source", r.URL,
		"reference", opts.ReferenceName,
	)

	_, err := git.Clone(memory.NewStorage(), worktree, opts)
	if err != nil {
		return nil, fmt.Errorf("cloning remote repository %s: %w", r.URL, err)
	}

	return iofs.New(worktree), nil
}

// resolveRef converts a short reference name (e.g. "main", "v1.0.0") into a
// full plumbing.ReferenceName. Branch names are prefixed with refs/heads/, tags
// with refs/tags/; values already beginning with "refs/" are used as-is.
func resolveRef(reference string) plumbing.ReferenceName {
	switch {
	case len(reference) >= 5 && reference[:5] == "refs/":
		return plumbing.ReferenceName(reference)
	case isTagLike(reference):
		return plumbing.NewTagReferenceName(reference)
	default:
		return plumbing.NewBranchReferenceName(reference)
	}
}

// isTagLike returns true when reference looks like a semver tag (starts with
// 'v' and contains a dot), allowing common patterns like "v1.0.0".
func isTagLike(reference string) bool {
	if len(reference) < 2 || reference[0] != 'v' {
		return false
	}

	for _, c := range reference[1:] {
		if c == '.' {
			return true
		}
	}

	return false
}
