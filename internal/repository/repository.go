// Package repository provides implementations for accessing upstream template
// repositories, either from a local filesystem path or a remote Git URL.
package repository

import "io/fs"

// Repository represents a source of template files that can be walked.
type Repository interface {
	// Open returns a read-only filesystem rooted at the template repository.
	Open() (fs.FS, error)
}
