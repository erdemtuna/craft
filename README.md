# craft

Agent Skills package manager — resolve, install, and manage skill dependencies.

## Overview

craft is a CLI tool for managing [Agent Skills](https://agentskills.io/specification) packages. It provides a manifest and pinfile system — analogous to Go modules — for declaring, validating, and (in future releases) resolving skill dependencies across repositories.

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
```

**Naming rules**: Package and skill names must be lowercase alphanumeric with hyphens (`my-package`). Versions follow strict semver (`MAJOR.MINOR.PATCH`).

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
| `craft validate` | Run all validation checks (schema, paths, frontmatter, deps, collisions) |
| `craft version` | Print version and exit |

## License

MIT
