package cli

import (
	"fmt"
	"os"

	"github.com/erdemtuna/craft/internal/fetch"
	"github.com/spf13/cobra"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage the local dependency cache",
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all cached repositories",
	Long:  "Remove all cached git repositories from ~/.craft/cache/. Dependencies will be re-downloaded on next install.",
	Args:  cobra.NoArgs,
	RunE:  runCacheClean,
}

func init() {
	cacheCmd.AddCommand(cacheCleanCmd)
}

func runCacheClean(cmd *cobra.Command, args []string) error {
	cacheRoot, err := fetch.DefaultCacheRoot()
	if err != nil {
		return err
	}

	// Check if cache exists
	info, err := os.Stat(cacheRoot)
	if os.IsNotExist(err) {
		cmd.Println("Cache is already empty.")
		return nil
	}
	if err != nil {
		return fmt.Errorf("checking cache: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("cache path is not a directory: %s", cacheRoot)
	}

	// Count entries before removing
	entries, err := os.ReadDir(cacheRoot)
	if err != nil {
		return fmt.Errorf("reading cache: %w", err)
	}

	// Filter out non-directory entries (like .lock files)
	repoCount := 0
	for _, e := range entries {
		if e.IsDir() {
			repoCount++
		}
	}

	if repoCount == 0 {
		cmd.Println("Cache is already empty.")
		return nil
	}

	// Remove and recreate cache directory
	if err := os.RemoveAll(cacheRoot); err != nil {
		return fmt.Errorf("cleaning cache: %w", err)
	}

	cmd.Printf("Removed %d cached repository(ies) from %s\n", repoCount, cacheRoot)
	return nil
}
