# craft

Agent Skills package manager — resolve, install, and manage skill dependencies.

## The Problem

Agent Skills for AI coding agents (Copilot, Claude Code, etc.) are becoming a real pattern. And skills naturally compose — a code-review skill needs git utilities, a test-generation skill needs code parsing, a deployment skill needs environment detection.

But there's no official way to declare or manage these dependencies. Without proper dependency management, you get:

- **Redundancy** — the same utility skill copy-pasted across dozens of repos
- **Drift** — copied skills diverge over time, bugs fixed in one copy but not others
- **Fragility** — no version pinning or integrity checks, so upstream changes silently break things

craft fixes this. It's a package manager for Agent Skills — think Go modules, but for skills.

## How It Works

craft borrows directly from Go's dependency model:

| craft | Go | Purpose |
|-------|-----|---------|
| `craft.yaml` | `go.mod` | Declare what you export and what you depend on |
| `craft.pin.yaml` | `go.sum` | Lock exact git commits + SHA-256 integrity digests |
| `SKILL.md` | package doc | Skill metadata (name, description) |

Dependencies are resolved using **Minimal Version Selection (MVS)**, fetched from git, and cached locally at `~/.craft/cache/`. Every resolved dependency is pinned to an exact commit SHA and verified with a SHA-256 integrity digest — no surprises.

## Example

Say your organization has a set of API conventions — naming rules, error response formats, pagination patterns, auth standards. Today these live in a wiki page that nobody reads. You encode them into a `company-standards` skill package so every AI coding agent in the org follows the same rules:

```yaml
# craft.yaml — published by the platform team
schema_version: 1
name: company-standards
version: 2.1.0
description: Org-wide API conventions, error formats, and naming rules.
license: MIT

skills:
  - ./skills/api-conventions
  - ./skills/error-formats
```

Now you're building a `code-reviewer` skill that reviews pull requests. It needs to check PRs against those org standards — not a copy-pasted snapshot that goes stale, but the real, versioned source of truth:

```yaml
# craft.yaml — your team's skill package
schema_version: 1
name: code-reviewer
version: 1.0.0
description: PR review skill that enforces org standards.
license: MIT

skills:
  - ./skills/review-pr

dependencies:
  standards: github.com/acme/company-standards@v2.1.0
```

Meanwhile, the docs team's `doc-generator` skill and the infra team's `api-designer` skill all depend on the same `company-standards` package. When the platform team updates the conventions to v2.2.0, every team bumps one version number and gets the update — no copy-paste drift, no stale rules.

**Set up and validate:**

```bash
# Initialize a new package — auto-discovers SKILL.md files and walks you through setup
$ craft init

# Validate everything: schema, skill paths, frontmatter, dependency URLs, collision checks
$ craft validate
✓ craft.yaml schema valid
✓ All skill paths resolve
✓ SKILL.md frontmatter valid for review-pr
✓ Dependency URLs well-formed
✓ No skill name collisions

# Resolve, pin, and install dependencies to your agent's skill directory
$ craft install
```

After install, `craft.pin.yaml` locks every dependency to an exact state:

```yaml
# craft.pin.yaml (generated — do not edit)
pin_version: 1

resolved:
  github.com/acme/company-standards@v2.1.0:
    commit: a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
    integrity: sha256-Xk9jR2mN5pQ8vW3yB7cF1dA4hL6tS0uE9iO2wR5nM3s=
    skills:
      - api-conventions
      - error-formats
```

Commit this alongside `craft.yaml`. Anyone who runs `craft install` gets the exact same dependency tree.

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

## Commands

| Command | Description |
|---------|-------------|
| `craft init` | Interactive package setup with skill auto-discovery |
| `craft install` | Resolve, pin, and install all dependencies |
| `craft update [alias]` | Update dependencies to latest semver tags |
| `craft validate` | Run all validation checks (schema, paths, frontmatter, deps, collisions) |
| `craft version` | Print version and exit |

## Manifest Reference (`craft.yaml`)

```yaml
schema_version: 1          # Always 1
name: my-package            # Lowercase alphanumeric + hyphens
version: 1.0.0              # Strict semver (MAJOR.MINOR.PATCH)
description: …              # Optional
license: MIT                # Optional

skills:                     # Relative paths to skill directories
  - ./skills/my-skill

dependencies:               # alias → host/org/repo@vX.Y.Z
  utils: github.com/example/util-skills@v1.0.0
```

## `SKILL.md`

Each skill directory must contain a `SKILL.md` file — a markdown file with YAML frontmatter that declares the skill's identity. craft parses the frontmatter to extract the `name` and `description` fields, and preserves any additional fields for forward compatibility.

For the full specification and a real-world example, see [Anthropic's skill-creator](https://github.com/anthropics/skills/tree/main/skills/skill-creator) — the canonical reference for the Agent Skills format that craft builds on.

## Acknowledgments

This project wouldn't exist without [Rob Emanuele](https://github.com/lossyrob)'s [PAW (Phased Agent Workflow)](https://github.com/lossyrob/phased-agent-workflow). Working with PAW — writing new skills, extending existing ones, watching them grow in complexity — is what made the dependency problem impossible to ignore. When your skills start depending on other skills and there's no way to say so, you feel it. craft is the answer to that friction.

## License

MIT
