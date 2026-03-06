package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/erdemtuna/craft/internal/agent"
	"github.com/erdemtuna/craft/internal/fetch"
	installlib "github.com/erdemtuna/craft/internal/install"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/resolve"
	"github.com/spf13/cobra"
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

	// Parse manifest
	m, err := manifest.ParseFile(filepath.Join(root, "craft.yaml"))
	if err != nil {
		return fmt.Errorf("reading craft.yaml: %w", err)
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
	resolver := resolve.NewResolver(fetcher)
	result, err := resolver.Resolve(m, resolve.ResolveOptions{
		ExistingPinfile: existingPinfile,
	})
	if err != nil {
		return fmt.Errorf("resolution failed: %w", err)
	}

	// Write pinfile atomically
	if err := writePinfileAtomic(pfPath, result.Pinfile); err != nil {
		return err
	}

	// Determine install path
	targetPath, err := resolveInstallTarget(installTarget)
	if err != nil {
		return err
	}

	// Collect skill files for installation
	skillFiles, err := collectSkillFiles(fetcher, result)
	if err != nil {
		return err
	}

	// Install
	if err := installlib.Install(targetPath, skillFiles); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	cmd.Printf("Resolved %d dependency(ies), installed %d skill(s) to %s\n",
		len(result.Resolved), countSkills(result), targetPath)

	return nil
}

func writePinfileAtomic(path string, pf *pinfile.Pinfile) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating pinfile: %w", err)
	}

	if err := pinfile.Write(pf, f); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("writing pinfile: %w", err)
	}

	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("writing pinfile: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("saving pinfile: %w", err)
	}

	return nil
}

func resolveInstallTarget(target string) (string, error) {
	if target != "" {
		return target, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("determining home directory: %w", err)
	}

	result, err := agent.Detect(home)
	if err != nil {
		return "", err
	}

	return result.InstallPath, nil
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

func countSkills(result *resolve.ResolveResult) int {
	count := 0
	for _, dep := range result.Resolved {
		count += len(dep.Skills)
	}
	return count
}
