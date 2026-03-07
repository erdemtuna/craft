# CodeResearch: craft-polish

## Codebase Snapshot

**Total Go code:** ~7,500 lines across 13 internal packages, 28 test files.
**External deps:** go-git v5.17.0, cobra v1.10.2, yaml.v3 v3.0.1.
**Transitive:** golang.org/x/term v0.37.0 available (via go-git's SSH deps).

## Key Integration Points

### CLI Command Registration
- **File:** `internal/cli/root.go:18-25`
- Pattern: commands registered in `init()` via `rootCmd.AddCommand()`
- New `addCmd` and `removeCmd` follow this pattern

### Manifest Write (atomic)
- **File:** `internal/cli/update.go:159-183` — `writeManifestAtomic()`
- Pattern: write to `.tmp`, close, rename. Reusable for `craft add` and `craft remove`.

### Pinfile Write (atomic)
- **File:** `internal/cli/install.go:104-128` — `writePinfileAtomic()`
- Same temp+rename pattern. Already shared between install and update.

### Dependency Resolution
- **File:** `internal/resolve/resolver.go:48-199` — `Resolve()`
- `craft add` can use `Resolve()` for validation (resolve single dep to verify it exists)
- `ResolveOptions.ForceResolve` forces re-resolution of specific deps

### Agent Detection
- **File:** `internal/agent/detect.go:48-88` — `Detect()`
- Currently errors on multi-agent detection (line 85)
- Must be changed to return all detected agents for interactive prompt

### Install Target Resolution
- **File:** `internal/cli/install.go:130-146` — `resolveInstallTarget()`
- Calls `agent.Detect()` — needs modification for multi-agent flow

### Skill File Collection
- **File:** `internal/cli/install.go:148-206` — `collectSkillFiles()`
- Used by both install and update. Needed by `craft remove` to identify orphaned files.

### Error Display Pattern
- **File:** `internal/cli/validate.go:33-38`
- Pattern: `error: <msg>\n  fix: <suggestion>` on stderr
- `validate.Error` struct has `.Suggestion` field

### Output Pattern
- All commands use `cmd.Println()` (stdout) and `cmd.PrintErrf()` (stderr)
- No progress indicators, no colors, no tree display currently

## TTY Detection

`golang.org/x/term` available as transitive dep. Need to add as direct dep:
```go
import "golang.org/x/term"
isTerminal := term.IsTerminal(int(os.Stderr.Fd()))
```

## Existing Test Patterns

- **MockFetcher:** `internal/fetch/mock.go` — maps for Refs, Tags, Trees, Files
- **Temp dirs:** Tests use `t.TempDir()` for isolation
- **Cobra testing:** `rootCmd.SetArgs([]string{...})` + capture output
- **No table-driven tests:** individual test functions throughout

## Files to Create

| File | Purpose |
|------|---------|
| `internal/cli/add.go` | `craft add` command |
| `internal/cli/add_test.go` | Tests for add command |
| `internal/cli/remove.go` | `craft remove` command |
| `internal/cli/remove_test.go` | Tests for remove command |
| `internal/ui/progress.go` | TTY-aware progress/spinner |
| `internal/ui/tree.go` | Dependency tree renderer |
| `internal/ui/progress_test.go` | Progress tests |
| `internal/ui/tree_test.go` | Tree renderer tests |
| `README.md` | Project documentation (expand existing 166-line README, not replace) |

## Files to Modify

| File | Changes |
|------|---------|
| `internal/cli/root.go` | Register addCmd, removeCmd |
| `internal/cli/install.go` | Add progress output + tree display |
| `internal/cli/update.go` | Add progress output + tree display |
| `internal/agent/detect.go` | Return all detected agents (multi-agent support) |
| `internal/agent/detect_test.go` | Test multi-agent scenarios |
| `go.mod` | Add golang.org/x/term as direct dep |
