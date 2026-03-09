# WorkShaping: Namespace Skills by Host/Owner/Repo

## Problem Statement

When two direct dependencies export a skill with the same name (e.g., `skill-creator` from both `lossyrob/phased-agent-workflow` and `anthropics/skills`), craft hard-errors with a collision. This is the real-world scenario from the E2E test: PAW exports `skill-creator` from its `.github/` folder, and Anthropic exports it as a primary skill.

The collision exists because skills install **flat** into `<target>/<skill-name>/`, so two `skill-creator` directories would overwrite each other. The fix is to **namespace skills by `host/owner/repo`** parsed from the dependency URL.

## Desired Outcome

Dependency skills install under `<target>/<host>/<owner>/<repo>/<skill-name>/`:

```
<target>/
├── github.com/
│   ├── lossyrob/
│   │   └── phased-agent-workflow/
│   │       ├── paw-implement/
│   │       ├── paw-spec/
│   │       └── skill-creator/         ← from PAW — no conflict
│   └── anthropics/
│       └── skills/
│           ├── pdf/
│           └── skill-creator/         ← from Anthropic — no conflict
└── gitlab.com/                        ← non-GitHub hosts work too
    └── corp/
        └── internal-skills/
            └── deploy/
```

Local skills (declared in `craft.yaml`'s `skills:` field) are **not installed** by craft — they stay in-repo, version-controlled, consumed from their original location. This change only affects dependency skills.

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Namespace key | `host/owner/repo` from dependency URL | Globally unique across all git hosts, deterministic, stable across alias renames, works for transitives |
| Local skills | Not affected (not installed) | `collectSkillFiles()` only processes `result.Resolved` — local skills stay in-repo |
| Transitive deps | Naturally handled | Every dep has a URL with `owner/repo`, so transitives get namespaced too |
| Cross-dep collision detection | **Remove entirely** (`detectCollisions`) | Namespacing makes cross-dep collisions impossible |
| Local collision check | **Keep** in validator | Same package shouldn't export duplicate skill names |
| Migration | Not needed | No existing craft users to migrate |
| `Install()` signature | **Unchanged** | Use composite key `"owner/repo/skillName"` in existing `map[string]map[string][]byte` — `filepath.Join` naturally creates nested dirs |

## Scope

### In Scope
- Change install layout: dependency skills go under `<target>/<host>/<owner>/<repo>/<skill-name>/`
- Namespace derived from parsed dependency URL (`host/owner/repo`), not user alias
- Remove `detectCollisions()` from resolver
- Update `collectSkillFiles` to prefix keys with `host/owner/repo/`
- Update `craft remove` cleanup for new paths; clean empty host/owner/repo dirs
- Update all affected tests
- Update `E2E_REAL_WORLD_TEST.md` expected paths

### Out of Scope
- Local skill installation (they aren't installed today, no change needed)
- Backwards-compatible migration of old flat layout (no existing users)
- Changes to pinfile structure (already tracks per-dep URL)

## Codebase Context

### Current Install Flow
1. **Resolver** (`internal/resolve/resolver.go`): Resolves deps → `[]ResolvedDep` (each has `URL`, `Alias`, `Skills[]`, `SkillPaths[]`)
2. **Resolver phase 5**: `detectCollisions()` — hard-errors if any skill name appears in 2+ deps
3. **CLI install** (`internal/cli/install.go`): `collectSkillFiles()` fetches skill file contents, returns `map[skillName]map[filePath][]byte` — **flat, no namespace**
4. **Installer** (`internal/install/installer.go`): `Install(target, skills)` writes to `<target>/<skillName>/<files>`

### Implementation Approach: Composite Key
Instead of changing `Install()`'s signature, use `"host/owner/repo/skillName"` as the map key:

```go
// In collectSkillFiles — change the key:
parsed, _ := resolve.ParseDepURL(dep.URL)  // DepURL has Host, Org, Repo fields
prefix := parsed.Host + "/" + parsed.Org + "/" + parsed.Repo
skills[prefix + "/" + skillName] = files
// e.g. "github.com/lossyrob/phased-agent-workflow/paw-implement"
// e.g. "github.com/anthropics/skills/skill-creator"
```

`Install()` calls `filepath.Join(target, skillName)` which naturally creates `target/github.com/lossyrob/phased-agent-workflow/paw-implement`. The path traversal check still passes — the path stays within target. The `.staging` directory becomes `target/github.com/lossyrob/phased-agent-workflow/paw-implement.staging` — still correct.

### Key Function Signatures (unchanged)
- `Install(target string, skills map[string]map[string][]byte) error` — **no change needed**
- `collectSkillFiles(fetcher, result) map[string]map[string][]byte` — prefix keys with `host/owner/repo/`
- `detectCollisions(resolved []ResolvedDep) error` — **to be removed**
- `DepURL` struct — already has `Host`, `Org`, `Repo` fields parsed from URL

### Files to Change
1. `internal/cli/install.go` — `collectSkillFiles` prefixes map keys with `host/owner/repo/` parsed from `dep.URL`
2. `internal/cli/update.go` — Same install path changes (shares `collectSkillFiles`)
3. `internal/cli/remove.go` — Use `host/owner/repo/skillName` in cleanup paths; clean empty parent dirs
4. `internal/resolve/resolver.go` — Remove `detectCollisions()` function and its call in `Resolve()`
5. `internal/install/installer.go` — **No changes needed** (composite key works naturally)
6. `internal/cli/add.go` — No changes needed (collision check was inside `Resolve()`)
7. `internal/validate/runner.go` — No changes needed (local collision check is separate)
8. Tests for install, resolve, remove
9. `E2E_REAL_WORLD_TEST.md` — Update expected install paths

## Open Questions

_None — all design decisions resolved during shaping._

## Future Enhancement

- **Alias symlinks** for shorter skill-to-skill references — see [issue #27](https://github.com/erdemtuna/craft/issues/27)

## References

- E2E test scenario: `E2E_REAL_WORLD_TEST.md` (PAW + Anthropic skills, both export `skill-creator`)
- Current collision error: `internal/resolve/resolver.go` lines 509-534
- Install layout: `internal/install/installer.go` `Install()` function
- URL parsing: `internal/resolve/depurl.go` — `DepURL` struct has `Host`, `Org`, `Repo` fields
