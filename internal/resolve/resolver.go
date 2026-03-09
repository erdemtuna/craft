package resolve

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/erdemtuna/craft/internal/fetch"
	"github.com/erdemtuna/craft/internal/integrity"
	"github.com/erdemtuna/craft/internal/manifest"
	"github.com/erdemtuna/craft/internal/pinfile"
	"github.com/erdemtuna/craft/internal/semver"
	"github.com/erdemtuna/craft/internal/skill"
)

const (
	maxResolutionDepth = 20
	maxTotalDeps       = 200
)

// Resolver orchestrates dependency resolution using MVS.
type Resolver struct {
	fetcher fetch.GitFetcher
}

// NewResolver creates a Resolver backed by the given GitFetcher.
func NewResolver(fetcher fetch.GitFetcher) *Resolver {
	return &Resolver{fetcher: fetcher}
}

// ResolveOptions configures resolution behavior.
type ResolveOptions struct {
	// ExistingPinfile is the existing pinfile for reuse (may be nil).
	ExistingPinfile *pinfile.Pinfile

	// ForceResolve lists dep URLs that should be re-resolved even if
	// pinned (used by craft update).
	ForceResolve map[string]bool
}

// ResolveResult holds the resolution output.
type ResolveResult struct {
	// Resolved contains all resolved dependencies (direct + transitive).
	Resolved []ResolvedDep

	// Pinfile is the assembled pinfile ready to write.
	Pinfile *pinfile.Pinfile
}

// Resolve resolves all dependencies from the given manifest.
func (r *Resolver) Resolve(m *manifest.Manifest, opts ResolveOptions) (*ResolveResult, error) {
	if len(m.Dependencies) == 0 {
		return &ResolveResult{
			Resolved: []ResolvedDep{},
			Pinfile:  &pinfile.Pinfile{PinVersion: 1, Resolved: map[string]pinfile.ResolvedEntry{}},
		}, nil
	}

	// Phase 1: Collect all dependency requirements (direct + transitive)
	graph := NewGraph()
	rootID := m.Name

	visited := make(map[string]string) // identity → version first visited
	allDeps, err := r.collectDeps(m, rootID, "", graph, opts, visited, 0)
	if err != nil {
		return nil, err
	}

	// Phase 2: Cycle detection
	if cycle := graph.DetectCycles(); cycle != nil {
		return nil, fmt.Errorf("%s", FormatCycle(cycle))
	}

	// Phase 3: MVS — group by package identity, select highest version.
	// After selection, re-collect transitive deps for any package where
	// MVS selected a version different from the one first visited.
	var selected map[string]ResolvedDep
	for {
		byIdentity := make(map[string][]ResolvedDep)
		for _, dep := range allDeps {
			parsed, err := ParseDepURL(dep.URL)
			if err != nil {
				return nil, err
			}
			identity := parsed.PackageIdentity()
			byIdentity[identity] = append(byIdentity[identity], dep)
		}

		selected = make(map[string]ResolvedDep)
		for identity, deps := range byIdentity {
			best := deps[0]
			// Error ignored: URL was validated in collectDeps
			bestParsed, _ := ParseDepURL(best.URL)
			for _, dep := range deps[1:] {
				// Error ignored: URL was validated in collectDeps
				parsed, _ := ParseDepURL(dep.URL)
				if semver.Compare(parsed.Version, bestParsed.Version) > 0 {
					best = dep
					bestParsed = parsed
				}
			}
			// Prefer direct dep metadata (Source == "") when available
			for _, dep := range deps {
				// Error ignored: URL was validated in collectDeps
				depParsed, _ := ParseDepURL(dep.URL)
				if depParsed.Version == bestParsed.Version && dep.Source == "" {
					best.Alias = dep.Alias
					best.Source = dep.Source
					break
				}
			}
			selected[identity] = best
		}

		// Re-collect transitive deps for packages where MVS selected
		// a different version than what was first visited.
		changed := false
		for identity, dep := range selected {
			// Error ignored: URL was validated in collectDeps
			parsed, _ := ParseDepURL(dep.URL)
			visitedVersion, ok := visited[identity]
			if !ok || visitedVersion == parsed.Version {
				continue
			}

			cloneURL := fetch.NormalizeCloneURL(identity)
			commitSHA, err := r.fetcher.ResolveRef(cloneURL, parsed.GitTag())
			if err != nil {
				return nil, fmt.Errorf("resolving %s: %w", dep.URL, err)
			}

			visited[identity] = parsed.Version

			files, err := r.fetcher.ReadFiles(cloneURL, commitSHA, []string{"craft.yaml"})
			if err != nil {
				// No craft.yaml → leaf dependency (no transitive deps)
				continue
			}

			craftYAML, ok := files["craft.yaml"]
			if !ok {
				continue
			}

			depManifest, err := manifest.Parse(bytes.NewReader(craftYAML))
			if err != nil {
				return nil, fmt.Errorf("parsing craft.yaml from %s at %s: %w", dep.URL, parsed.Version, err)
			}

			if len(depManifest.Dependencies) > 0 {
				transitive, err := r.collectDeps(depManifest, identity, dep.URL, graph, opts, visited, 0)
				if err != nil {
					return nil, err
				}
				if len(transitive) > 0 {
					allDeps = append(allDeps, transitive...)
					changed = true
				}
			}
		}

		if !changed {
			break
		}
	}

	// Re-check for cycles after potential graph modifications
	if cycle := graph.DetectCycles(); cycle != nil {
		return nil, fmt.Errorf("%s", FormatCycle(cycle))
	}

	// Phase 4: Resolve commit SHAs, discover skills, compute integrity
	var resolved []ResolvedDep
	for _, dep := range selected {
		full, err := r.resolveOne(dep, opts)
		if err != nil {
			return nil, err
		}
		resolved = append(resolved, full)
	}

	// Sort for determinism
	sort.Slice(resolved, func(i, j int) bool {
		return resolved[i].URL < resolved[j].URL
	})

	// Phase 5: Skill name collision detection
	if err := detectCollisions(resolved); err != nil {
		return nil, err
	}

	// Phase 6: Build pinfile
	pf := &pinfile.Pinfile{
		PinVersion: 1,
		Resolved:   make(map[string]pinfile.ResolvedEntry),
	}
	for _, dep := range resolved {
		pf.Resolved[dep.URL] = pinfile.ResolvedEntry{
			Commit:     dep.Commit,
			Integrity:  dep.Integrity,
			Source:     dep.Source,
			Skills:     dep.Skills,
			SkillPaths: dep.SkillPaths,
		}
	}

	return &ResolveResult{Resolved: resolved, Pinfile: pf}, nil
}

