# Copilot Instructions for afon

## Build, Test, and Lint

### Quick Commands

- **Full development check**: `task develop` (runs linting, formatting, and tests)
- **Linting**: `task lint` or `task go:lint`
- **Formatting**: `task go:fmt` (writes changes to files)
- **Testing**: `task go:test`
- **Build binary**: `task go:build` (builds for current platform in `bin/afon`)
- **Clean**: `task clean` (removes build artifacts)

All commands use [Task](https://taskfile.dev/) for automation. Run `task -l` to see all available tasks.

## High-Level Architecture

**afon** is a GitHub Action and Go CLI tool that applies an upstream template
repository to a downstream repository. It processes `.tmpl`/`.t` files using
`text/template` + sprig, copies all other files verbatim, and removes output
files whose templates render to an empty string.

- **Entry point**: GitHub Action via `action.yaml` — runs in a Docker container
- **Implementation**: Go CLI tool at `./cmd/afon/main.go`
- **Distribution**: Multi-platform binary (Linux/Darwin/Windows, amd64/arm64) built by GoReleaser
- **Container**: Scratch-based Docker image published to GCR (`gcr.io/n3tuk/afon`) and GHCR
- **Build info**: Version, commit, branch, and build date are injected via ldflags during compilation

## Package Structure

```
github.com/n3tuk/afon
├── cmd/afon/main.go            — Cobra root command, apply + version subcommands
├── internal/
│   ├── config/config.go        — Config struct, viper-based YAML loader
│   ├── repository/
│   │   ├── repository.go       — Repository interface: Open() (fs.FS, error)
│   │   ├── local.go            — LocalRepository (os.DirFS)
│   │   └── remote.go           — RemoteRepository (go-git in-memory clone)
│   ├── engine/engine.go        — text/template + sprig: Render(), IsEmpty()
│   └── apply/apply.go          — Orchestration: walk FS, render, write/delete
└── examples/workflows/afon.yaml — Example downstream GitHub Workflow
```

## Configuration File Format

The tool reads `.afon.yaml` by default (override with `--config`):

```yaml
template:
    source: https://github.com/org/template-repo # local path or remote URL
    ref: main # branch, tag, or full ref (remote only)

variables:
    project_name: my-project
    language: go
    go_version: '1.24'
```

## CLI Interface

```
afon apply [flags]
  --config    Path to config file (default: .afon.yaml)
  --template  Path or URL to template repository (overrides config)
  --ref       Branch, tag, or ref for remote templates (overrides config)

afon version
```

## Template Processing Rules

1. Files without `.tmpl`/`.t` extension → copied verbatim to output
2. Files with `.tmpl`/`.t` extension → rendered via `text/template` + sprig
    - Output path = source path minus the `.tmpl`/`.t` suffix
    - Rendered output is empty (all whitespace) → **delete** output file if it exists, otherwise **skip**
    - Rendered output is non-empty → **write** file (create parent directories as needed)

## Key Conventions

### Import Management

Strict import restrictions are enforced via `depguard` in `.golangci.yaml`:

- **`internal/**`** (non-test): standard library + `go-cmp`, `viper`, `sprig/v3`, `go-git/v5`, `github.com/n3tuk/afon`
- **`cmd/**`** (non-test): standard library + `cobra`, `viper`, `github.com/n3tuk/afon`

Always check `.golangci.yaml` before adding new dependencies to internal packages.

### Code Formatting & Linting

The project uses aggressive Go formatting and linting:

- **Formatters**: gofumpt (strict), goimports (local prefix aware), gci (deterministic import order), gofmt
- **Linters**: golangci-lint with comprehensive checks (staticcheck, stylecheck, errorlint, musttag, etc.)
- **Standards**: Enforces UK English spelling (misspell checker)
- **Import order**: standard → default → `prefix(github.com/n3tuk)` → localmodule → alias → blank

Ensure `task go:fmt` passes locally before committing.

### Multi-Platform Builds

Builds are configured for:

- **OS**: Linux, Darwin (macOS), Windows
- **Architectures**: amd64 (v3), arm64, arm (v7 on linux only)
- **Apple Silicon exclusion**: Darwin amd64 builds are disabled (v1 only, not usable)

GoReleaser (`goreleaser` CLI or `task go:build`) handles all cross-compilation automatically.

### Directory Structure

- `cmd/afon/`: CLI entry point
- `internal/`: Internal packages (strict import restrictions apply)
- `examples/workflows/`: Example GitHub Workflow for downstream repositories
- `.tasks/`: Task automation includes (modular task definitions)
- `dist/`: Build output and goreleaser artifacts
- `bin/`: Local binary builds (gitignored)
- `coverage/`: Test coverage reports (gitignored)

### Testing & Coverage

- **Target**: 95%+ coverage across all internal packages
- Tests generate HTML coverage reports in `coverage.html`
- Use `-covermode=count` for accurate line-level coverage
- Test data belongs in `**/testdata/` directories
- Use `fstest.MapFS` for filesystem test doubles in `internal/apply`
- `remote.Open()` requires `file://` URL test repos (created via go-git in tests)

### Docker & Container Releases

- Scratch-based image (minimal, production-ready)
- Multi-platform (linux/amd64, linux/arm64)
- Tags: `latest` or `nightly`, plus `v{version}` and short commit SHA
- SBOM (Software Bill of Materials) generated for non-nightly releases
- Published to both GCR and GHCR registries
