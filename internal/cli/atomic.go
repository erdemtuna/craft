package cli

import (
	"fmt"
	"io"
	"os"
)

// writeAtomic writes to a file atomically by writing to a temp file first,
// then renaming it to the target path.
func writeAtomic(path string, writeFn func(w io.Writer) error) error {
	tmpPath := path + ".tmp"
	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	if err := writeFn(f); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