// collectDeps recursively collects all dependency requirements.
func (r *Resolver) collectDeps(m *manifest.Manifest, parentID, source string, graph *Graph, opts ResolveOptions, visited map[string]string, depth int) ([]ResolvedDep, error) {
	if depth > maxResolutionDepth {
		return nil, fmt.Errorf("dependency resolution exceeded maximum depth of %d (possible deep dependency chain)", maxResolutionDepth)
	}

	var allDeps []ResolvedDep

	for alias, depURL := range m.Dependencies {
		parsed, err := ParseDepURL(depURL)
		if err != nil {
			return nil, fmt.Errorf("invalid dependency URL for %q: %w", alias, err)
		}

		identity := parsed.PackageIdentity()
		graph.AddEdge(parentID, identity)

		dep := ResolvedDep{
			URL:    depURL,
			Alias:  alias,
			Source: source,
		}
		allDeps = append(allDeps, dep)

		// Recursively resolve transitive dependencies (avoid re-visiting)
		if _, ok := visited[identity]; ok {
			continue
		}
		visited[identity] = parsed.Version
		if len(visited) > maxTotalDeps {
			return nil, fmt.Errorf("dependency resolution exceeded maximum of %d total dependencies", maxTotalDeps)
		}

		cloneURL := fetch.NormalizeCloneURL(identity)

		commitSHA, err := r.fetcher.ResolveRef(cloneURL, parsed.GitTag())
		if err != nil {
			return nil, fmt.Errorf("resolving %s: %w", depURL, err)
		}

		files, err := r.fetcher.ReadFiles(cloneURL, commitSHA, []string{"craft.yaml"})
		if err != nil {
			// No craft.yaml → leaf dependency (no transitive deps)
			continue
		}

		if craftYAML, ok := files["craft.yaml"]; ok {
			depManifest, err := manifest.Parse(bytes.NewReader(craftYAML))
			if err != nil {
				return nil, fmt.Errorf("parsing craft.yaml from %s: %w", depURL, err)
			}

			if len(depManifest.Dependencies) > 0 {
				transitive, err := r.collectDeps(depManifest, identity, depURL, graph, opts, visited, depth+1)
				if err != nil {
					return nil, err
				}
				allDeps = append(allDeps, transitive...)
			}
		}
	}

	return allDeps, nil
}

