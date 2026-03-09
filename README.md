# craft

Agent Skills package manager — resolve, install, and manage skill dependencies.

## The Problem

Agent Skills for AI coding agents (Copilot, Claude Code, etc.) are becoming a real pattern. And skills naturally compose — a code-review skill needs git utilities, a test-generation skill needs code parsing, a deployment skill needs environment detection.

But there's no official way to declare or manage these dependencies. Without proper dependency management, you get:

- **Redundancy** — the same utility skill copy-pasted across dozens of repos
- **Drift** — copied skills diverge over time, bugs fixed in one copy but not others
- **Fragility** — no version pinning or integrity checks, so upstream changes silently break things

craft fixes this. It's a package manager for Agent Skills — think Go modules, but for skills.

## Quick Start: Install Skills

Found a skill package you want to use? One command:

```bash
$ craft get github.com/acme/company-standards@v2.1.0
Installed 2 skill(s) from 1 package(s) to /home/user/.claude/skills
```

That's it — skills are installed to your AI agent and tracked for updates. No project setup needed.

```bash
# Manage what you've installed
$ craft list -g              # list globally installed skills
$ craft update -g            # update to newer versions
$ craft remove -g standards  # uninstall
```

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

# Resolve, pin, and vendor dependencies to forge/ (project-local)
$ craft install
```

After install, `craft.pin.yaml` locks every dependency to an exact state, and `forge/` contains the vendored skill files (gitignored, reproducible from the pinfile):

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

## Depending on Repos That Don't Use craft

Not every skill repo has a `craft.yaml` — and that's fine. craft doesn't require upstream repos to adopt it.

When you depend on a repo that has no manifest, craft falls back to **auto-discovery**: it scans the repo's file tree for `SKILL.md` files, parses their frontmatter, and treats each one as a skill. The dependency is pinned to an exact commit and integrity-checked just like any other.

```yaml
# This works even if acme/legacy-skills has no craft.yaml
dependencies:
  legacy: github.com/acme/legacy-skills@v1.0.0           # if they have tags
  legacy: github.com/acme/legacy-skills@branch:main       # or track a branch
  legacy: github.com/acme/legacy-skills@abc1234def5678    # or pin a commit
```

The only difference: repos without `craft.yaml` are treated as **leaf dependencies** — craft can't resolve transitive dependencies from them because there's no manifest declaring any. If `legacy-skills` itself depends on other packages, those won't be pulled in automatically. Once the upstream repo adds a `craft.yaml`, transitive resolution kicks in with no changes on your side.

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
| `craft get [alias] <url> [url...]` | Install skills globally to your AI agent |
| `craft init` | Interactive package setup with skill auto-discovery |
| `craft install` | Resolve, pin, and vendor dependencies to `forge/` |
| `craft update [alias]` | Update dependencies (re-resolve branches, skip commit pins, bump tags) |
| `craft add [alias] <url>` | Add a dependency (verify, then update manifest) |
| `craft remove <alias>` | Remove a dependency and clean up orphaned skills |
| `craft list` | List resolved dependencies with versions and skill counts |
| `craft tree` | Print the dependency tree |
| `craft outdated` | Show available dependency updates |
| `craft validate` | Run all validation checks (schema, paths, frontmatter, deps, collisions, non-tagged warnings) |
| `craft cache clean` | Remove all cached repositories from `~/.craft/cache/` |
| `craft version` | Print version and exit |

All management commands support `--global` / `-g` to operate on globally installed skills instead of the project manifest.

### `craft get`

Install skills from any repository directly into your AI agent's skill directory. No `craft init` or project setup required.

```bash
# Install a skill package
$ craft get github.com/acme/company-standards@v2.1.0
Installed 2 skill(s) from 1 package(s) to /home/user/.claude/skills

# Install with a custom alias
$ craft get standards github.com/acme/company-standards@v2.1.0

# Install multiple packages at once
$ craft get github.com/acme/standards@v2.1.0 github.com/acme/utils@v1.0.0

# Preview what would be installed
$ craft get --dry-run github.com/acme/standards@v2.1.0
```

Global state is tracked at `~/.craft/craft.yaml` and `~/.craft/craft.pin.yaml` — auto-created on first use.

### `craft add`

Add a dependency to your `craft.yaml`. The dependency is resolved and verified before the manifest is updated.

```bash
# Add with auto-derived alias (uses repo name)
$ craft add github.com/acme/utility-skills@v1.0.0
Added "utility-skills" → github.com/acme/utility-skills@v1.0.0
  skills: git-helper, file-parser
  version: v1.0.0

# Add with custom alias
$ craft add utils github.com/acme/utility-skills@v1.0.0

# Add and immediately vendor to forge/
$ craft add --install github.com/acme/utility-skills@v1.0.0

# Add a dependency from a branch (for repos without tags)
$ craft add github.com/acme/experimental@branch:main
⚠ Non-tagged dependency: github.com/acme/experimental@branch:main
  Branch-tracked deps have weaker reproducibility guarantees.
Added "experimental" → github.com/acme/experimental@branch:main
  branch: main

# Add a dependency pinned to a specific commit
$ craft add github.com/acme/tools@abc1234def5678
⚠ Non-tagged dependency: github.com/acme/tools@abc1234def5678
  Commit-pinned deps are reproducible but frozen; no updates available.
