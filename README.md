# craft

Agent Skills package manager — resolve, install, and manage skill dependencies.

## Overview

craft is a CLI tool for managing [Agent Skills](https://agentskills.io/specification) packages. It provides a manifest and pinfile system — analogous to Go modules — for declaring, resolving, and installing skill dependencies across git repositories.

## Installation

```bash
go install github.com/erdemtuna/craft/cmd/craft@latest
```

Or build from source:

```bash
git clone https://github.com/erdemtuna/craft.git
cd craft
go build -o craft ./cmd/craft
```

## Quick Start

```bash
# Initialize a new package (interactive)
craft init

# Install dependencies
craft install

# Update all dependencies to latest versions
craft update

# Validate your package
craft validate

# Check version
craft version
```

## craft.yaml

The package manifest declares identity, exported skills, and dependencies:

```yaml
schema_version: 1
name: my-skills
version: 1.0.0
description: My collection of agent skills.
license: MIT

skills:
  - ./skills/code-review
  - ./skills/test-gen

dependencies:
  utils: github.com/example/util-skills@v1.0.0
  helpers: github.com/other-org/helper-skills@v2.1.0
```

**Naming rules**: Package and skill names must be lowercase alphanumeric with hyphens (`my-package`). Versions follow strict semver (`MAJOR.MINOR.PATCH`).

**Dependency URLs**: `host/org/repo@vMAJOR.MINOR.PATCH` (e.g., `github.com/example/skills@v1.0.0`).

## SKILL.md

Each skill directory must contain a `SKILL.md` file with YAML frontmatter:

```markdown
---
name: code-review
description: Automated code review skill.
---

# Code Review
...
```

## Commands

| Command | Description |
|---------|-------------|
| `craft init` | Interactive package setup with skill auto-discovery |
| `craft install` | Resolve, pin, and install all dependencies |
| `craft update [alias]` | Update dependencies to latest semver tags |
| `craft validate` | Run all validation checks (schema, paths, frontmatter, deps, collisions) |
| `craft version` | Print version and exit |

### craft install

Resolves all dependencies declared in `craft.yaml`, writes a deterministic `craft.pin.yaml`, and installs skill directories to the detected agent's skill path.

```bash
# Install to auto-detected agent path
craft install

# Install to a custom path
craft install --target /path/to/skills
```

**Resolution**: Uses Minimum Version Selection (MVS) to resolve transitive dependencies. Detects cycles and skill name collisions across the full dependency tree.

**Caching**: Repositories are cached as bare clones at `~/.craft/cache/` to avoid redundant network fetches.

**Pinfile reuse**: If `craft.pin.yaml` exists and matches `craft.yaml`, pinned versions are used without re-resolving.

### craft update

Re-resolves dependencies to the latest available semver tags.

```bash
# Update all dependencies
craft update

# Update a single dependency
craft update utils
```

Updates both `craft.yaml` (new version tags) and `craft.pin.yaml` (new commits and integrity digests).

## Authentication

For private repositories, set one of these environment variables:

| Variable | Priority | Description |
|----------|----------|-------------|
| `CRAFT_TOKEN` | Highest | Personal access token for HTTPS auth |
| `GITHUB_TOKEN` | Medium | GitHub token for HTTPS auth |
| SSH agent | Fallback | SSH keys via ssh-agent |

```bash
export GITHUB_TOKEN=ghp_your_token_here
craft install
```

## Agent Detection

craft auto-detects your AI agent and installs skills to the correct path:

| Agent | Marker | Install Path |
|-------|--------|-------------|
| Claude Code | `~/.claude/` | `~/.claude/skills/<skill-name>/` |
| GitHub Copilot | `~/.copilot/` | `~/.copilot/skills/<skill-name>/` |

When multiple agents are detected, Claude Code takes precedence. Use `--target` to override.

## Cache

Fetched repositories are cached at `~/.craft/cache/` as bare git clones. The cache is keyed by repository URL (one entry per repo, serving all versions).

## License

MIT
