package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	installlib "github.com/erdemtuna/craft/internal/install"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/erdemtuna/craft/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var getDryRun bool
var getTarget string

var getCmd = &cobra.Command{
	Use:   "get [alias] <url> [url...]",
	Short: "Install skills globally",
	Long: `Install skills from one or more repositories into your AI agent's skill directory.

Skills are tracked in ~/.craft/craft.yaml and pinned in ~/.craft/craft.pin.yaml.
Use craft list -g, craft update -g, and craft remove -g to manage globally installed skills.

Examples:
  craft get github.com/alice/skills@v1.0.0
  craft get myalias github.com/alice/skills@v1.0.0
  craft get github.com/alice/skills@v1.0.0 github.com/bob/tools@v2.0.0`,
	Args: cobra.MinimumNArgs(1),
	RunE: runGet,
}

func init() {
	getCmd.Flags().BoolVar(&getDryRun, "dry-run", false, "Show what would be resolved and installed without making changes")
	getCmd.Flags().StringVar(&getTarget, "target", "", "Override agent auto-detection with a custom install path")
}

func runGet(cmd *cobra.Command, args []string) error {
	// Parse arguments: optional alias + one or more URLs
	type depEntry struct {
		alias  string
		url    string
		parsed *resolve.DepURL
	}

	var deps []depEntry

	// Detect if first arg is an alias (not a URL)
	firstIsDep := true
	if len(args) >= 2 {
		if _, err := resolve.ParseDepURL(args[0]); err != nil {
			// First arg is not a valid URL — treat as alias for the second arg
			firstIsDep = false
		}
	}

	if !firstIsDep {
		// First arg is alias, rest are URLs
		if len(args) != 2 {
			return fmt.Errorf("when providing an alias, exactly one URL must follow\n  usage: craft get [alias] <url>")
		}
		if err := manifest.ValidateName(args[0]); err != nil {
			return fmt.Errorf("invalid alias %q: %w\n  hint: aliases must be lowercase alphanumeric with hyphens (e.g. 'my-skills')", args[0], err)
		}
		parsed, err := resolve.ParseDepURL(args[1])
		if err != nil {
			return fmt.Errorf("%w\n  hint: expected format: host/org/repo@v1.0.0, host/org/repo@<sha>, or host/org/repo@branch:<name>", err)
		}
		deps = append(deps, depEntry{alias: args[0], url: args[1], parsed: parsed})
	} else {
		// All args are URLs
		for _, arg := range args {
			parsed, err := resolve.ParseDepURL(arg)
			if err != nil {
				return fmt.Errorf("%w\n  hint: expected format: host/org/repo@v1.0.0, host/org/repo@<sha>, or host/org/repo@branch:<name>", err)
			}
			deps = append(deps, depEntry{alias: parsed.Repo, url: arg, parsed: parsed})
		}
	}

	// Load or create global manifest
	manifestPath, err := GlobalManifestPath()
	if err != nil {
		return err
	}
	pfPath, err := GlobalPinfilePath()
	if err != nil {
		return err
	}

	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("reading global craft.yaml: %w", err)
		}
		// Auto-create global manifest
		if err := ensureGlobalDir(); err != nil {
			return err
		}
		m = &manifest.Manifest{
			SchemaVersion: 1,
			Name:          "global",
			Dependencies:  make(map[string]string),
		}
	}

	if m.Dependencies == nil {
		m.Dependencies = make(map[string]string)
	}

	// Check for already-installed deps and prompt if different
	isTTY := term.IsTerminal(int(os.Stdin.Fd()))
	for i, dep := range deps {
		existing, ok := m.Dependencies[dep.alias]
		if !ok {
			continue
		}
		if existing == dep.url {
			cmd.Printf("%q is already installed at %s — skipping.\n", dep.alias, dep.url)
			// Mark as skip by clearing URL
			deps[i].url = ""
			continue
		}
		// Different version — prompt
		if !isTTY {
			return fmt.Errorf("%q is already installed at %s (requested %s)\n  hint: use an interactive terminal to confirm updates, or use craft update -g",
				dep.alias, existing, dep.url)
		}
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%q is currently at %s. Update to %s? [y/N]: ", dep.alias, existing, dep.url)
		scanner := bufio.NewScanner(os.Stdin)
		if !scanner.Scan() || !strings.HasPrefix(strings.ToLower(strings.TrimSpace(scanner.Text())), "y") {
			cmd.Printf("Skipping %q.\n", dep.alias)
			deps[i].url = ""
			continue
		}
	}

	// Filter out skipped deps
	var activeDeps []depEntry
	for _, dep := range deps {
		if dep.url != "" {
			activeDeps = append(activeDeps, dep)
		}
	}

	if len(activeDeps) == 0 {
		cmd.Println("Nothing to install.")
		return nil
	}

	// Check for alias collisions among parsed deps
	seen := make(map[string]string) // alias → url
	for _, dep := range activeDeps {
		if existing, ok := seen[dep.alias]; ok {
			return fmt.Errorf("alias collision: %q resolves to both %s and %s\n  hint: use 'craft get <alias> <url>' to provide distinct aliases",
				dep.alias, existing, dep.url)
		}
		seen[dep.alias] = dep.url
	}

	// Add all deps to manifest
	for _, dep := range activeDeps {
		m.Dependencies[dep.alias] = dep.url
		// Warn about non-tagged dependencies
		if dep.parsed.RefType != resolve.RefTypeTag {
			cmd.PrintErrln("⚠ Non-tagged dependency: " + dep.url)
		}
	}

	// Resolve full tree
	progress := ui.NewProgress()
	fetcher, err := newFetcher()
	if err != nil {
		return err
	}

	var existingPinfile *pinfile.Pinfile
	if pf, err := pinfile.ParseFile(pfPath); err == nil {
		existingPinfile = pf
	}

	forceResolve := make(map[string]bool)
	for _, dep := range activeDeps {
		forceResolve[dep.url] = true
	}

	progress.Start("Resolving dependencies...")
	resolver := resolve.NewResolver(fetcher)
	result, err := resolver.Resolve(m, resolve.ResolveOptions{
		ExistingPinfile: existingPinfile,
		ForceResolve:    forceResolve,
	})
	if err != nil {
		progress.Fail("Resolution failed")
		return fmt.Errorf("resolution failed: %w", err)
	}
	progress.Update(fmt.Sprintf("Resolved %d dependency(ies)", len(result.Resolved)))

	if getDryRun {
		progress.Done("Dry-run complete")
		printDryRunSummary(cmd, result, "+")
		return nil
	}

	// Finalize progress line before agent prompt may write multi-line output to stderr.
	progress.Done(fmt.Sprintf("Resolved %d dependency(ies)", len(result.Resolved)))

	// Resolve agent install targets before writing manifest.
	// This way, if the user cancels the agent prompt, nothing is persisted.
	targetPaths, err := resolveInstallTargets(getTarget)
	if err != nil {
		return err
	}

	// Restart progress for the install phase.
	progress.Start("Writing manifest...")

	// Write manifest and pinfile before install (write-ahead).
	// If install fails, the dep is tracked but not installed — recoverable via `craft install -g`.
	if err := writeManifestAtomic(manifestPath, m); err != nil {
		return err
	}
	if err := writePinfileAtomic(pfPath, result.Pinfile); err != nil {
		return err
	}

	// Collect skill files
	skillFiles, err := collectSkillFiles(fetcher, result)
	if err != nil {
		progress.Fail("Fetching failed")
		return fmt.Errorf("%w\n  note: dependencies were added to the global manifest but installation could not complete\n  hint: run 'craft install -g' to retry, or 'craft remove -g <alias>' to undo", err)
	}

	// Verify integrity
	if err := verifyIntegrity(result, skillFiles); err != nil {
		progress.Fail("Integrity check failed")
		return fmt.Errorf("%w\n  note: dependencies were added to the global manifest but installation could not complete\n  hint: run 'craft install -g' to retry, or 'craft remove -g <alias>' to undo", err)
	}

	// Install to agent directories
	progress.Update("Installing skills...")
	for _, targetPath := range targetPaths {
		if err := installlib.InstallFlat(targetPath, skillFiles); err != nil {
			progress.Fail("Installation failed")
			return fmt.Errorf("installation failed: %w\n  note: dependencies were added to the global manifest but installation could not complete\n  hint: run 'craft install -g' to retry, or 'craft remove -g <alias>' to undo", err)
		}
	}

	skillCount := countSkills(result)
	msg := fmt.Sprintf("Installed %d skill(s) from %d package(s) to %s",
		skillCount, len(result.Resolved), strings.Join(targetPaths, ", "))
	progress.Done(msg)
	if !progress.IsTTY() {
		cmd.Println(msg)
	}

	// Print summary for each added dep
	for _, dep := range activeDeps {
		var skills []string
		for _, r := range result.Resolved {
			if r.Alias == dep.alias {
				skills = r.Skills
				break
			}
		}
		if len(skills) > 0 {
			cmd.Printf("  %s: %s\n", dep.alias, strings.Join(skills, ", "))
		}
	}

	printDependencyTree(cmd, m, result)

	return nil
}