Added "tools" → github.com/acme/tools@abc1234def5678
  commit: abc1234def56
```

### `craft remove`

Remove a dependency and clean up skills that are no longer needed by any remaining dependency.

```bash
$ craft remove utils
Removed "utils" (github.com/acme/utility-skills@v1.0.0)
  cleaned up 2 orphaned skill(s): git-helper, file-parser
```

Skills shared with other dependencies are retained — only truly orphaned skills are removed.

### `craft cache clean`

Remove all cached git repositories from `~/.craft/cache/`.

```bash
$ craft cache clean
Removed cache directory: /home/user/.craft/cache
```

### `craft list`

Show all resolved dependencies from `craft.pin.yaml`.

```bash
$ craft list
company-standards  v2.1.0  (2 skills)
utility-skills     v1.0.0  (1 skill)

# Show extended information including URLs and skill names
$ craft list --detailed
company-standards  v2.1.0  github.com/org/standards
  skills: api-conventions, error-formats

utility-skills     v1.0.0  github.com/org/utils
  skills: git-helper
```

### `craft tree`

Print the dependency tree showing local skills and all resolved dependencies.

```bash
$ craft tree
my-package
├── Local skills
│   ├── skill-a
│   └── skill-b
├── company-standards (github.com/org/standards@v2.1.0)
│   ├── api-conventions
│   └── error-formats
└── utility-skills (github.com/org/utils@v1.0.0)
    └── git-helper
```

### `craft outdated`

Check each direct dependency for newer versions. Exits with code 1 when updates are available (useful for CI).

```bash
$ craft outdated
company-standards  v2.1.0 → v2.2.0  (minor)
utility-skills     v1.0.0            (up to date)

$ echo $?
1
```

### Global Flags

| Flag | Description |
|------|-------------|
| `--verbose`, `-v` | Enable verbose diagnostic output (shows fetches, version comparisons, cache operations) |
| `--global`, `-g` | Operate on globally installed skills (`~/.craft/`) instead of the project manifest |

### Operation Flags

| Flag | Available On | Description |
|------|-------------|-------------|
| `--dry-run` | `install`, `update`, `get` | Show what would happen without making any changes |
| `--target <path>` | `install`, `update`, `remove`, `get` | Override agent auto-detection with a custom install path |

## Manifest Reference (`craft.yaml`)

```yaml
schema_version: 1          # Always 1
name: my-package            # Lowercase alphanumeric + hyphens
description: …              # Optional
license: MIT                # Optional

skills:                     # Relative paths to skill directories
  - ./skills/my-skill

dependencies:               # alias → host/org/repo@<ref>
  utils: github.com/example/util-skills@v1.0.0           # tagged version
  tools: github.com/example/dev-tools@branch:main        # branch tracking
  legacy: github.com/example/old-skills@abc1234def5678   # commit pin
```

## `SKILL.md`

Each skill directory must contain a `SKILL.md` file — a markdown file with YAML frontmatter that declares the skill's identity. craft parses the frontmatter to extract the `name` and `description` fields, and preserves any additional fields for forward compatibility.

For the full specification and a real-world example, see [Anthropic's skill-creator](https://github.com/anthropics/skills/tree/main/skills/skill-creator) — the canonical reference for the Agent Skills format that craft builds on.

## Agent Support

craft auto-detects your AI agent and installs skills to the correct directory:

| Agent | Marker | Install Path |
|-------|--------|-------------|
| Claude Code | `~/.claude/` | `~/.claude/skills/` |
| GitHub Copilot | `~/.copilot/` | `~/.copilot/skills/` |

When both agents are detected, craft prompts you to choose. Use `--target <path>` to override auto-detection.

**Global installs** (`craft get`, `craft install -g`) write to agent skill directories.  
**Project installs** (`craft install`) vendor to `forge/` in the project root (gitignored).

## Known Limitations

- **go-git SSH limitations** — craft uses [go-git](https://github.com/go-git/go-git) for git operations. This means no `~/.ssh/config` ProxyJump support, no hardware token (YubiKey) auth, and no agent forwarding. Set `GITHUB_TOKEN` or `CRAFT_TOKEN` for private repos as a reliable alternative.
- **No monorepo subpath support** — dependency URLs point to whole repositories (`github.com/org/repo@v1.0.0`). Subpath support (e.g., `repo/path/to/skills@v1`) is designed for but not yet implemented.
- **No pre-release versions** — tagged version refs must be strict semver (`vMAJOR.MINOR.PATCH`). Pre-release suffixes like `-beta.1` are rejected. (Commit SHA and branch refs are unaffected.)
- **No version ranges** — tagged dependencies use exact versions. `craft update` bumps tags to the latest available, re-resolves branch deps to HEAD, and skips commit pins; there are no `^` or `~` range constraints.
- **Cache grows unbounded** — use `craft cache clean` periodically to reclaim disk space.

## Acknowledgments

This project wouldn't exist without [Rob Emanuele](https://github.com/lossyrob)'s [PAW (Phased Agent Workflow)](https://github.com/lossyrob/phased-agent-workflow). Working with PAW — writing new skills, extending existing ones — made me realize the dependency problem.

## Development

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full developer guide — setup, code quality tools, git hooks, testing, and CI/CD.

Quick start:

```bash
task ci   # run the full CI pipeline locally (fmt, vet, lint, vuln, test, build)
```

## License

MIT