// resolveOne resolves a single dependency's commit, skills, and integrity.
func (r *Resolver) resolveOne(dep ResolvedDep, opts ResolveOptions) (ResolvedDep, error) {
	parsed, err := ParseDepURL(dep.URL)
	if err != nil {
		return dep, err
	}

	// Check pinfile reuse
	if opts.ExistingPinfile != nil && !opts.ForceResolve[dep.URL] {
		if entry, ok := opts.ExistingPinfile.Resolved[dep.URL]; ok {
			dep.Commit = entry.Commit
			dep.Integrity = entry.Integrity
			dep.Skills = entry.Skills
			dep.SkillPaths = entry.SkillPaths
			return dep, nil
		}
	}

	cloneURL := fetch.NormalizeCloneURL(parsed.PackageIdentity())

	commitSHA, err := r.fetcher.ResolveRef(cloneURL, parsed.GitTag())
	if err != nil {
		return dep, fmt.Errorf("resolving %s: %w", dep.URL, err)
	}
	dep.Commit = commitSHA

	// Discover skills and compute integrity
	skillNames, skillPaths, skillFiles, err := r.discoverSkillsForDep(cloneURL, commitSHA)
	if err != nil {
		return dep, err
	}

	dep.Skills = skillNames
	dep.SkillPaths = skillPaths
	dep.Integrity = integrity.Digest(skillFiles)

	return dep, nil
}

// discoverSkillsForDep finds skills in a dependency, trying craft.yaml first,
// then falling back to auto-discovery.
func (r *Resolver) discoverSkillsForDep(cloneURL, commitSHA string) (names []string, dirs []string, files map[string][]byte, err error) {
	// Try reading craft.yaml
	craftFiles, readErr := r.fetcher.ReadFiles(cloneURL, commitSHA, []string{"craft.yaml"})
	if readErr == nil {
		if craftYAML, ok := craftFiles["craft.yaml"]; ok {
			depManifest, parseErr := manifest.Parse(bytes.NewReader(craftYAML))
			if parseErr != nil {
				return nil, nil, nil, fmt.Errorf("parsing craft.yaml in %s: %w", cloneURL, parseErr)
			}
			if len(depManifest.Skills) > 0 {
				return r.discoverFromManifestSkills(cloneURL, commitSHA, depManifest.Skills)
			}
		}
	}

	// Auto-discovery fallback
	return r.autoDiscoverSkills(cloneURL, commitSHA)
}

// discoverFromManifestSkills reads skills declared in a dependency's manifest.
func (r *Resolver) discoverFromManifestSkills(cloneURL, commitSHA string, skillDirs []string) ([]string, []string, map[string][]byte, error) {
	var mdPaths []string
	for _, sp := range skillDirs {
		sp = strings.TrimPrefix(sp, "./")
		mdPaths = append(mdPaths, sp+"/SKILL.md")
	}

	mdFiles, err := r.fetcher.ReadFiles(cloneURL, commitSHA, mdPaths)
	if err != nil {
		return nil, nil, nil, err
	}

	// Fetch tree once for all skill directories
	treePaths, listErr := r.fetcher.ListTree(cloneURL, commitSHA)
	if listErr != nil {
		return nil, nil, nil, fmt.Errorf("listing tree for integrity: %w", listErr)
	}

	var names []string
	var dirs []string
	allFiles := make(map[string][]byte)

	for _, sp := range skillDirs {
		sp = strings.TrimPrefix(sp, "./")
		mdPath := sp + "/SKILL.md"
		content, ok := mdFiles[mdPath]
		if !ok {
			continue
		}

		fm, parseErr := skill.ParseFrontmatter(bytes.NewReader(content))
		if parseErr != nil || fm.Name == "" {
			continue
		}

		if manifest.ValidateName(fm.Name) != nil {
			continue
		}

		names = append(names, fm.Name)
		dirs = append(dirs, sp)

		dirFiles, readErr := CollectSkillDirFiles(r.fetcher, cloneURL, commitSHA, treePaths, sp)
		if readErr != nil {
			return nil, nil, nil, fmt.Errorf("reading skill files for integrity: %w", readErr)
		}
		for k, v := range dirFiles {
			allFiles[k] = v
		}
	}

	return names, dirs, allFiles, nil
}

