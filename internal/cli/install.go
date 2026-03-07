package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erdemtuna/craft/internal/agent"
	"github.com/erdemtuna/craft/internal/fetch"
	installlib "github.com/erdemtuna/craft/internal/install"
	"github.com/erdemtuna/craft/internal/integrity"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/erdemtuna/craft/internal/ui"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var installTarget string

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install skill dependencies",
	Long:  "Resolve, pin, and install all dependencies declared in craft.yaml.",
	Args:  cobra.NoArgs,
	RunE:  runInstall,
}

func init() {
	installCmd.Flags().StringVar(&installTarget, "target", "", "Override agent auto-detection with a custom install path")
}

func runInstall(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	progress := ui.NewProgress()

	// Parse manifest
	m, err := manifest.ParseFile(filepath.Join(root, "craft.yaml"))
	if err != nil {
		return fmt.Errorf("reading craft.yaml: %w\n  hint: run `craft init` to create one", err)
	}

	if len(m.Dependencies) == 0 {
		cmd.Println("No dependencies to install.")
		return nil
	}

	// Load existing pinfile (if any)
	var existingPinfile *pinfile.Pinfile
	pfPath := filepath.Join(root, "craft.pin.yaml")
	if pf, err := pinfile.ParseFile(pfPath); err == nil {
		existingPinfile = pf
	}

	// Set up cache and fetcher
	cacheRoot, err := fetch.DefaultCacheRoot()
	if err != nil {
		return err
	}
	cache, err := fetch.NewCache(cacheRoot)
	if err != nil {
		return err
	}
	fetcher := fetch.NewGoGitFetcher(cache)

	// Resolve dependencies
	progress.Start("Resolving dependencies...")
	resolver := resolve.NewResolver(fetcher)
	result, err := resolver.Resolve(m, resolve.ResolveOptions{
		ExistingPinfile: existingPinfile,
	})
	if err != nil {
		progress.Fail("Resolution failed")
		return fmt.Errorf("resolution failed: %w", err)
	}
	progress.Update(fmt.Sprintf("Resolved %d dependency(ies)", len(result.Resolved)))

	// Write pinfile atomically
	if err := writePinfileAtomic(pfPath, result.Pinfile); err != nil {
		return err
	}

	// Determine install path(s)
	targetPaths, err := resolveInstallTargets(installTarget)
	if err != nil {
		return err
	}

	// Collect skill files for installation
	skillFiles, err := collectSkillFiles(fetcher, result)
	if err != nil {
		progress.Fail("Fetching failed")
		return err
	}

	// Verify integrity of collected files against pinfile digests
	if err := verifyIntegrity(result, skillFiles); err != nil {
		progress.Fail("Integrity check failed")
		return err
	}

	// Install to each target path
	progress.Update("Installing skills...")
	for _, targetPath := range targetPaths {
		if err := installlib.Install(targetPath, skillFiles); err != nil {
			progress.Fail("Installation failed")
			return fmt.Errorf("installation failed: %w", err)
		}
	}

	skillCount := countSkills(result)
	progress.Done(fmt.Sprintf("Installed %d skill(s) from %d package(s) to %s",
		skillCount, len(result.Resolved), strings.Join(targetPaths, ", ")))

	// Print dependency tree to stderr
	printDependencyTree(cmd, m, result)

	return nil
}

func writePinfileAtomic(path string, pf *pinfile.Pinfile) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating pinfile: %w", err)
	}

	if err := pinfile.Write(pf, f); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing pinfile: %w", err)
	}

	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("writing pinfile: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("saving pinfile: %w", err)
	}

	return nil
}

// resolveInstallTargets returns one or more install target paths.
// If --target is provided, returns that single path.
// If multiple agents detected on TTY, prompts and may return multiple paths.
func resolveInstallTargets(target string) ([]string, error) {
	if target != "" {
		return []string{target}, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("determining home directory: %w", err)
	}

	// Try single-agent detection first
	result, err := agent.Detect(home)
	if err == nil {
		return []string{result.InstallPath}, nil
	}

	// Check for multi-agent scenario
	agents := agent.DetectAll(home)
	if len(agents) == 0 {
		return nil, fmt.Errorf("no known AI agent detected\n  hint: use --target <path> to specify the installation directory")
	}

	// Multiple agents detected — prompt if stdin is TTY
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		names := make([]string, len(agents))
		for i, a := range agents {
			names[i] = a.Agent.String()
		}
		return nil, fmt.Errorf("multiple AI agents detected (%s)\n  hint: use --target <path> to specify the installation directory",
			strings.Join(names, ", "))
	}

	return promptAgentChoice(agents)
}

