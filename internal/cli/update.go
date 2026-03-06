package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/erdemtuna/craft/internal/fetch"
	installlib "github.com/erdemtuna/craft/internal/install"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/spf13/cobra"
)

var updateTarget string

var updateCmd = &cobra.Command{
	Use:   "update [alias]",
	Short: "Update dependencies to latest versions",
	Long:  "Re-resolve dependencies to latest available semver tags. Updates craft.yaml and craft.pin.yaml.",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runUpdate,
}

func init() {
	updateCmd.Flags().StringVar(&updateTarget, "target", "", "Override agent auto-detection with a custom install path")
}

func runUpdate(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	manifestPath := filepath.Join(root, "craft.yaml")
	m, err := manifest.ParseFile(manifestPath)
	if err != nil {
		return fmt.Errorf("reading craft.yaml: %w", err)
	}

	if len(m.Dependencies) == 0 {
		cmd.Println("No dependencies to update.")
		return nil
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

	// Determine which deps to update
	var targetAlias string
	if len(args) > 0 {
		targetAlias = args[0]
		if _, ok := m.Dependencies[targetAlias]; !ok {
			return fmt.Errorf("dependency %q not found in craft.yaml", targetAlias)
		}
	}

	// Find latest versions for targeted deps
	updated := false
	for alias, depURL := range m.Dependencies {
		if targetAlias != "" && alias != targetAlias {
			continue
		}

		parsed, err := resolve.ParseDepURL(depURL)
		if err != nil {
			return fmt.Errorf("invalid dependency URL for %q: %w", alias, err)
		}

		cloneURL := fetch.NormalizeCloneURL(parsed.PackageIdentity())

		tags, err := fetcher.ListTags(cloneURL)
		if err != nil {
			return fmt.Errorf("listing tags for %s: %w", depURL, err)
		}

		latest := findLatestSemverTag(tags)
		if latest == "" {
			cmd.PrintErrf("warning: no semver tags found for %s\n", parsed.PackageIdentity())
			continue
		}

		if latest != parsed.GitTag() {
			newURL := parsed.WithVersion(latest)
			m.Dependencies[alias] = newURL
			updated = true
			cmd.Printf("  %s: %s → %s\n", alias, parsed.GitTag(), latest)
		}
	}

	if !updated {
		cmd.Println("All dependencies are up to date.")
		return nil
	}

	// Write updated manifest atomically
	if err := writeManifestAtomic(manifestPath, m); err != nil {
		return err
	}

	// Now run install with the updated manifest (force re-resolve)
	forceResolve := make(map[string]bool)
	for alias, depURL := range m.Dependencies {
		if targetAlias == "" || alias == targetAlias {
			forceResolve[depURL] = true
		}
	}

	// Load existing pinfile
	var existingPinfile *pinfile.Pinfile
	pfPath := filepath.Join(root, "craft.pin.yaml")
	if pf, err := pinfile.ParseFile(pfPath); err == nil {
		existingPinfile = pf
	}

	resolver := resolve.NewResolver(fetcher)
	result, err := resolver.Resolve(m, resolve.ResolveOptions{
		ExistingPinfile: existingPinfile,
		ForceResolve:    forceResolve,
	})
	if err != nil {
		return fmt.Errorf("resolution failed: %w", err)
	}

	if err := writePinfileAtomic(pfPath, result.Pinfile); err != nil {
		return err
	}

	// Install
	targetPath, err := resolveInstallTarget(updateTarget)
	if err != nil {
		return err
	}

	skillFiles, err := collectSkillFiles(fetcher, result)
	if err != nil {
		return err
	}

	if err := installlib.Install(targetPath, skillFiles); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	cmd.Printf("Updated and installed %d skill(s) to %s\n", countSkills(result), targetPath)

	return nil
}

func writeManifestAtomic(path string, m *manifest.Manifest) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating manifest: %w", err)
	}

	if err := manifest.Write(m, f); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing manifest: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing manifest: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("saving manifest: %w", err)
	}

	return nil
}

func findLatestSemverTag(tags []string) string {
	var latest string
	var latestParts [3]int

	for _, tag := range tags {
		if len(tag) < 2 || tag[0] != 'v' {
			continue
		}
		version := tag[1:]
		var parts [3]int
		n, _ := fmt.Sscanf(version, "%d.%d.%d", &parts[0], &parts[1], &parts[2])
		if n != 3 {
			continue
		}

		if latest == "" || compareParts(parts, latestParts) > 0 {
			latest = tag
			latestParts = parts
		}
	}

	return latest
}

func compareParts(a, b [3]int) int {
	for i := 0; i < 3; i++ {
		if a[i] > b[i] {
			return 1
		}
		if a[i] < b[i] {
			return -1
		}
	}
	return 0
}