// autoDiscoverSkills finds skills via ListTree + SKILL.md scanning.
func (r *Resolver) autoDiscoverSkills(cloneURL, commitSHA string) ([]string, []string, map[string][]byte, error) {
	allPaths, err := r.fetcher.ListTree(cloneURL, commitSHA)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("listing tree: %w", err)
	}

	var mdPaths []string
	for _, p := range allPaths {
		if strings.HasSuffix(p, "/SKILL.md") || p == "SKILL.md" {
			mdPaths = append(mdPaths, p)
		}
	}

	if len(mdPaths) == 0 {
		return nil, nil, nil, nil
	}

	mdFiles, err := r.fetcher.ReadFiles(cloneURL, commitSHA, mdPaths)
	if err != nil {
		return nil, nil, nil, err
	}

	var names []string
	var dirs []string
	allFiles := make(map[string][]byte)

	for _, mdPath := range mdPaths {
		content, ok := mdFiles[mdPath]
		if !ok {
			continue
		}

		fm, parseErr := skill.ParseFrontmatter(bytes.NewReader(content))
		if parseErr != nil || fm.Name == "" {
			continue
		}

		if manifest.ValidateName(fm.Name) != nil {
			continue
		}

		skillDir := strings.TrimSuffix(mdPath, "/SKILL.md")
		if skillDir == "SKILL.md" {
			skillDir = ""
		}

		names = append(names, fm.Name)
		dirs = append(dirs, skillDir)

		dirFiles, readErr := CollectSkillDirFiles(r.fetcher, cloneURL, commitSHA, allPaths, skillDir)
		if readErr != nil {
			return nil, nil, nil, fmt.Errorf("reading skill files for integrity: %w", readErr)
		}
		for k, v := range dirFiles {
			allFiles[k] = v
		}
	}

	return names, dirs, allFiles, nil
}

// detectCollisions checks for duplicate skill names across resolved deps.
func detectCollisions(resolved []ResolvedDep) error {
	type skillSource struct {
		depURL string
		commit string
	}

	seen := make(map[string]skillSource)
	for _, dep := range resolved {
		for _, name := range dep.Skills {
			if existing, ok := seen[name]; ok {
				existShort := existing.commit
				if len(existShort) > 8 {
					existShort = existShort[:8]
				}
				depShort := dep.Commit
				if len(depShort) > 8 {
					depShort = depShort[:8]
				}
				return fmt.Errorf("skill name collision: %q is exported by both %s (commit %s) and %s (commit %s)",
					name, existing.depURL, existShort, dep.URL, depShort)
			}
			seen[name] = skillSource{depURL: dep.URL, commit: dep.Commit}
		}
	}
	return nil
}

// CollectSkillDirFiles filters allPaths by skillDir, excludes infra files for
// root-level skills, and reads the matching files via fetcher. Returned paths
// are original (not stripped). Callers that need relative paths should strip
// the prefix themselves.
func CollectSkillDirFiles(fetcher fetch.GitFetcher, cloneURL, commitSHA string, allPaths []string, skillDir string) (map[string][]byte, error) {
	prefix := skillDir + "/"
	if skillDir == "" {
		prefix = ""
	}

	var filePaths []string
	for _, p := range allPaths {
		if prefix == "" || strings.HasPrefix(p, prefix) {
			if prefix == "" && IsInfraFile(p) {
				continue
			}
			filePaths = append(filePaths, p)
		}
	}

	return fetcher.ReadFiles(cloneURL, commitSHA, filePaths)
}

// IsInfraFile returns true for common infrastructure files that should not
// be included in root-level skill file collection. Subdirectory skills
// are not affected — this only applies when SKILL.md is at the repo root.
var infraFiles = map[string]bool{
	"license": true, "licence": true,
	"license.md": true, "licence.md": true,
	"license.txt": true, "licence.txt": true,
	"readme.md": true, "readme.txt": true, "readme": true,
	"changelog.md": true, "changelog.txt": true, "changelog": true,
	"contributing.md": true, "contributing.txt": true,
	"code_of_conduct.md": true,
	"craft.yaml":         true,
	".gitignore":         true, ".gitattributes": true,
}

func IsInfraFile(path string) bool {
	// Exclude common infrastructure files at any level
	base := strings.ToLower(filepath.Base(path))
	if infraFiles[base] {
		return true
	}

	// Exclude common infrastructure directories
	dir := strings.ToLower(path)
	infraDirs := []string{".github/", ".gitlab/", ".circleci/", ".vscode/", ".idea/"}
	for _, prefix := range infraDirs {
		if strings.HasPrefix(dir, prefix) {
			return true
		}
	}

	return false
}
