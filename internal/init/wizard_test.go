package initcmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWizardBasicFlow(t *testing.T) {
	root := t.TempDir()

	// Create a skill directory
	createSkillDir(t, root, "skills/my-skill")

	// Simulate user input: accept all defaults (empty lines)
	input := "\n\n\n"
	in := strings.NewReader(input)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	wizard := NewWizard(root, in, out, errOut)
	err := wizard.Run()
	if err != nil {
		t.Fatalf("Wizard failed: %v", err)
	}

	// Verify craft.yaml was created
	manifestPath := filepath.Join(root, "craft.yaml")
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		t.Fatal("craft.yaml was not created")
	}

	// Read and check the output
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("Reading craft.yaml: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "schema_version: 1") {
		t.Error("craft.yaml should contain schema_version: 1")
	}
	if strings.Contains(content, "\nversion:") {
		t.Error("craft.yaml should not contain a version field")
	}
	if !strings.Contains(content, "skills/my-skill") {
		t.Error("craft.yaml should contain discovered skill path")
	}
}

func TestWizardCustomValues(t *testing.T) {
	root := t.TempDir()

	// Provide custom values (name, description, license)
	input := "my-custom-pkg\nMy awesome package\nMIT\n"
	in := strings.NewReader(input)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	wizard := NewWizard(root, in, out, errOut)
	err := wizard.Run()
	if err != nil {
		t.Fatalf("Wizard failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(root, "craft.yaml"))
	if err != nil {
		t.Fatalf("Reading craft.yaml: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "name: my-custom-pkg") {
		t.Error("craft.yaml should contain custom name")
	}
	if !strings.Contains(content, "description: My awesome package") {
		t.Error("craft.yaml should contain custom description")
	}
	if !strings.Contains(content, "license: MIT") {
		t.Error("craft.yaml should contain custom license")
	}
}

func TestWizardInvalidNameRetry(t *testing.T) {
	root := t.TempDir()

	// First name invalid (uppercase), second valid
	input := "INVALID\nvalid-name\n\n\n"
	in := strings.NewReader(input)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	wizard := NewWizard(root, in, out, errOut)
	err := wizard.Run()
	if err != nil {
		t.Fatalf("Wizard failed: %v", err)
	}

	// Should have printed validation error
	if !strings.Contains(errOut.String(), "invalid") {
		t.Error("Should print validation error for invalid name")
	}

	data, err := os.ReadFile(filepath.Join(root, "craft.yaml"))
	if err != nil {
		t.Fatalf("Reading craft.yaml: %v", err)
	}
	if !strings.Contains(string(data), "name: valid-name") {
		t.Error("craft.yaml should contain the valid name")
	}
}

func TestWizardOverwriteDecline(t *testing.T) {
	root := t.TempDir()

	// Create existing craft.yaml
	existing := filepath.Join(root, "craft.yaml")
	if err := os.WriteFile(existing, []byte("existing content"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Decline overwrite
	input := "n\n"
	in := strings.NewReader(input)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	wizard := NewWizard(root, in, out, errOut)
	err := wizard.Run()
	if err != nil {
		t.Fatalf("Wizard failed: %v", err)
	}

	// Original content should be preserved
	data, _ := os.ReadFile(existing)
	if string(data) != "existing content" {
		t.Error("Original craft.yaml should be preserved")
	}

	if !strings.Contains(out.String(), "Aborted") {
		t.Error("Should print abort message")
	}
}

func TestWizardOverwriteAccept(t *testing.T) {
	root := t.TempDir()

	// Create existing craft.yaml
	if err := os.WriteFile(filepath.Join(root, "craft.yaml"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Accept overwrite, then provide values
	input := "y\nnew-pkg\n\n\n"
	in := strings.NewReader(input)
	out := new(bytes.Buffer)
	errOut := new(bytes.Buffer)

	wizard := NewWizard(root, in, out, errOut)
	err := wizard.Run()
	if err != nil {
		t.Fatalf("Wizard failed: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(root, "craft.yaml"))
	if !strings.Contains(string(data), "name: new-pkg") {
		t.Error("craft.yaml should be overwritten with new content")
	}
}

func TestInferPackageName(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/my-project", "my-project"},
		{"/home/user/MyProject", "myproject"},
		{"/home/user/My Cool Project", "my-cool-project"},
		{"/home/user/123-project", "pkg-123-project"},
		{"/home/user/craft", "craft"},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			got := inferPackageName(tc.path)
			if got != tc.want {
				t.Errorf("inferPackageName(%q) = %q, want %q", tc.path, got, tc.want)
			}
		})
	}
}

func TestValidateName(t *testing.T) {
	if err := validateName("valid-name"); err != nil {
		t.Errorf("Valid name rejected: %v", err)
	}
	if err := validateName("INVALID"); err == nil {
		t.Error("Invalid name accepted")
	}
	if err := validateName(""); err == nil {
		t.Error("Empty name accepted")
	}
}
