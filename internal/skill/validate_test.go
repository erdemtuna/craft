package skill

import "testing"

func TestValidateFrontmatterValid(t *testing.T) {
	fm := &Frontmatter{Name: "lint-check"}
	errs := ValidateFrontmatter(fm)
	if len(errs) != 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}
}

func TestValidateFrontmatterMissingName(t *testing.T) {
	fm := &Frontmatter{}
	errs := ValidateFrontmatter(fm)
	if len(errs) != 1 {
		t.Fatalf("Expected 1 error, got %d: %v", len(errs), errs)
	}
}

func TestValidateFrontmatterInvalidNames(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{"valid-name", false},
		{"a", false},
		{"my-cool-skill", false},
		{"", true},            // empty
		{"MySkill", true},     // uppercase
		{"-leading", true},    // leading hyphen
		{"trailing-", true},   // trailing hyphen
		{"with spaces", true}, // spaces
		{"with_under", true},  // underscore
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fm := &Frontmatter{Name: tc.name}
			errs := ValidateFrontmatter(fm)
			if tc.wantErr && len(errs) == 0 {
				t.Errorf("ValidateFrontmatter(%q) should return error", tc.name)
			}
			if !tc.wantErr && len(errs) != 0 {
				t.Errorf("ValidateFrontmatter(%q) returned unexpected errors: %v", tc.name, errs)
			}
		})
	}
}
