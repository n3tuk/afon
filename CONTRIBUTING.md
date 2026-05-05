# Contributing to afon

Thank you for your interest in contributing to `afon`. This document covers
everything you need to get started: prerequisites, development commands, code
conventions, and the pull request process.

## Prerequisites

| Tool                                                      | Purpose                          | Installation                            |
| --------------------------------------------------------- | -------------------------------- | --------------------------------------- |
| [Go 1.26+](https://go.dev/dl/)                            | Build and test                   | `brew install go` or official installer |
| [Task](https://taskfile.dev/#/installation)               | Task runner                      | `brew install go-task`                  |
| [golangci-lint](https://golangci-lint.run/usage/install/) | Linting and formatting           | `brew install golangci-lint`            |
| [GoReleaser](https://goreleaser.com/install/)             | Multi-platform builds (optional) | `brew install goreleaser`               |

## Setting up

```shell
git clone https://github.com/n3tuk/afon.git
cd afon
go mod download
```

## Development commands

All development tasks are automated with [Task](https://taskfile.dev/). Run
`task -l` for the full list of available tasks.

| Command          | Description                                            |
| ---------------- | ------------------------------------------------------ |
| `task develop`   | Run linting, formatting, and tests in one step         |
| `task go:fmt`    | Format all `.go` files in-place                        |
| `task go:lint`   | Run golangci-lint against the full codebase            |
| `task go:test`   | Run all tests with coverage reporting                  |
| `task go:build`  | Build binary for the current platform to `bin/afon`    |
| `task go:schema` | Re-generate `schemas/afon.json` from the config struct |
| `task clean`     | Remove build artefacts                                 |

Run `task develop` before opening a pull request to ensure linting, formatting,
and all tests pass.

## Project structure

```plain
afon/
├── cmd/afon/main.go          — CLI entry point (Cobra commands, logging setup,
│                               URL detection, flag binding)
├── internal/
│   ├── apply/apply.go        — Walk, render, write/delete orchestration
│   ├── config/config.go      — YAML configuration loader (viper-backed)
│   ├── engine/engine.go      — text/template + sprig wrapper
│   └── repository/
│       ├── repository.go     — Repository interface (Open → fs.FS)
│       ├── local.go          — Local filesystem repository
│       └── remote.go         — Remote Git repository (go-git in-memory clone)
├── tools/schema/main.go      — JSON Schema generator (//go:build ignore)
├── schemas/afon.json         — Generated JSON Schema for .afon.yaml
└── examples/workflows/       — Example downstream GitHub Workflow
```

## Testing

Tests live alongside their packages in `*_test.go` files. Run the full suite
with coverage:

```bash
task go:test
```

The target coverage is **95%+** across all internal packages.

Conventions for tests:

- Use [`fstest.MapFS`][mapfs] for in-memory filesystem test doubles in
  `internal/apply` rather than writing to the real filesystem.
- Remote repository tests create temporary `file://` URL repositories using
  go-git directly. See `internal/repository/repository_test.go` for the
  pattern.
- Test data (fixture files, template sources) belongs in `**/testdata/`
  directories.

[mapfs]: https://pkg.go.dev/testing/fstest#MapFS

## Code style

The project uses strict Go formatting and linting enforced by golangci-lint.
Run `task go:fmt` before committing; it applies `gofumpt`, `goimports`, and
`gci` in the correct order.

### Import ordering

Imports must be grouped and ordered in exactly this sequence:

```go
import (
    // 1. Standard library
    "fmt"
    "os"

    // 2. External dependencies
    "github.com/spf13/cobra"

    // 3. Internal packages under github.com/n3tuk
    "github.com/n3tuk/afon/internal/config"

    // 4. Aliased imports (e.g. go-git transport)
    githttp "github.com/go-git/go-git/v5/plumbing/transport/http"
)
```

### Key linting rules

| Rule               | Detail                                                                             |
| ------------------ | ---------------------------------------------------------------------------------- |
| `noinlineerr`      | Split `if err := f(); err != nil` — assignment and check must be on separate lines |
| `forbidigo`        | No `fmt.Print*` outside tests — use `fmt.Fprintf(os.Stderr, ...)` or `slog`        |
| `gochecknoglobals` | Package-level variables require `//nolint:gochecknoglobals // reason`              |
| `nolintlint`       | Every `//nolint:` must name the linter and include an explanation comment          |
| `wsl_v5`           | Blank line required before `if`/`for` blocks with multi-line bodies                |
| `decorder`         | Package-level declarations must follow `type → const → var → func` order           |

### Spelling

The misspell linter is configured with the **UK locale**. Use UK English
spellings throughout comments and documentation: `artefact`, `behaviour`,
`initialise`, `licence` (noun), `organise`, etc.

### Adding new dependencies

Before importing a new package in `internal/**` or `cmd/**`, check the
`depguard` rules in `.golangci.yaml`. The allowed sets are deliberately
restrictive:

- `internal/**`: standard library, `go-cmp`, `viper`, `sprig/v3`, `go-git/v5`,
  `go-billy/v5`, and `github.com/n3tuk/afon`.
- `cmd/**`: standard library, `cobra`, `viper`, and `github.com/n3tuk/afon`.

Raise a discussion in your pull request if you believe a new dependency is
warranted.

## Updating the JSON Schema

When you add or rename fields in `internal/config/config.go`, regenerate the
schema:

```bash
task go:schema
```

Also update the `metadata` map in `tools/schema/main.go` to add a description
for each new field. The generator uses this map to populate the `description`
property in the emitted schema.

## Commit conventions

Commits must follow [Conventional Commits][conv-commits]:

| Prefix      | When to use                                 |
| ----------- | ------------------------------------------- |
| `feat:`     | A new feature visible to users              |
| `fix:`      | A bug fix                                   |
| `docs:`     | Documentation changes only                  |
| `refactor:` | Code restructuring with no behaviour change |
| `test:`     | Adding or updating tests                    |
| `chore:`    | Maintenance, dependency updates, tooling    |

[conv-commits]: https://www.conventionalcommits.org/

## Pull requests

1. Fork the repository and create a branch from `main`.
2. Make your changes, ensuring `task develop` passes cleanly.
3. Commit with a conventional commit message.
4. Open a pull request — the PR title is used as the release note entry, so
   make it descriptive.
5. A maintainer will review and merge your changes.
