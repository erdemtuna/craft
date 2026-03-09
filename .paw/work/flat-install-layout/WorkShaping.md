# Work Shaping: Flat Install Layout for Agent Directories

## Problem Statement

**Who benefits:** Users who install craft skills globally for AI agents (Claude Code, GitHub Copilot).

**What problem is solved:** Nested `github.com/org/repo/skill/` directory paths break agent skill discovery. Agents expect flat directories under their skills root (e.g., `~/.claude/skills/my-skill/`), not deeply nested trees. The current composite-key layout, while excellent for project-scoped `forge/` vendoring, creates paths that agents cannot traverse.

**Example of the problem:**
```
# Current (broken for agents):
~/.claude/skills/github.com/lossyrob/phased-agent-workflow/paw-implement/SKILL.md

# Desired (agent-discoverable):
~/.claude/skills/github-com--lossyrob--phased-agent-workflow--paw-implement/SKILL.md
```

## Work Breakdown

### Core Functionality

1. **`FlatKey()` function** in `internal/install/installer.go`
   - Input: composite key `github.com/org/repo/skill`
   - Output: flat key `github-com--org--repo--skill`
   - Rules:
     - `/` → `--` (double-dash separates path components)
     - `.` → `-` (dots become single dashes, applied to ALL segments)
     - Casing preserved (no lowercasing)
   - `--` separator is collision-safe: GitHub org/repo names and directory-derived skill names cannot contain `--`

2. **`InstallFlat()` method** in `internal/install/installer.go`
   - Same atomic staging + swap semantics as `Install()`
   - Same path traversal security validation
   - Transforms composite keys via `FlatKey()` before writing
   - Encapsulates the flat layout decision within the install package

3. **CLI wiring** in `install.go`, `get.go`, `update.go`
   - Global installs → call `InstallFlat()` instead of `Install()`
   - Project installs → continue calling `Install()` (nested layout in `forge/`)

4. **Remove cleanup** in `remove.go`
   - Global removes: use `FlatKey()` to build directory name, `os.RemoveAll()`, skip `cleanEmptyParents` (no parents to clean in flat layout)
   - Project removes: keep existing nested cleanup with `cleanEmptyParents`

### Supporting Work

5. **Tests** in `installer_test.go`
   - 5+ new tests for `InstallFlat()`/`FlatKey()`
   - Existing tests (including `TestInstallCompositeKeys`) remain unchanged

6. **Documentation** updates to `README.md` and `E2E_REAL_WORLD_TEST.md`
   - Update expected directory structures for global installs
   - Document flat key format

## Edge Cases

| Edge Case | Expected Handling |
|-----------|-------------------|
| Dots in non-host segments (e.g., `github.com/my.org/repo/skill`) | All dots → dashes: `github-com--my-org--repo--skill` |
| Skill names with dashes (e.g., `paw-implement`) | Preserved as-is; `--` separator is unambiguous |
| Mixed casing (e.g., `GitHub.com/MyOrg/Repo/Skill`) | Preserved: `GitHub-com--MyOrg--Repo--Skill` |
| Existing nested global installs | N/A | No existing installs use nested layout — clean-slate change |
| Project-scoped installs (`forge/`) | Unchanged — continue using nested composite-key layout |
| Remove after flat install | Pinfile has original composite keys; apply `FlatKey()` to derive directory name |
| `cleanEmptyParents` on global remove | No-op — flat dirs have no parents to clean |

## Architecture

### Component Interactions

```
CLI Layer (install.go / get.go / update.go / remove.go)
    │
    ├── Global scope ──→ InstallFlat(agentSkillsDir, skills)
    │                         │
    │                         ├── FlatKey(compositeKey) → flat directory name
    │                         ├── Staging + atomic swap (same as Install)
    │                         └── Path traversal validation (same as Install)
    │
    └── Project scope ──→ Install(forgeDir, skills)  [unchanged]
```

### Data Flow for Global Install

```
craft.pin.yaml        CLI (install.go)          installer.go
┌──────────────┐      ┌──────────────────┐      ┌───────────────────┐
│ skills:      │      │                  │      │ InstallFlat()     │
│  github.com/ │ ──→  │ collectSkillFiles│ ──→  │  for each skill:  │
│   org/repo/  │      │ (composite keys) │      │   FlatKey(key)    │
│    skill     │      │                  │      │   stage + swap    │
└──────────────┘      └──────────────────┘      └───────────────────┘
                                                         │
                                                         ▼
                                                ~/.claude/skills/
                                                  github-com--org--repo--skill/
                                                    SKILL.md
```