func promptAgentChoice(agents []agent.DetectResult) ([]string, error) {
	fmt.Fprintf(os.Stderr, "\nMultiple AI agents detected. Where should skills be installed?\n\n")
	for i, a := range agents {
		fmt.Fprintf(os.Stderr, "  %d) %s (%s)\n", i+1, a.Agent.String(), a.InstallPath)
	}
	fmt.Fprintf(os.Stderr, "  %d) Both\n", len(agents)+1)
	fmt.Fprintf(os.Stderr, "\nChoice [1-%d]: ", len(agents)+1)

	scanner := bufio.NewScanner(os.Stdin)
	if !scanner.Scan() {
		return nil, fmt.Errorf("no input received\n  hint: use --target <path> for non-interactive use")
	}

	choice := strings.TrimSpace(scanner.Text())

	// "Both" selection — return all agent paths
	bothChoice := fmt.Sprintf("%d", len(agents)+1)
	if choice == bothChoice {
		paths := make([]string, len(agents))
		for i, a := range agents {
			paths[i] = a.InstallPath
		}
		return paths, nil
	}

	// Parse numeric choice
	var idx int
	if _, err := fmt.Sscanf(choice, "%d", &idx); err != nil || idx < 1 || idx > len(agents) {
		return nil, fmt.Errorf("invalid choice %q\n  hint: enter a number from 1 to %d", choice, len(agents)+1)
	}

	return []string{agents[idx-1].InstallPath}, nil
}

// printDependencyTree prints a formatted dependency tree to stderr.
func printDependencyTree(cmd *cobra.Command, m *manifest.Manifest, result *resolve.ResolveResult) {
	packageName := m.Name
	if m.Version != "" {
		packageName += "@" + m.Version
	}

	// Extract local skill names from paths
	var localSkills []string
	for _, s := range m.Skills {
		// Use the last path component as the display name
		parts := strings.Split(strings.TrimRight(s, "/"), "/")
		localSkills = append(localSkills, parts[len(parts)-1])
	}

	// Build dep nodes
	var deps []ui.DepNode
	for _, dep := range result.Resolved {
		deps = append(deps, ui.DepNode{
			Alias:  dep.Alias,
			URL:    dep.URL,
			Skills: dep.Skills,
		})
	}

	fmt.Fprintln(cmd.ErrOrStderr())
	ui.RenderTree(cmd.ErrOrStderr(), packageName, localSkills, deps)
}

func collectSkillFiles(fetcher fetch.GitFetcher, result *resolve.ResolveResult) (map[string]map[string][]byte, error) {
	skills := make(map[string]map[string][]byte)

	for _, dep := range result.Resolved {
		parsed, err := resolve.ParseDepURL(dep.URL)
		if err != nil {
			return nil, fmt.Errorf("collecting files for %s: %w", dep.URL, err)
		}

		cloneURL := fetch.NormalizeCloneURL(parsed.PackageIdentity())

		for i, skillName := range dep.Skills {
			var skillDir string
			if i < len(dep.SkillPaths) {
				skillDir = dep.SkillPaths[i]
			}

			// Read all files in skill directory
			allPaths, err := fetcher.ListTree(cloneURL, dep.Commit)
			if err != nil {
				return nil, fmt.Errorf("listing files for %s: %w", skillName, err)
			}

			prefix := skillDir + "/"
			if skillDir == "" {
				prefix = ""
			}
			var filePaths []string
			for _, p := range allPaths {
				if prefix == "" || strings.HasPrefix(p, prefix) {
					// For root-level skills, exclude infrastructure files
					if prefix == "" && resolve.IsInfraFile(p) {
						continue
					}
					filePaths = append(filePaths, p)
				}
			}

			files, err := fetcher.ReadFiles(cloneURL, dep.Commit, filePaths)
			if err != nil {
				return nil, fmt.Errorf("reading files for %s: %w", skillName, err)
			}

			// Remap paths to be relative to skill directory
			mapped := make(map[string][]byte)
			for p, content := range files {
				relPath := p
				if prefix != "" {
					relPath = strings.TrimPrefix(p, prefix)
				}
				mapped[relPath] = content
			}

			skills[skillName] = mapped
		}
	}

	return skills, nil
}

// verifyIntegrity checks that the collected skill files match the integrity
// digests stored in the pinfile. Returns an error if any dependency's files
// produce a different digest than expected (indicating cache corruption).
func verifyIntegrity(result *resolve.ResolveResult, skills map[string]map[string][]byte) error {
	for _, dep := range result.Resolved {
		pinEntry, ok := result.Pinfile.Resolved[dep.URL]
		if !ok || pinEntry.Integrity == "" {
			continue
		}

		// Reconstruct combined file map with original paths (matching resolver)
		combined := make(map[string][]byte)
		for i, skillName := range dep.Skills {
			var prefix string
			if i < len(dep.SkillPaths) && dep.SkillPaths[i] != "" {
				prefix = dep.SkillPaths[i] + "/"
			}

			skillFiles, ok := skills[skillName]
			if !ok {
				continue
			}
			for relPath, content := range skillFiles {
				combined[prefix+relPath] = content
			}
		}

		got := integrity.Digest(combined)
		if got != pinEntry.Integrity {
			return fmt.Errorf("integrity mismatch for %s: expected %s, got %s (cache may be corrupted, try 'craft cache clean')", dep.URL, pinEntry.Integrity, got)
		}
	}
	return nil
}

func countSkills(result *resolve.ResolveResult) int {
	count := 0
	for _, dep := range result.Resolved {
		count += len(dep.Skills)
	}
	return count
}
