package manifest

import (
	"strings"
	"testing"
)

func TestValidateValid(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "my-package",
		Version:       "1.0.0",
		Skills:        []string{"./skills/one"},
	}
	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}
}

func TestValidateWithDependencies(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "my-package",
		Version:       "1.0.0",
		Skills:        []string{"./skills/one"},
		Dependencies: map[string]string{
			"git-ops": "github.com/example/git@v1.0.0",
		},
	}
	errs := Validate(m)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}
}

func TestValidateSchemaVersion(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 2,
		Name:          "my-package",
		Version:       "1.0.0",
		Skills:        []string{"./skill"},
	}
	errs := Validate(m)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
	assertContains(t, errs[0].Error(), "schema_version")
}

func TestValidateNameFormats(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-name", false},
		{"a", false},
		{"abc123", false},
		{"my-cool-package", false},
		{"a1-b2-c3", false},
		{"", true},                // empty
		{"MyPackage", true},       // uppercase
		{"my package", true},      // spaces
		{"-leading", true},        // leading hyphen
		{"trailing-", true},       // trailing hyphen
		{"double--hyphen", true},  // consecutive hyphens
		{"123start", true},        // starts with number
		{"with_underscore", true}, // underscore
		{"with.dot", true},        // dot
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateName(tc.name)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateName(%q) should return error", tc.name)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateName(%q) returned unexpected error: %v", tc.name, err)
			}
		})
	}
}

func TestValidateNameLength(t *testing.T) {
	// 129 characters — too long
	longName := "a"
	for i := 0; i < 128; i++ {
		longName += "b"
	}
	err := ValidateName(longName)
	if err == nil {
		t.Error("ValidateName should reject names over 128 characters")
	}
}

func TestValidateVersionFormats(t *testing.T) {
	tests := []struct {
		version string
		wantErr bool
	}{
		{"1.0.0", false},
		{"0.1.0", false},
		{"10.20.30", false},
		{"0.0.0", false},
		{"", true},            // empty
		{"1.0", true},         // missing patch
		{"1", true},           // only major
		{"1.0.0-alpha", true}, // pre-release suffix
		{"1.0.0+build", true}, // build metadata
		{"1.0.0-rc.1", true},  // release candidate
		{"v1.0.0", true},      // v prefix
		{"abc", true},         // not a version
		{"1.0.0.0", true},     // too many parts
		{"01.0.0", true},      // leading zero in major
		{"0.01.0", true},      // leading zero in minor
		{"1.0.00", true},      // leading zero in patch
	}

	for _, tc := range tests {
		t.Run(tc.version, func(t *testing.T) {
			err := ValidateVersion(tc.version)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateVersion(%q) should return error", tc.version)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateVersion(%q) returned unexpected error: %v", tc.version, err)
			}
		})
	}
}

func TestValidateEmptySkills(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 1,
		Name:          "my-package",
		Version:       "1.0.0",
		Skills:        []string{},
	}
	errs := Validate(m)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "skills") {
			found = true
		}
	}
	if !found {
		t.Errorf("Expected skills error, got %v", errs)
	}
}

func TestValidateDependencyURLFormats(t *testing.T) {
	tests := []struct {
		url     string
		wantErr bool
	}{
		{"github.com/example/skills@v1.0.0", false},
		{"gitlab.com/org/repo@v2.3.1", false},
		{"github.com/org/my-repo@v0.1.0", false},
		{"github.com/org/repo", true},             // missing version
		{"github.com/org/repo@1.0.0", true},       // missing v prefix
		{"just-a-name@v1.0.0", true},              // missing host/org
		{"github.com/org/repo@latest", true},      // non-semver tag
		{"github.com/org/repo@v1.0.0-beta", true}, // pre-release
	}

	for _, tc := range tests {
		t.Run(tc.url, func(t *testing.T) {
			m := &Manifest{
				SchemaVersion: 1,
				Name:          "test",
				Version:       "1.0.0",
				Skills:        []string{"./skill"},
				Dependencies:  map[string]string{"dep": tc.url},
			}
			errs := Validate(m)
			hasDepErr := false
			for _, e := range errs {
				if strings.Contains(e.Error(), "dependencies") {
					hasDepErr = true
				}
			}
			if tc.wantErr && !hasDepErr {
				t.Errorf("Validate should report dependency error for URL %q", tc.url)
			}
			if !tc.wantErr && hasDepErr {
				t.Errorf("Validate should not report dependency error for URL %q, got %v", tc.url, errs)
			}
		})
	}
}

func TestValidateMultipleErrors(t *testing.T) {
	m := &Manifest{
		SchemaVersion: 2,     // invalid
		Name:          "",    // missing
		Version:       "bad", // invalid
		Skills:        nil,   // empty
	}
	errs := Validate(m)
	if len(errs) < 4 {
		t.Errorf("Expected at least 4 errors, got %d: %v", len(errs), errs)
	}
}

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if !strings.Contains(s, substr) {
		t.Errorf("Expected %q to contain %q", s, substr)
	}
}