### Data Flow for Global Remove

```
craft.pin.yaml        CLI (remove.go)           filesystem
┌──────────────┐      ┌──────────────────┐      ┌────────────────────────┐
│ skills:      │      │ for each skill:  │      │                        │
│  github.com/ │ ──→  │  FlatKey(key)    │ ──→  │ os.RemoveAll(          │
│   org/repo/  │      │  build path      │      │   target/flat-key/)    │
│    skill     │      │  (no parent      │      │ (no parent cleanup)    │
└──────────────┘      │   cleanup)       │      └────────────────────────┘
                      └──────────────────┘
```

## Critical Analysis

### Value Assessment

- **High value**: This is a blocking issue — global installs are currently non-functional for agent discovery
- **Low risk**: The change is well-scoped — `Install()` is untouched, `InstallFlat()` is additive
- **Clean separation**: Project (nested) vs global (flat) is a clear boundary

### Build vs Modify Tradeoffs

- **New code**: `FlatKey()` (~10 lines), `InstallFlat()` (~15 lines wrapping `Install()` with key transform)
- **Modified code**: 4 CLI files (routing to `InstallFlat` for global), 1 file (`remove.go` for flat cleanup)
- **Total new code**: Minimal — most logic is reused from existing `Install()`

### No Backward Compatibility Concern

- No existing global installs use the nested layout — clean-slate change
- No migration, no orphaned directories, no stale-remove risk

## Codebase Fit

### Reuse Opportunities

- `InstallFlat()` can delegate to `Install()` after transforming keys, reusing all atomic staging, path validation, and file-writing logic
- `FlatKey()` is a pure function — easy to test in isolation
- Remove logic already looks up skills from pinfile — just needs `FlatKey()` applied for global scope

### Similar Patterns

- The existing `Install()` → staging → atomic swap pattern is well-established and proven
- CLI already branches on `globalFlag` for project vs global behavior — adding `InstallFlat()` call follows existing pattern

## Risk Assessment

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| N/A — no existing nested global installs | — | — | Clean-slate; no migration needed |
| `--` separator collision | Very Low | High | Validated: GitHub disallows `--` in org/repo names |
| Dot-to-dash creates ambiguous keys | Very Low | Medium | In practice, dots in org/repo/skill names are extremely rare |
| `InstallFlat()` diverges from `Install()` over time | Low | Medium | `InstallFlat()` should delegate to `Install()` internally |

## Implementation Phases (from brief)

| Phase | Files | What |
|-------|-------|------|
| 1 | `installer.go` | Add `FlatKey()` + `InstallFlat()` |
| 2 | `install.go`, `get.go`, `update.go` | Wire `InstallFlat()` for global paths |
| 3 | `remove.go` | Use `FlatKey()` for global cleanup, keep nested for project |
| 4 | `installer_test.go` | 5+ new tests (FlatKey, basic, multi-package, same-name, overwrite) + FlatKey edge case tests |
| 5 | `README.md`, `E2E_REAL_WORLD_TEST.md` | Update expected directory structures |

## Open Questions for Downstream Stages

1. **Should `InstallFlat()` be a wrapper that transforms keys then calls `Install()`, or a parallel implementation?** (Recommendation: wrapper/delegate to minimize code duplication)
2. **Should `craft list -g` / `craft tree -g` display flat keys or reconstructed composite keys?** (Display is cosmetic; doesn't affect this feature but worth deciding)

## Session Notes

- **Key decision**: No backward compatibility / no migration. Clean break.
- **Key decision**: `--` separator validated as collision-safe by user.
- **Key decision**: Dots replaced in ALL segments, not just host.
- **Key decision**: Casing preserved (no lowercasing).
- **Key decision**: `FlatKey()` + `InstallFlat()` encapsulated in install package (not CLI-layer transform).
- **Key decision**: Existing tests unchanged; new tests added alongside.
- **Key decision**: `cleanEmptyParents` is a no-op for flat global removes — just `os.RemoveAll` the single flat directory.
- **Discovery**: Pinfile always has original composite keys, so `FlatKey()` can be applied at remove time without needing to reverse the transformation.
