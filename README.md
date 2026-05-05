# afon — Upstream Template Repository Engine

[![CodeQL](https://github.com/n3tuk/afon/actions/workflows/codeql.yaml/badge.svg)](https://github.com/n3tuk/afon/actions/workflows/codeql.yaml)
[![Code Coverage](https://codecov.io/gh/n3tuk/afon/branch/main/graph/badge.svg?token=GSu1DCSng1)](https://codecov.io/gh/n3tuk/afon)
[![Draft Release](https://github.com/n3tuk/afon/actions/workflows/draft-release.yaml/badge.svg?branch=main)](https://github.com/n3tuk/afon/actions/workflows/draft-release.yaml)

`afon` is a Go CLI tool and GitHub Action for continuously applying an upstream
template repository to one or more downstream repositories. Similar to
[Cookiecutter][cookiecutter] and [Yeoman][yeoman] for initial scaffolding,
`afon` goes further: it is designed specifically to be lightweight and to be run
repeatedly on a schedule so that downstream repositories stay aligned with an
evolving template over their entire lifetime - not just at creation time.

[cookiecutter]: https://www.cookiecutter.io/
[yeoman]: https://yeoman.io/

## How it works

There are three components:

1. **This repository** containing the `afon` GitHub Action and CLI tool which
   will do the processing;
2. **An upstream template repository** (a Git repository) containing `.tmpl`/`.t`
   template files, and other static files, that define the shared structure
   (GitHub Workflows, linter configurations, tooling settings, etc.) across one
   or more repositories; and
3. **One or more downstream repositories** each with a `.afon.yaml`
   configuration file and a GitHub Workflow that runs this `afon` GitHub Action
   on a schedule, or on demand, to sync the upstream templates with this
   downstream repository.

When `afon apply` runs, it:

1. Opens the upstream template repository;
2. Walks every file within the configured sub-directory of the upstream
   repository repository;
3. Renders `.tmpl`/`.t` files through Go's [`text/template`][text-template]
   engine with the [sprig][sprig] function library, using the variables set in
   your `.afon.yaml` file;
4. Writes rendered templates to files, and copies static files, into the output
   (relative to the root of the downstream repository); and
5. Removes output files whose templates render to an empty string.

[text-template]: https://pkg.go.dev/text/template
[sprig]: https://masterminds.github.io/sprig/

## Template processing rules

| Source file                                                  | Behaviour                                                            |
| ------------------------------------------------------------ | -------------------------------------------------------------------- |
| Any file without `.tmpl` or `.t` extension                   | Copied verbatim to the output path                                   |
| `*.tmpl` / `*.t` — renders to non-empty output               | Rendered and written (output path = source path minus the extension) |
| `*.tmpl` / `*.t` — renders to empty output (whitespace only) | Output file deleted if it exists; skipped otherwise                  |
| `.git/` directory                                            | Always skipped (never propagated to the downstream repository)       |
| `.afon.yaml`                                                 | Always skipped (never propagated to the downstream repository)       |

## Configuration

Downstream repositories need a `.afon.yaml` file at their root:

```yaml
---
# yamllint disable-line rule:line-length
# yaml-language-server: $schema=https://raw.githubusercontent.com/n3tuk/afon/main/schemas/afon.json

template:
    source: https://github.com/your-org/template-repo
    path: templates
    # branch, tag, or commit SHA (optional)
    reference: main

variables:
    project_name: my-service
    language: go
    go_version: '1.26'
```

### Configuration reference

| Key               | Required | Description                                                                                                                                                 |
| ----------------- | -------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `template.source` | ✓        | Local filesystem path or remote Git URL to the upstream template repository                                                                                 |
| `template.ref`    |          | Branch, tag, or commit SHA to check out for remote sources                                                                                                  |
| `template.path`   |          | Subdirectory within the template repository to use as the root. Only files within this path are processed and the prefix is stripped from all output paths. |
| `template.token`  |          | Personal access token for private remote repositories. Falls back to the `GITHUB_TOKEN` environment variable.                                               |
| `variables`       |          | Free-form YAML map. All values are available in templates as `{{ .key }}`.                                                                                  |

### JSON Schema

A JSON Schema for `.afon.yaml` is published alongside each release:

```plain
https://raw.githubusercontent.com/n3tuk/afon/main/schemas/afon.json
```

Add a `yaml-language-server` comment at the top of `.afon.yaml` for in-editor
validation in [Neovim][neovim] (with the [yaml-language-server][yamlls]) or
[Visual Studio Code][vscode] (with the [YAML extension][yaml-ext]) and other
schema-aware editors:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/n3tuk/afon/main/schemas/afon.json
```

[neovim]: https://neovim.io/
[yamlls]: https://github.com/redhat-developer/yaml-language-server
[vscode]: https://code.visualstudio.com/
[yaml-ext]: https://marketplace.visualstudio.com/items?itemName=redhat.vscode-yaml

## Writing templates

Template files use Go's [`text/template`][text-template] syntax, augmented by
the [sprig][sprig] function library. All values defined under `variables:` in
`.afon.yaml` are available directly by name.

### Basic example

A file named `go.mod.tmpl` in the upstream template repository:

```go
module github.com/your-org/{{ .project_name }}

go {{ .go_version }}
```

With the example `.afon.yaml` configuration above, this template renders to
`go.mod` in the downstream repository to:

```go
module github.com/your-org/my-service

go 1.26
```

### Useful sprig functions

| Function          | Example                                            |
| ----------------- | -------------------------------------------------- |
| `default`         | `{{ default "main" .branch }}`                     |
| `lower` / `upper` | `{{ lower .project_name }}`                        |
| `title`           | `{{ title .project_name }}`                        |
| `replace`         | `{{ replace "-" "_" .project_name }}`              |
| `trimSpace`       | `{{ trimSpace .value }}`                           |
| `ternary`         | `{{ ternary "enabled" "disabled" .feature_flag }}` |
| `toYaml`          | `{{ toYaml .config \| indent 2 }}`                 |

See the [sprig documentation][sprig] for the full function reference.

### Conditional file generation

A template that renders to an empty or whitespace-only string causes the
corresponding output file to be deleted (if it exists) or skipped (if it does
not). This lets you conditionally exclude files based on variable values:

```Dockerfile
{{- if .enable_docker -}}
FROM golang:{{ .go_version }}-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o /app/bin/app ./cmd/app
{{- end -}}
```

If the `enable_docker` variable is set to `false` (or the variable is absent) in
the `.afon.yaml` configuration file, `Dockerfile.tmpl` renders to an empty
string and `Dockerfile` is removed from the downstream repository.

## CLI reference

```shell
afon [global flags] <command> [flags]
```

### Global flags

| Flag          | Default | Description                                                                                                                         |
| ------------- | ------- | ----------------------------------------------------------------------------------------------------------------------------------- |
| `--log-level` | `info`  | Log level: `debug`, `info`, `warn`, or `error`. Automatically defaults to `debug` in GitHub Actions when step debugging is enabled. |

### `afon apply`

Apply the upstream template to the current directory.

```shell
afon apply [flags]
```

| Flag          | Short | Default      | Description                                                                                    |
| ------------- | ----- | ------------ | ---------------------------------------------------------------------------------------------- |
| `--config`    | `-c`  | `.afon.yaml` | Path to the configuration file                                                                 |
| `--template`  | `-t`  |              | Path or URL to the upstream template repository (overrides `template.source`)                  |
| `--reference` | `-r`  |              | Branch, tag, or commit reference (overrides `template.reference`)                              |
| `--path`      | `-p`  |              | Subdirectory within the template repository (overrides `template.path`)                        |
| `--output`    | `-o`  | `.`          | Output directory                                                                               |
| `--token`     |       |              | Personal access token for private repositories (overrides `template.token` and `GITHUB_TOKEN`) |

## Environment variables

| Variable             | Description                                                                                                                          |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------ |
| `GITHUB_TOKEN`       | Authentication token for cloning private remote template repositories. Used when `--token` is not set and `template.token` is empty. |
| `GITHUB_ACTIONS`     | When `true` (set automatically inside GitHub Actions), combined with `ACTIONS_STEP_DEBUG=true` to default the log level to `debug`.  |
| `ACTIONS_STEP_DEBUG` | When `true` alongside `GITHUB_ACTIONS=true`, enables debug-level logging automatically.                                              |

## Using as a GitHub Action

Reference the action from any downstream repository workflow:

> [!CAUTION]
> When using `afon` in a GitHub Workflow, ensure that the workflow has write
> permissions for `contents` and `pull-requests` to allow the action to commit
> changes and open pull requests.
>
> Additionally, it is **highly recommended** that you always pin the full SHA
> reference for the GitHub Action to ensure supply chain security in production
> workflows.

```yaml
jobs:
    afon:
        permissions:
            contents: write
            pull-requests: write
        steps:
            - name: Apply upstream templates with afon
              uses: n3tuk/afon@latest
```

The action reads `.afon.yaml` from the root of the checked-out workspace. No
additional inputs are required — all configuration is handled through the
configuration file and environment variables.

### Full downstream workflow

A ready-to-use example workflow is provided at
[`examples/workflows/afon.yaml`](examples/workflows/afon.yaml). Copy it to
`.github/workflows/afon.yaml` in your downstream repository:

```yaml
name: Repository Template Updates
run-name: Render & Apply Upstream Templates

on:
    schedule:
        - cron: 0 6 * * 1 # Every Monday at 06:00
    workflow_dispatch: # Or on-demand

jobs:
    apply:
        name: afon
        runs-on: ubuntu-latest
        permissions:
            contents: write
            pull-requests: write
        steps:
            - name: Checkout the repository
              uses: actions/checkout@v4
            - name: Apply upstream templates with afon
              uses: n3tuk/afon@latest
            - name: Open a pull request for changes
              uses: peter-evans/create-pull-request@v7
              with:
                  title: >-
                      chore(repository): Apply upstream template changes
                  body: |-
                      Automated pull request created by
                      [afon](https://github.com/n3tuk/afon).
                  commit-message: >-
                      chore: Apply upstream template changes
                  branch: chore/apply-template-updates
                  delete-branch: true
```

The workflow checks out the downstream repository, runs `afon`, and opens a
pull request for any resulting changes.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for instructions on setting up the
development environment, running tests, linting, and submitting pull requests.

## Authors

- Jonathan Wright (<jon@than.io>)

## Licence

[MIT](LICENSE)
