# Contributing to craft

Thank you for your interest in contributing to craft! This guide covers everything you need to get started.

## Development Setup

### Prerequisites

| Tool | Install |
|------|---------|
| [Go 1.26+](https://go.dev/dl/) | Required |
| [Task](https://taskfile.dev/) | `go install github.com/go-task/task/v3/cmd/task@latest` |
| gcc | `sudo apt-get install gcc` (needed for `-race` tests) |

### First-Time Setup

```bash
git clone https://github.com/erdemtuna/craft.git
cd craft
task tools:install    # install golangci-lint and govulncheck
task build            # verify everything compiles
task test             # verify tests pass
task hooks:install    # enable git hooks (recommended)
```

## Code Quality

### Formatting

All Go code must be formatted with `gofmt`. This is enforced in CI and by the pre-commit hook.

```bash
task fmt        # auto-format all Go files
task fmt:check  # check without modifying (CI uses this)
```

### Linting

We use [golangci-lint](https://golangci-lint.run/) with a curated set of linters configured in `.golangci-lint.yml`:

- **errcheck** — unchecked errors
- **gosimple** — code simplification suggestions
- **govet** — suspicious constructs (same as `go vet`)
- **ineffassign** — ineffectual assignments
- **staticcheck** — advanced static analysis
- **unused** — unused code detection
- **gocritic** — opinionated style and performance checks
- **gofmt** — formatting enforcement
- **misspell** — typos in comments and strings
- **prealloc** — slice pre-allocation suggestions
- **unconvert** — unnecessary type conversions

```bash
task lint  # run all linters
```

### Security Scanning

[govulncheck](https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck) checks dependencies for known vulnerabilities:

```bash
task vuln  # scan for known vulnerabilities
```

### Static Analysis

```bash
task vet  # run go vet
```

## Task Reference

All development tasks are managed with [Task](https://taskfile.dev/) via `Taskfile.yml`:

| Command | Description |
|---------|-------------|
| `task` | List all available tasks |
| `task build` | Build the binary (with version injection via ldflags) |
| `task test` | Run tests with race detector |
| `task test:coverage` | Run tests with coverage report (HTML output) |
| `task fmt` | Format Go source files |
| `task fmt:check` | Check formatting (fails if files need formatting) |
| `task vet` | Run `go vet` |
| `task lint` | Run golangci-lint |
| `task vuln` | Run govulncheck for known vulnerabilities |
| `task ci` | Run full CI pipeline locally |
| `task install` | Install binary to `$GOPATH/bin` |
| `task hooks:install` | Install git hooks |
| `task hooks:uninstall` | Remove git hooks |
| `task clean` | Remove build artifacts |

## Git Hooks

Git hooks catch issues locally before they reach CI.

### Install

```bash
task hooks:install    # enable hooks
task hooks:uninstall  # disable hooks
```

### What They Check

| Hook | Checks | Speed |
|------|--------|-------|
| **pre-commit** | `gofmt` formatting, `go vet` | ~1-2 seconds |
| **pre-push** | `golangci-lint`, `go test` | ~10-30 seconds |

### Skipping Hooks

When you need to bypass hooks (e.g., WIP commits):

```bash
git commit --no-verify -m "wip: work in progress"
git push --no-verify
```

## Testing

### Running Tests

```bash
task test             # run all tests
task test:coverage    # run with coverage report
```

Coverage output:
- `coverage.out` — raw coverage data
- `coverage.html` — visual HTML report (open in browser)

### Test Patterns

The codebase uses standard Go testing conventions:

- **Table-driven tests** — most packages use `[]struct{ name, input, expected }` patterns
- **Test fixtures** — static test data lives in `testdata/` (manifests, packages, pinfiles, skills)
- **Mock implementations** — `internal/fetch/mock.go` provides a mock fetcher for unit tests
- **Package-level tests** — each package in `internal/` has its own `*_test.go` files

### Writing Tests

- Place test files next to the code they test (`foo.go` → `foo_test.go`)
- Use `testdata/` for fixture files — Go tooling automatically excludes this directory from builds
- Aim for both positive and negative test cases (valid input and error paths)

## CI/CD Pipeline

### Continuous Integration (`ci.yml`)

Runs on every push to `main` and on pull requests. Three parallel jobs:

| Job | What It Does |
|-----|-------------|
| **Quality Checks** | Format check (`gofmt`) → `go vet` → golangci-lint → govulncheck |
| **Tests** | `go test -race` with coverage → uploads `coverage.out` as artifact |
| **Build Binaries** | Cross-compiles for 6 platforms → uploads binaries as artifacts |

**Build matrix:**

| OS | Architectures |
|----|--------------|
| Linux | amd64, arm64 |
| macOS | amd64, arm64 |
| Windows | amd64, arm64 |

Binary artifacts are available for download from the Actions tab on every successful CI run.

### Releases (`release.yml`)

Triggered by pushing a version tag. Uses [GoReleaser](https://goreleaser.com/) to:

1. Build binaries for all platforms (same 6 combos as CI)
2. Generate checksums (SHA-256)
3. Create a GitHub Release with auto-generated release notes
4. Upload archives (`.tar.gz` for Linux/macOS, `.zip` for Windows)

**To create a release:**

```bash
git tag v1.2.3
git push origin v1.2.3
```

The release workflow handles everything else automatically.

## Pull Request Guidelines

1. **Branch from `main`** — create a descriptive branch name
2. **Run CI locally first** — `task ci` runs the same checks as GitHub Actions
3. **All CI jobs must pass** — quality checks, tests, and build
4. **Keep commits focused** — one logical change per commit
5. **Write descriptive commit messages** — explain *why*, not just *what*
