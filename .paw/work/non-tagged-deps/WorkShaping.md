# Work Shaping: Non-Tagged Repository Dependencies

## Problem Statement

**Who benefits**: Teams and individuals building AI agent skill packages with `craft` who want to consume skills from third-party repositories that have not published semver git tags.

**What problem is solved**: Currently, `craft` requires all dependency repositories to have strict semver git tags (`vMAJOR.MINOR.PATCH`). The dependency URL format (`host/org/repo@vX.Y.Z`) enforces this at the syntax level, and the resolution pipeline (tag listing, semver comparison, MVS selection) is built entirely around tagged versions. This means useful skill repositories that haven't formalized releases are completely inaccessible — a significant adoption barrier.

**Impact**: Many third-party skill repositories on GitHub are maintained without formal versioning. Their authors publish and iterate on skills without tagging releases. Craft users cannot consume these skills at all, forcing manual copy-paste — the exact anti-pattern craft was designed to eliminate.

## Work Breakdown

### Core Functionality

1. **Extended dependency URL syntax** — Extend the `@`-notation in `depurl` parsing to support three reference types:
   - `host/org/repo@vX.Y.Z` — existing semver tag (unchanged)
   - `host/org/repo@<commit-sha>` — pin to a specific commit SHA
   - `host/org/repo@branch:<name>` — track a named branch (resolved to commit at install time)

2. **Ref-type-aware resolution** — Update `Resolver` to handle non-tagged refs:
   - Commit SHA refs: resolve directly (verify SHA exists via fetcher)
   - Branch refs: resolve branch name to current HEAD commit SHA via fetcher
   - Tag refs: existing behavior (unchanged)

3. **Pinfile ref-type metadata** — Extend `craft.pin.yaml` entries with a `ref_type` field (`tag`, `commit`, `branch`) so the provenance of each resolved dependency is explicit and machine-readable.

4. **Update behavior by ref type**:
   - Tag deps: existing behavior — find latest semver tag via MVS (unchanged)
   - Branch deps: resolve to latest commit on the tracked branch, update pinfile
   - Commit deps: skip silently — a commit pin is a deliberate freeze

5. **Conflict detection** — When MVS encounters the same package identity (`host/org/repo`) referenced with incompatible ref types (e.g., one dep requires `@v1.2.0` and another requires `@branch:main`), raise an error requiring the user to resolve the conflict manually.

### Supporting Features

6. **Warning system** — Non-tagged dependencies operate under "best-effort" guarantees. Warnings should surface at:
   - `craft add`: when adding a non-tagged dependency, warn that guarantees are weaker
   - `craft validate`: flag non-tagged dependencies as validation warnings
   - Warnings use yellow text formatting, non-blocking

7. **`craft add` support** — Extend `craft add` to accept non-tagged refs:
   - Auto-detect ref type from the URL (SHA pattern vs `branch:` prefix vs semver tag)
   - Validate the ref exists before updating `craft.yaml`

8. **Integrity verification** — Non-tagged deps still get SHA-256 integrity digests in the pinfile. The integrity guarantee applies per-install (files match the computed digest) even though the version guarantee is weaker.

## Edge Cases & Expected Handling

| Edge Case | Expected Handling |
|-----------|-------------------|
| Branch name that looks like a SHA (e.g., `deadbeef`) | `branch:` prefix disambiguates; bare hex strings ≥7 chars treated as commit SHAs |
| Branch is deleted after pinning | `craft install` uses the pinned commit SHA (still works); `craft update` fails with clear error |
| Commit SHA doesn't exist in repo | `craft add` / `craft install` fail with "commit not found" error |
| Same package as both `@v1.0.0` and `@branch:main` | Error: "conflicting ref types for package X — resolve manually" |
| Transitive dep is non-tagged | Same rules apply; warnings bubble up to root user |
| Short SHA provided (e.g., 7 chars) | Resolve to full SHA via fetcher; store full SHA in pinfile |
| Non-tagged dep repo later adds tags | No automatic migration; user can manually switch to tagged ref |
| `craft update` with only commit-pinned deps | No-op with clean exit (nothing to update) |

## Rough Architecture

### Component Changes

```
depurl.go          ← Parse @commit-sha and @branch:name syntax; add RefType field
resolver.go        ← Route resolution by RefType; skip MVS for non-tagged refs
fetcher.go         ← Add ResolveBranch(url, branch) method
types.go (resolve) ← Add RefType enum (Tag, Commit, Branch)
types.go (pinfile) ← Add RefType field to PinnedDep
update.go          ← Branch: resolve latest; Commit: skip; Tag: existing
add.go             ← Accept non-tagged refs; auto-detect type; validate existence
validate/runner.go ← Warn on non-tagged deps
```

