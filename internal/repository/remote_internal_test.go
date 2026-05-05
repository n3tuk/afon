package repository

import (
	"testing"

	"github.com/go-git/go-git/v5/plumbing"
)

func TestResolveRef_Branches(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		reference string
		want      plumbing.ReferenceName
	}{
		{
			name:      "plain branch name",
			reference: "main",
			want:      plumbing.NewBranchReferenceName("main"),
		},
		{
			name:      "feature branch",
			reference: "feature/my-feature",
			want:      plumbing.NewBranchReferenceName("feature/my-feature"),
		},
		{
			name:      "semver tag",
			reference: "v1.2.3",
			want:      plumbing.NewTagReferenceName("v1.2.3"),
		},
		{
			name:      "short tag no dot",
			reference: "v1",
			want:      plumbing.NewBranchReferenceName("v1"),
		},
		{
			name:      "explicit refs/heads",
			reference: "refs/heads/main",
			want:      plumbing.ReferenceName("refs/heads/main"),
		},
		{
			name:      "explicit refs/tags",
			reference: "refs/tags/v1.0.0",
			want:      plumbing.ReferenceName("refs/tags/v1.0.0"),
		},
		{
			name:      "explicit refs/pull",
			reference: "refs/pull/42/head",
			want:      plumbing.ReferenceName("refs/pull/42/head"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := resolveRef(tt.reference)

			if got != tt.want {
				t.Errorf("resolveRef(%q) = %q, want %q", tt.reference, got, tt.want)
			}
		})
	}
}

func TestIsTagLike(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		reference string
		want      bool
	}{
		{name: "semver tag", reference: "v1.0.0", want: true},
		{name: "minor semver", reference: "v1.2", want: true},
		{name: "no dot", reference: "v1", want: false},
		{name: "empty string", reference: "", want: false},
		{name: "just v", reference: "v", want: false},
		{name: "branch main", reference: "main", want: false},
		{name: "branch with dot", reference: "release.1.0", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isTagLike(tt.reference); got != tt.want {
				t.Errorf("isTagLike(%q) = %v, want %v", tt.reference, got, tt.want)
			}
		})
	}
}
