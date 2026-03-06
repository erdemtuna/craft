# Spec: Craft Polish — Package Operations & UI

## Overview

Workflow 3 of 3 for craft (Agent Skills Package Manager). Adds the remaining two CLI commands (`craft add`, `craft remove`), introduces npm-style UI with progress indicators and dependency tree visualization, improves multi-agent detection UX, enhances error messages, and delivers project documentation (README).

## User Stories

### US-1: Adding a Dependency
**As a** skill author,
**I want to** run `craft add github.com/org/repo@v1.0.0`
**So that** the dependency is added to my `craft.yaml`, verified, and optionally installed — without manually editing YAML.

**Acceptance Criteria:**
- Accepts git URL format: `github.com/org/repo@v1.0.0`
- Accepts optional alias: `craft add my-alias github.com/org/repo@v1.0.0`
- Resolves the dependency to verify it exists, has valid skills, and does not collide
- Adds the dependency entry to `craft.yaml` (preserving existing content)
- If `--install` flag is passed, runs the full install pipeline after adding
- If the dependency already exists in `craft.yaml`, updates the version (with confirmation message)
- Prints summary: skill names discovered, version resolved
- On failure: clear error with suggestion (repo not found → auth hint; no skills → check SKILL.md)

### US-2: Removing a Dependency
**As a** skill author,
**I want to** run `craft remove <alias>`
**So that** the dependency and its orphaned skills are cleaned up from my project.

**Acceptance Criteria:**
- Accepts the dependency alias (the key in `craft.yaml` dependencies map)
- Removes the dependency from `craft.yaml`
- Removes the dependency's entry from `craft.pin.yaml` (if pinfile exists)
- Identifies orphaned skills (skills only provided by the removed dependency, not by any remaining dependency)
- Removes orphaned skill directories from the install target
- Prints summary: what was removed, which skills were cleaned up
- If alias doesn't exist in `craft.yaml`, error with available aliases listed
- Handles transitive deps: only removes skills that are truly orphaned (not needed by other remaining deps)

### US-3: Progress Indicators
**As a** skill consumer,
**I want to** see progress during `craft install` and `craft update`
**So that** I know the tool is working and approximately how long it will take.

**Acceptance Criteria:**
- Shows spinner/status line during: git fetch, dependency resolution, skill installation
- Displays phase transitions: "Resolving dependencies...", "Fetching github.com/org/repo...", "Installing skills..."
- Shows count progress: "Fetching dependency 2/5..."
- On completion: summary line with counts (e.g., "Installed 8 skills from 3 packages")
- On error: progress stops, error is displayed clearly
- Output goes to stderr (so stdout remains clean for scripting)
- No progress output when stderr is not a TTY (CI-friendly)

### US-4: Dependency Tree Visualization
**As a** skill consumer,
**I want to** see a tree view of my resolved dependencies after install
**So that** I understand what was installed and where skills came from.

**Acceptance Criteria:**
- After successful install/update, prints a tree like:
  ```
  code-quality@1.0.0
  ├── git-operations (github.com/example/git-skills@v1.0.0)
  │   ├── git-commit
  │   ├── git-branch
  │   └── git-operations
  └── style-guides (github.com/other-org/style-skills@v2.3.1)
      ├── python-style
      └── js-style
  ```
- Tree shows: package name, dependency alias, URL+version, and skill names under each
- Local skills (from `skills[]`) shown separately at the top
- Output goes to stderr

### US-5: Multi-Agent Detection
**As a** user with both Claude Code and GitHub Copilot installed,
**I want to** be prompted to choose an install target
**So that** I'm not blocked by an error when both agents are detected.

**Acceptance Criteria:**
- When multiple agents detected, prompt interactively: Claude Code / Copilot / Both
- When stdin is not a TTY, fall back to error with `--target` suggestion (non-interactive)
- `--target` flag always overrides detection (no prompt)
- Persist nothing for MVP — ask each time (config persistence is post-MVP)

### US-6: Improved Error Messages
**As a** craft user,
**I want to** see actionable error messages with suggestions
**So that** I can fix issues without reading documentation.

**Acceptance Criteria:**
- All errors follow pattern: `error: <what happened>\n  hint: <what to do>`
- Missing `craft.yaml` → "Run `craft init` to create one"
- Repository not found → "Check the URL or set GITHUB_TOKEN for private repos"
- No skills found in dependency → "Ensure the repo has SKILL.md files"
- Dependency alias not found → list available aliases
- Version tag doesn't exist → list available tags (up to 10)
- Network errors → "Check your connection or run with cached data"

### US-7: README Documentation
**As a** potential craft user,
**I want to** read a comprehensive README
**So that** I can understand what craft does, how to install it, and how to use it.

**Acceptance Criteria:**
- Sections: Overview, Installation, Quick Start, Commands Reference, Configuration, craft.yaml Reference, craft.pin.yaml Reference, Agent Support, Known Limitations
- Installation: `go install` command, binary download mention
- Quick Start: end-to-end example from `craft init` to `craft install`
- Each command documented with usage, flags, and examples
- Known limitations documented (go-git SSH, monorepo paths, etc.)

## Requirements

### Functional Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| FR-1 | `craft add <url>` adds dependency to craft.yaml | Must |
| FR-2 | `craft add <alias> <url>` adds with custom alias | Must |
| FR-3 | `craft add --install` triggers install after add | Must |
| FR-4 | `craft add` validates dependency resolves before committing to manifest | Must |
| FR-5 | `craft remove <alias>` removes dependency from craft.yaml | Must |
| FR-6 | `craft remove` cleans up orphaned skills from install target | Must |
| FR-7 | `craft remove` updates craft.pin.yaml | Must |
| FR-8 | Progress indicators during install/update operations | Must |
| FR-9 | Dependency tree visualization after install/update | Must |
| FR-10 | Interactive multi-agent prompt when multiple agents detected | Should |
| FR-11 | Improved error messages with hints across all commands | Must |
| FR-12 | README with complete usage documentation | Must |

### Non-Functional Requirements

| ID | Requirement |
|----|-------------|
| NFR-1 | Progress output goes to stderr; data output to stdout |
| NFR-2 | No progress/color when stderr is not a TTY (CI-friendly) |
| NFR-3 | No new external dependencies for progress UI (use stdlib) |
| NFR-4 | All new code has unit tests |
| NFR-5 | Existing tests continue to pass |

## Technical Constraints

- **No new dependencies**: Progress UI implemented with stdlib (ANSI escape codes + `\r` carriage return). No external terminal UI libraries.
- **TTY detection**: Use `os.Stderr.Fd()` with `golang.org/x/term.IsTerminal()` — already a transitive dependency via go-git's SSH code. If not available, use simple `os.Stat()` check.
- **Atomic manifest writes**: `craft add` and `craft remove` must write craft.yaml atomically (temp file + rename), matching existing patterns.
- **Existing interfaces**: `craft add` reuses the resolver's `Resolve()` for validation. No new fetcher interface methods needed.

## Success Criteria

1. `craft add github.com/org/repo@v1.0.0` adds the dependency, reports discovered skills
2. `craft remove <alias>` removes the dependency, cleans orphaned skills, updates pinfile
3. `craft install` shows progress phases and a dependency tree on completion
4. Multi-agent detection prompts instead of erroring
5. All error messages include actionable hints
6. README covers all 8 commands with examples
7. All existing tests pass; new commands have full test coverage
8. `go vet` and `go build` pass cleanly