### Data Flow

```
User: craft add skills github.com/acme/tools@branch:main

1. depurl.Parse("github.com/acme/tools@branch:main")
   → DepURL{Host, Org, Repo, Ref: "main", RefType: Branch}

2. fetcher.ResolveRef(url, "refs/heads/main")
   → commitSHA "abc123..."

3. Manifest updated: dependencies.skills = "github.com/acme/tools@branch:main"

4. Warning printed: "⚠ Non-tagged dependency: github.com/acme/tools@branch:main
   Branch-tracked deps have weaker reproducibility guarantees."

---

User: craft install

5. Resolver sees RefType=Branch
   → fetcher.ResolveRef(url, "refs/heads/main") → commitSHA
   → discoverSkills(url, commitSHA)
   → computeIntegrity(files)

6. Pinfile entry written:
   github.com/acme/tools@branch:main:
     commit: abc123...
     ref_type: branch
     integrity: sha256-...
     skills: [tool-a, tool-b]

---

User: craft update

7. For branch dep: re-resolve branch HEAD → new commitSHA
   → If changed: re-discover, re-compute integrity, update pinfile
   → Print: "Updated github.com/acme/tools@branch:main → def456..."

8. For commit dep: skip silently

9. For tag dep: existing MVS behavior
```

## Critical Analysis

### Value Assessment

- **High value**: Removes the single biggest barrier to consuming third-party skills
- **Low risk**: Tiered guarantee model preserves existing rigor for tagged deps
- **Incremental**: Existing tagged dependency behavior is completely unchanged
- **Adoption enabler**: Third-party skill authors don't need to change their workflow

### Build vs. Modify Tradeoffs

This is a **modify** task — extending existing systems rather than building new ones:
- `depurl` parsing: extend regex + add RefType field
- Resolver: add branching logic by RefType (most complex change)
- Fetcher: already has `ResolveRef` — may just need branch→ref mapping
- Pinfile: add one field
- CLI commands: small extensions to add/update

The resolver changes are the highest-risk area since resolution logic is the core of craft.

## Codebase Fit

### Existing patterns to leverage
- `depurl.go` already parses `@` syntax — extend the regex and struct
- `fetcher.ResolveRef()` already resolves arbitrary git refs — branch support may be minimal
- `pinfile` types already have extensible struct — adding `RefType` field is straightforward
- Auto-discovery already works for repos without `craft.yaml` — no changes needed there
- Warning/error patterns exist in `validate/` and CLI commands

### Reuse opportunities
- The `ResolveRef` fetcher method may already handle branch refs (refs/heads/name)
- Integrity computation is ref-type-agnostic — works on any commit's files
- Skill discovery is ref-type-agnostic — works on any commit's tree

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| Resolver complexity increase | Medium | Clear RefType branching; extensive tests for each type |
| Branch deps cause "works on my machine" issues | Medium | Pinfile still locks to exact commit; warnings communicate the tradeoff |
| Short SHA ambiguity with branch names | Low | `branch:` prefix eliminates ambiguity |
| Breaking change to pinfile format | Medium | `ref_type` field defaults to `tag` for backward compatibility |
| MVS bypass for non-tagged deps | Low | Explicit error on mixed ref-type conflicts |

## Open Questions for Downstream Stages

1. **SHA length validation**: What's the minimum commit SHA length to accept? (7 chars is git's default short SHA, but 12+ is safer for uniqueness)
2. **Branch ref syntax alternatives**: Is `branch:main` the best prefix, or should we consider `ref:main`, `head:main`, or `@main` (no prefix, auto-detect)?
3. **Transitive non-tagged deps**: Should craft warn more aggressively when a transitive dep pulls in a non-tagged dependency the user didn't directly choose?
4. **`craft init` integration**: Should `craft init` offer non-tagged ref options in its interactive wizard?
5. **Default branch inference**: If user provides `github.com/acme/tools` with no ref at all, should craft infer the default branch? Or require an explicit ref?

## Session Notes

### Key Decisions
- **Primary pain point**: Third-party repos without tags — this is the adoption blocker
- **Syntax**: Extend `@` notation with commit SHAs and `branch:` prefix (not a separate YAML field)
- **Tiered guarantees**: Tagged deps keep full rigor; non-tagged get best-effort with warnings
- **Update behavior**: Branch deps resolve to latest commit; commit-pinned deps are frozen (skipped silently)
- **Conflict resolution**: Error on mixed ref-type conflicts for the same package — no auto-resolution magic
- **Warning surfaces**: `craft add`, `craft validate`, and `ref_type` metadata in pinfile
- **Subdirectory/monorepo**: Already works via auto-discovery — not in scope for this work
