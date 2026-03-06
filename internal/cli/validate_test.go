package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidateNoManifest(t *testing.T) {
	t.Chdir(t.TempDir())

	buf := new(bytes.Buffer)
	rootCmd.SetOut(buf)
	rootCmd.SetArgs([]string{"validate"})

	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("validate in empty directory should return error")
	}
	if !strings.Contains(err.Error(), "validation failed") {
		t.Errorf("error should mention validation failure, got: %v", err)
	}
}
